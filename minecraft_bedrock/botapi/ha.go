package main

// Home Assistant entities over MQTT discovery: server status, players,
// playtime of tracked players, join/leave events, and controls (game rules,
// difficulty, the allowlist, quick actions). MQTT credentials come from the
// Supervisor (services: mqtt:want).

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	topicBase         = "minecraft_bedrock"
	topicAvailability = topicBase + "/availability"
	topicControls     = topicBase + "/controls" // "online" while the server runs
	topicState        = topicBase + "/state"
	topicEvent        = topicBase + "/event"
	topicCommand      = topicBase + "/command/"
)

var (
	ruleUpdated       = regexp.MustCompile(`Game rule (\w+) has been updated to (\S+)`)
	difficultyChanged = regexp.MustCompile(`Set game difficulty to (\w+)`)
	allowlistToggled  = regexp.MustCompile(`Turned (on|off) the allowlist`)
	slugPattern       = regexp.MustCompile(`[^a-z0-9]+`)
)

// stats is what Home Assistant sees, kept up to date from the log
var stats = struct {
	sync.Mutex
	startedAt     string
	lastJoined    string
	lastJoinedAt  string
	rules         map[string]any
	difficulty    string
	allowlist     *bool
	playtime      map[string]*playtime // by lower-case name
	lastTick      time.Time
	tracked       []string // gamertags, as configured
	mqtt          *mqttClient
	publishNeeded chan struct{}
}{rules: map[string]any{}, playtime: map[string]*playtime{}, publishNeeded: make(chan struct{}, 1)}

type playtime struct {
	Name     string `json:"name"`
	TotalSec int64  `json:"total_seconds"`
	TodaySec int64  `json:"today_seconds"`
	Day      string `json:"day"` // local date of TodaySec
	LastSeen string `json:"last_seen,omitempty"`
}

func slug(name string) string {
	return strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(name), "_"), "_")
}

// ----- events from the log -----

// observeLine updates what Home Assistant sees from one server log line
func observeLine(line string) {
	now := time.Now()
	changed := true
	stats.Lock()
	switch {
	case strings.Contains(line, "Server started."):
		stats.startedAt = now.UTC().Format(time.RFC3339)
		stats.lastTick = now
	case strings.Contains(line, "Stopping server") || strings.Contains(line, "Quit correctly"):
		tickLocked(now)
	default:
		if m := connected.FindStringSubmatch(line); m != nil {
			name := strings.TrimSpace(m[1])
			tickLocked(now)
			stats.lastJoined, stats.lastJoinedAt = name, now.UTC().Format(time.RFC3339)
			playtimeOf(trackedName(name, m[2])).LastSeen = stats.lastJoinedAt
			defer emitEvent("joined", name)
		} else if m := disconnected.FindStringSubmatch(line); m != nil {
			name := strings.TrimSpace(m[1])
			tickLocked(now)
			playtimeOf(trackedName(name, m[2])).LastSeen = now.UTC().Format(time.RFC3339)
			defer emitEvent("left", name)
		} else if m := ruleUpdated.FindStringSubmatch(line); m != nil {
			if rule, ok := findRule(m[1]); ok {
				stats.rules[rule.Name] = parseRuleValue(m[2])
			}
		} else if m := difficultyChanged.FindStringSubmatch(line); m != nil {
			stats.difficulty = strings.ToLower(m[1])
		} else if m := allowlistToggled.FindStringSubmatch(line); m != nil {
			on := m[1] == "on"
			stats.allowlist = &on
		} else {
			changed = false
		}
	}
	stats.Unlock()
	if changed {
		requestPublish()
	}
}

func parseRuleValue(value string) any {
	if value == "true" || value == "false" {
		return value == "true"
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	return value
}

// trackedName is the Tracked players name a joining player counts as: their
// gamertag, or an allowlist label with the same Xbox ID (e.g. "Robin" for an
// allowlist entry named Robin, whatever their real gamertag is)
func trackedName(gamertag, xuid string) string {
	for _, name := range stats.tracked {
		if strings.EqualFold(name, gamertag) {
			return name
		}
	}
	if xuid == "" {
		return gamertag
	}
	var allowlist []allowEntry
	readJSON("allowlist.json", &allowlist)
	for _, name := range stats.tracked {
		for _, entry := range allowlist {
			if strings.EqualFold(entryName(entry), name) && entryXuid(entry) == xuid {
				return name
			}
		}
	}
	return gamertag
}

func playtimeOf(name string) *playtime {
	key := strings.ToLower(name)
	entry := stats.playtime[key]
	if entry == nil {
		entry = &playtime{Name: name}
		stats.playtime[key] = entry
	}
	return entry
}

// tickLocked adds the time since the last tick to everyone online
// (stats must be locked)
func tickLocked(now time.Time) {
	if stats.lastTick.IsZero() {
		stats.lastTick = now
		return
	}
	elapsed := int64(now.Sub(stats.lastTick).Seconds())
	stats.lastTick = now
	today := now.Format("2006-01-02")
	online := onlinePlayers()
	for _, entry := range stats.playtime {
		if entry.Day != today {
			entry.Day, entry.TodaySec = today, 0
		}
	}
	if elapsed <= 0 || elapsed > 600 { // asleep or clock jump: don't count it
		return
	}
	for _, player := range online {
		entry := playtimeOf(trackedName(player.name, player.xuid))
		entry.TotalSec += elapsed
		entry.TodaySec += elapsed
		entry.LastSeen = now.UTC().Format(time.RFC3339)
	}
	_ = writeJSON("playtime.json", stats.playtime)
}

// emitEvent is called with stats unlocked (deferred in observeLine)
type onlinePlayer struct{ name, xuid string }

func onlinePlayers() []onlinePlayer {
	live.Lock()
	defer live.Unlock()
	list := make([]onlinePlayer, 0, len(live.online))
	for name := range live.online {
		list = append(list, onlinePlayer{name, live.xuid[name]})
	}
	return list
}

func emitEvent(kind, name string) {
	stats.Lock()
	client := stats.mqtt
	stats.Unlock()
	if client == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{"event_type": kind, "player": name})
	client.Publish(topicEvent, payload, false)
}

func requestPublish() {
	select {
	case stats.publishNeeded <- struct{}{}:
	default:
	}
}

// ----- the world seed -----

// worldSeed reads RandomSeed (a little-endian long) from the world's level.dat
func worldSeed(level string) string {
	raw, err := os.ReadFile(path("worlds/" + level + "/level.dat"))
	if err != nil {
		return ""
	}
	tag := []byte("\x04\x0a\x00RandomSeed")
	i := strings.Index(string(raw), string(tag))
	if i < 0 || i+len(tag)+8 > len(raw) {
		return ""
	}
	return strconv.FormatInt(int64(binary.LittleEndian.Uint64(raw[i+len(tag):])), 10)
}

// ----- state -----

func options() map[string]any {
	opts := map[string]any{}
	if raw, err := os.ReadFile(path("options.json")); err == nil {
		_ = json.Unmarshal(raw, &opts)
	}
	return opts
}

func statePayload() []byte {
	opts := options()
	level, _ := opts["level_name"].(string)
	live.Lock()
	running := live.running
	online := make([]string, 0, len(live.online))
	for name := range live.online {
		online = append(online, name)
	}
	live.Unlock()
	sort.Strings(online)
	var allowlist []allowEntry
	readJSON("allowlist.json", &allowlist)

	stats.Lock()
	defer stats.Unlock()
	difficulty := stats.difficulty
	if difficulty == "" {
		difficulty, _ = opts["difficulty"].(string)
	}
	allowlistOn, _ := opts["allow_list"].(bool)
	if stats.allowlist != nil {
		allowlistOn = *stats.allowlist
	}
	// Names as players typed them (online keys are lower case)
	names := []string{}
	for _, key := range online {
		if entry := stats.playtime[key]; entry != nil {
			names = append(names, entry.Name)
		} else {
			names = append(names, key)
		}
	}
	players := map[string]any{}
	today := time.Now().Format("2006-01-02")
	playing := map[string]bool{}
	for _, player := range onlinePlayers() {
		playing[strings.ToLower(trackedName(player.name, player.xuid))] = true
	}
	for _, name := range stats.tracked {
		entry := stats.playtime[strings.ToLower(name)]
		p := map[string]any{"online": playing[strings.ToLower(name)], "today_min": 0, "total_min": 0, "last_seen": nil}
		if entry != nil {
			if entry.Day == today {
				p["today_min"] = entry.TodaySec / 60
			}
			p["total_min"] = entry.TotalSec / 60
			if entry.LastSeen != "" {
				p["last_seen"] = entry.LastSeen
			}
		}
		players[slug(name)] = p
	}
	state := map[string]any{
		"running":        running,
		"online_count":   len(names),
		"online":         names,
		"version":        os.Getenv("VERSION"),
		"world":          level,
		"seed":           worldSeed(level),
		"gamemode":       opts["gamemode"],
		"difficulty":     difficulty,
		"allowlist":      allowlistOn,
		"allowed_count":  len(allowlist),
		"last_joined":    nullable(stats.lastJoined),
		"last_joined_at": nullable(stats.lastJoinedAt),
		"started_at":     nullable(stats.startedAt),
		"rules":          stats.rules,
		"players":        players,
	}
	raw, _ := json.Marshal(state)
	return raw
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ----- discovery -----

func device() map[string]any {
	return map[string]any{
		"identifiers":  []string{topicBase},
		"name":         "Minecraft Bedrock",
		"manufacturer": "Mojang",
		"model":        "Bedrock Dedicated Server",
		"sw_version":   os.Getenv("VERSION"),
	}
}

type entity struct {
	component string
	id        string
	config    map[string]any
	controls  bool // only available while the server runs
}

func entities(tracked []string) []entity {
	list := []entity{
		{"binary_sensor", "server_running", map[string]any{"name": "Server", "device_class": "running", "value_template": "{{ 'ON' if value_json.running else 'OFF' }}"}, false},
		{"sensor", "players_online", map[string]any{"name": "Players online", "icon": "mdi:account-group", "unit_of_measurement": "players", "state_class": "measurement",
			"value_template": "{{ value_json.online_count }}", "json_attributes_topic": topicState, "json_attributes_template": "{{ {'players': value_json.online} | tojson }}"}, false},
		{"sensor", "bedrock_version", map[string]any{"name": "Bedrock version", "icon": "mdi:information-outline", "entity_category": "diagnostic", "value_template": "{{ value_json.version }}"}, false},
		{"sensor", "world", map[string]any{"name": "World", "icon": "mdi:earth", "value_template": "{{ value_json.world }}"}, false},
		{"sensor", "seed", map[string]any{"name": "World seed", "icon": "mdi:sprout", "entity_category": "diagnostic", "value_template": "{{ value_json.seed }}"}, false},
		{"sensor", "game_mode", map[string]any{"name": "Game mode", "icon": "mdi:sword", "value_template": "{{ value_json.gamemode | capitalize }}"}, false},
		{"sensor", "allowed_players", map[string]any{"name": "Allowed players", "icon": "mdi:account-check", "unit_of_measurement": "players", "value_template": "{{ value_json.allowed_count }}"}, false},
		{"sensor", "last_player", map[string]any{"name": "Last player to join", "icon": "mdi:account-arrow-right", "value_template": "{{ value_json.last_joined or 'None' }}"}, false},
		{"sensor", "last_join", map[string]any{"name": "Last join", "device_class": "timestamp", "value_template": "{{ value_json.last_joined_at or None }}"}, false},
		{"sensor", "started", map[string]any{"name": "Server started", "device_class": "timestamp", "entity_category": "diagnostic", "value_template": "{{ value_json.started_at or None }}"}, false},
		{"event", "activity", map[string]any{"name": "Player activity", "icon": "mdi:account-switch", "state_topic": topicEvent, "event_types": []string{"joined", "left"}}, false},
		// Controls
		{"switch", "allowlist", map[string]any{"name": "Only allowed players", "icon": "mdi:account-lock", "entity_category": "config",
			"value_template": "{{ 'ON' if value_json.allowlist else 'OFF' }}", "command_topic": topicCommand + "allowlist"}, true},
		{"select", "difficulty", map[string]any{"name": "Difficulty", "icon": "mdi:skull", "options": difficulties,
			"value_template": "{{ value_json.difficulty }}", "command_topic": topicCommand + "difficulty"}, true},
		{"button", "day", map[string]any{"name": "Make it day", "icon": "mdi:weather-sunny", "command_topic": topicCommand + "day"}, true},
		{"button", "night", map[string]any{"name": "Make it night", "icon": "mdi:weather-night", "command_topic": topicCommand + "night"}, true},
		{"button", "clear_weather", map[string]any{"name": "Clear the weather", "icon": "mdi:weather-cloudy-clock", "command_topic": topicCommand + "clear"}, true},
		{"button", "kick_everyone", map[string]any{"name": "Kick everyone", "icon": "mdi:account-cancel", "command_topic": topicCommand + "kick_all"}, true},
		{"text", "message", map[string]any{"name": "Message everyone", "icon": "mdi:message-text", "max": 200, "mode": "text", "command_topic": topicCommand + "say"}, true},
	}
	for _, rule := range gameRules {
		id := "rule_" + slug(rule.Name)
		if rule.Max != 0 {
			list = append(list, entity{"number", id, map[string]any{"name": rule.Label, "icon": "mdi:bed", "entity_category": "config", "min": rule.Min, "max": rule.Max, "step": 1, "mode": "box",
				"value_template": fmt.Sprintf("{{ value_json.rules.%s | default(None) }}", rule.Name), "command_topic": topicCommand + "rule/" + rule.Name}, true})
			continue
		}
		list = append(list, entity{"switch", id, map[string]any{"name": rule.Label, "icon": "mdi:cog-outline", "entity_category": "config",
			"value_template": fmt.Sprintf("{%% set v = value_json.rules.%s %%}{{ 'ON' if v == true else ('OFF' if v == false else None) }}", rule.Name),
			"command_topic":  topicCommand + "rule/" + rule.Name}, true})
	}
	for _, name := range tracked {
		id := slug(name)
		key := fmt.Sprintf("value_json.players['%s']", id)
		list = append(list,
			entity{"binary_sensor", id + "_playing", map[string]any{"name": name + " playing", "device_class": "presence", "icon": "mdi:minecraft",
				"value_template": fmt.Sprintf("{{ 'ON' if %s.online else 'OFF' }}", key)}, false},
			entity{"sensor", id + "_playtime_today", map[string]any{"name": name + " playtime today", "device_class": "duration", "unit_of_measurement": "min", "state_class": "total_increasing",
				"value_template": fmt.Sprintf("{{ %s.today_min }}", key)}, false},
			entity{"sensor", id + "_playtime", map[string]any{"name": name + " total playtime", "device_class": "duration", "unit_of_measurement": "min", "state_class": "total_increasing",
				"value_template": fmt.Sprintf("{{ %s.total_min }}", key)}, false},
			entity{"sensor", id + "_last_seen", map[string]any{"name": name + " last seen", "device_class": "timestamp",
				"value_template": fmt.Sprintf("{{ %s.last_seen or None }}", key)}, false},
		)
	}
	return list
}

// legacyDiscovery are entities earlier versions published under other ids
var legacyDiscovery = []string{
	"homeassistant/binary_sensor/minecraft_bedrock/running/config",
	"homeassistant/sensor/minecraft_bedrock/version/config",
}

func publishDiscovery(client *mqttClient, tracked []string) {
	published := []string{}
	defer func() {
		// Remove entities no longer published (renamed, or players no longer
		// tracked): an empty retained config deletes them in Home Assistant
		var before []string
		readJSON("discovery.json", &before)
		for _, topic := range append(before, legacyDiscovery...) {
			if !contains(published, topic) {
				client.Publish(topic, nil, true)
			}
		}
		_ = writeJSON("discovery.json", published)
	}()
	for _, e := range entities(tracked) {
		config := map[string]any{
			"unique_id":         topicBase + "_" + e.id,
			"default_entity_id": e.component + "." + topicBase + "_" + e.id,
			"device":            device(),
			"origin":            map[string]any{"name": "Minecraft Bedrock add-on", "url": "https://github.com/danechristenson/home-assistant/tree/main/minecraft_bedrock"},
			"has_entity_name":   true,
		}
		for k, v := range e.config {
			config[k] = v
		}
		if e.component != "button" && e.component != "event" && e.component != "text" {
			if _, set := config["state_topic"]; !set {
				config["state_topic"] = topicState
			}
		}
		availability := []map[string]string{{"topic": topicAvailability}}
		if e.controls {
			availability = append(availability, map[string]string{"topic": topicControls})
		}
		config["availability"] = availability
		config["availability_mode"] = "all"
		raw, _ := json.Marshal(config)
		topic := fmt.Sprintf("homeassistant/%s/%s/%s/config", e.component, topicBase, e.id)
		client.Publish(topic, raw, true)
		published = append(published, topic)
	}
}

// ----- commands from Home Assistant -----

func handleCommand(msg mqttMessage) {
	command := strings.TrimPrefix(msg.topic, topicCommand)
	payload := strings.TrimSpace(string(msg.payload))
	var err error
	switch {
	case strings.HasPrefix(command, "rule/"):
		name := strings.TrimPrefix(command, "rule/")
		rule, ok := findRule(name)
		switch {
		case !ok:
			err = errNotFound
		case rule.Max != 0:
			number, parseErr := strconv.ParseFloat(payload, 64)
			if parseErr != nil {
				err = apiError{400, "bad number"}
			} else {
				err = setRule(rule.Name, number)
			}
		default:
			err = setRule(rule.Name, payload == "ON")
		}
	case command == "allowlist":
		err = setAllowlistOn(payload == "ON")
	case command == "difficulty":
		err = setDifficulty(strings.ToLower(payload))
	case command == "day" || command == "night" || command == "clear":
		err = action(command, "")
	case command == "say":
		err = action("say", payload)
	case command == "kick_all":
		live.Lock()
		names := make([]string, 0, len(live.online))
		for name := range live.online {
			names = append(names, name)
		}
		live.Unlock()
		for _, name := range names {
			if kickErr := console(`kick "` + name + `" Kicked from Home Assistant`); kickErr != nil {
				err = kickErr
			}
		}
	default:
		return
	}
	if err != nil && err != errNotSaved {
		log.Printf("[WARN] Home Assistant command %s failed: %v", command, err)
	} else {
		log.Printf("[INFO] Home Assistant: %s", command)
	}
	refreshRules()
	requestPublish()
}

// refreshRules asks the server for its game rules (it only logs changes made
// after the add-on started)
func refreshRules() {
	if rules := readRules(); rules != nil {
		stats.Lock()
		stats.rules = rules
		stats.Unlock()
	}
}

// ----- start -----

func mqttCredentials() (address, username, password string, err error) {
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return "", "", "", fmt.Errorf("no Supervisor token")
	}
	request, _ := http.NewRequest(http.MethodGet, envOr("SUPERVISOR_URL", "http://supervisor")+"/services/mqtt", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return "", "", "", err
	}
	defer response.Body.Close()
	var body struct {
		Data struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&body) != nil || body.Data.Host == "" {
		return "", "", "", fmt.Errorf("no MQTT broker (install the Mosquitto broker add-on)")
	}
	return net.JoinHostPort(body.Data.Host, strconv.Itoa(body.Data.Port)), body.Data.Username, body.Data.Password, nil
}

// startHomeAssistant publishes the entities and keeps them up to date
func startHomeAssistant(tracked []string) {
	readJSON("playtime.json", &stats.playtime)
	if stats.playtime == nil {
		stats.playtime = map[string]*playtime{}
	}
	stats.tracked = tracked

	// The broker may start after this add-on (e.g. when Home Assistant boots):
	// keep asking, and count playtime meanwhile
	go tickForever()
	address, username, password, err := mqttCredentials()
	for wait := 10 * time.Second; err != nil; address, username, password, err = mqttCredentials() {
		if wait == 10*time.Second {
			log.Printf("[INFO] Home Assistant entities are waiting for MQTT: %v", err)
		}
		time.Sleep(wait)
		if wait < 5*time.Minute {
			wait *= 2
		}
	}
	client := &mqttClient{
		address: address, username: username, password: password, clientID: "minecraft_bedrock",
		willTopic: topicAvailability, willPayload: "offline",
		subscriptions: []string{topicCommand + "#"},
		onMessage:     func(msg mqttMessage) { go handleCommand(msg) },
	}
	client.onConnect = func() {
		client.Publish(topicAvailability, []byte("online"), true)
		publishDiscovery(client, tracked)
		requestPublish()
	}
	stats.Lock()
	stats.mqtt = client
	stats.Unlock()
	go client.Run()

	// Publish on changes, and at least every minute
	lastRunning := false
	ticker := time.NewTicker(60 * time.Second)
	for {
		select {
		case <-stats.publishNeeded:
		case <-ticker.C:
			stats.Lock()
			tickLocked(time.Now())
			stats.Unlock()
		}
		running := serverRunning()
		if running && (!lastRunning || len(stats.rules) == 0) {
			time.Sleep(time.Second) // let the server settle after starting
			refreshRules()
		}
		lastRunning = running
		controls := "offline"
		if running {
			controls = "online"
		}
		client.Publish(topicControls, []byte(controls), true)
		client.Publish(topicState, statePayload(), true)
	}
}

// tickForever counts playtime even when MQTT isn't available
func tickForever() {
	for range time.Tick(60 * time.Second) {
		stats.Lock()
		if stats.mqtt != nil {
			stats.Unlock()
			return // the publish loop ticks instead
		}
		tickLocked(time.Now())
		stats.Unlock()
	}
}
