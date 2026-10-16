package main

// The add-on's "Minecraft" dashboard in Home Assistant, built from the
// tracked players and the allowlist and updated when they change:
//
//   - Reports (everyone): server status, who's online, a section per
//     tracked player, playtime per day, players online, joins and leaves.
//   - Actions (administrators): supper time, quick actions, a message to
//     everyone, players (add, role, remove) and game settings.
//
// If someone edits the dashboard in Home Assistant, the add-on notices (the
// config no longer matches what it last saved) and stops updating it.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const dashboardPath = "minecraft-dashboard"

var dashboardMu sync.Mutex

type dashboardRecord struct {
	Hash   string `json:"hash"`   // of the config the add-on last saved
	Edited bool   `json:"edited"` // someone changed it: leave it alone
}

func configHash(config any) string {
	raw, _ := json.Marshal(config) // map keys sorted, so stable
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func addonSlug() string {
	token := envOr("SUPERVISOR_TOKEN", "")
	request, _ := http.NewRequest(http.MethodGet, envOr("SUPERVISOR_URL", "http://supervisor")+"/addons/self/info", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	var body struct {
		Data struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	_ = json.NewDecoder(response.Body).Decode(&body)
	return body.Data.Slug
}

func tile(entity string, extra ...any) map[string]any {
	card := map[string]any{"type": "tile", "entity": entity}
	for i := 0; i+1 < len(extra); i += 2 {
		card[extra[i].(string)] = extra[i+1]
	}
	return card
}

func heading(text, icon string) map[string]any {
	return map[string]any{"type": "heading", "heading": text, "icon": icon}
}

func pressButton(entity, name, icon, confirm string) map[string]any {
	action := map[string]any{"action": "perform-action", "perform_action": "button.press", "target": map[string]any{"entity_id": entity}}
	if confirm != "" {
		action["confirmation"] = map[string]any{"text": confirm}
	}
	return map[string]any{"type": "button", "entity": entity, "name": name, "icon": icon, "show_state": false, "tap_action": action}
}

// buildDashboard makes the dashboard config for these players
func buildDashboard(tracked []string, players []rosterEntry, admins []string, addon string) map[string]any {
	e := func(kind, id string) string { return kind + "." + topicBase + "_" + id }
	server := []any{
		heading("Server", "mdi:minecraft"),
		tile(e("binary_sensor", "server_running"), "name", "Server"),
		tile(e("sensor", "players_online")),
		map[string]any{"type": "markdown", "content": "**Online now:** {{ (state_attr('" + e("sensor", "players_online") + "', 'players') or []) | join(', ') or 'Nobody' }}"},
		tile(e("sensor", "supper"), "name", "Supper time"),
		tile(e("sensor", "last_player")),
		tile(e("sensor", "last_join")),
		tile(e("sensor", "world")),
		tile(e("sensor", "game_mode")),
		tile(e("select", "difficulty"), "name", "Difficulty"),
		tile(e("sensor", "allowed_players")),
		tile(e("sensor", "bedrock_version")),
		tile(e("sensor", "started")),
		tile(e("sensor", "seed")),
	}
	reports := []any{map[string]any{"type": "grid", "cards": server}}
	playtime, playing := []string{}, []string{}
	for _, name := range tracked {
		id := slug(name)
		reports = append(reports, map[string]any{"type": "grid", "cards": []any{
			heading(name, "mdi:account"),
			tile(e("binary_sensor", id+"_playing"), "name", "Playing now"),
			tile(e("sensor", id+"_playtime_today"), "name", "Today"),
			tile(e("sensor", id+"_playtime"), "name", "Total"),
			tile(e("sensor", id+"_last_seen"), "name", "Last seen"),
		}})
		playtime = append(playtime, e("sensor", id+"_playtime"))
		playing = append(playing, e("binary_sensor", id+"_playing"))
	}
	activity := []any{heading("Activity", "mdi:chart-bar")}
	if len(playtime) > 0 {
		activity = append(activity, map[string]any{"type": "statistics-graph", "title": "Playtime per day (minutes)", "entities": playtime,
			"period": "day", "stat_types": []string{"change"}, "chart_type": "bar", "days_to_show": 14, "grid_options": map[string]any{"columns": "full"}})
	}
	activity = append(activity,
		map[string]any{"type": "history-graph", "title": "Players online (last 24 hours)", "hours_to_show": 24,
			"entities": []string{e("sensor", "players_online")}, "grid_options": map[string]any{"columns": "full"}},
		map[string]any{"type": "logbook", "title": "Joins and leaves", "hours_to_show": 168,
			"target":       map[string]any{"entity_id": append(playing, e("sensor", "last_player"), e("binary_sensor", "server_running"))},
			"grid_options": map[string]any{"columns": "full"}},
	)
	reports = append(reports, map[string]any{"type": "grid", "column_span": 3, "cards": activity})

	panelLink := map[string]any{"type": "button", "name": "Open the add-on (web UI)", "icon": "mdi:open-in-app", "show_state": false,
		"tap_action": map[string]any{"action": "navigate", "navigation_path": "/config/app/" + addon + "/info"}}
	actions := []any{
		map[string]any{"type": "grid", "cards": []any{
			heading("Supper time", "mdi:silverware-fork-knife"),
			tile(e("sensor", "supper"), "name", "Supper time"),
			pressButton(e("button", "supper_start"), "Start supper time", "mdi:silverware-fork-knife",
				fmt.Sprintf("Count down %d minutes in the game, then kick the tracked players for a minute?", supperCountdown)),
			pressButton(e("button", "supper_cancel"), "Cancel", "mdi:cancel", ""),
			heading("Quick actions", "mdi:lightning-bolt"),
			pressButton(e("button", "day"), "Make it day", "mdi:weather-sunny", ""),
			pressButton(e("button", "night"), "Make it night", "mdi:weather-night", ""),
			pressButton(e("button", "clear_weather"), "Clear the weather", "mdi:weather-cloudy-clock", ""),
			pressButton(e("button", "kick_everyone"), "Kick everyone", "mdi:account-cancel", "Kick everyone out of Minecraft?"),
			map[string]any{"type": "entities", "entities": []any{map[string]any{"entity": e("text", "message"), "name": "Message everyone"}}},
		}},
	}
	playerRows := []any{map[string]any{"entity": e("text", "add_player"), "name": "Add player (Xbox gamertag)"}}
	for _, player := range players {
		playerRows = append(playerRows,
			map[string]any{"entity": e("select", "player_"+player.Slug+"_role"), "name": player.Name},
			// A button row, so removing asks first
			map[string]any{"type": "button", "name": "Remove " + player.Name, "icon": "mdi:account-remove", "action_name": "Remove",
				"tap_action": map[string]any{"action": "perform-action", "perform_action": "button.press",
					"target":       map[string]any{"entity_id": e("button", "player_"+player.Slug+"_remove")},
					"confirmation": map[string]any{"text": "Remove " + player.Name + " from the Minecraft server?"}}},
		)
	}
	actions = append(actions, map[string]any{"type": "grid", "cards": []any{
		heading("Players", "mdi:account-group"),
		map[string]any{"type": "entities", "entities": playerRows},
		panelLink,
	}})
	game := []any{
		map[string]any{"entity": e("select", "difficulty"), "name": "Difficulty"},
		map[string]any{"entity": e("switch", "allowlist"), "name": "Only allowed players"},
	}
	for _, rule := range gameRules {
		kind := "switch"
		if rule.Max != 0 {
			kind = "number"
		}
		game = append(game, map[string]any{"entity": e(kind, "rule_"+slug(rule.Name)), "name": rule.Label})
	}
	actions = append(actions, map[string]any{"type": "grid", "cards": []any{heading("Game", "mdi:cog"), map[string]any{"type": "entities", "entities": game}}})

	visible := []any{}
	for _, id := range admins {
		visible = append(visible, map[string]any{"user": id})
	}
	return map[string]any{"views": []any{
		map[string]any{"title": "Reports", "path": "reports", "icon": "mdi:chart-box-outline", "type": "sections", "max_columns": 3, "sections": reports},
		map[string]any{"title": "Actions", "path": "actions", "icon": "mdi:gamepad-variant", "type": "sections", "max_columns": 3, "visible": visible, "sections": actions},
	}}
}

// The admins and the add-on's slug change rarely: ask Home Assistant hourly
var dashboardCache struct {
	admins  []string
	slug    string
	fetched time.Time
}

func adminUsers(ws *haSocket) ([]string, error) {
	raw, err := ws.call(map[string]any{"type": "config/auth/list"})
	if err != nil {
		return nil, err
	}
	var users []struct {
		ID              string   `json:"id"`
		GroupIDs        []string `json:"group_ids"`
		SystemGenerated bool     `json:"system_generated"`
		IsActive        bool     `json:"is_active"`
	}
	_ = json.Unmarshal(raw, &users)
	admins := []string{}
	for _, u := range users {
		if u.IsActive && !u.SystemGenerated && contains(u.GroupIDs, "system-admin") {
			admins = append(admins, u.ID)
		}
	}
	return admins, nil
}

func dashboardExists(ws *haSocket) (bool, error) {
	raw, err := ws.call(map[string]any{"type": "lovelace/dashboards/list"})
	if err != nil {
		return false, err
	}
	var dashboards []struct {
		URLPath string `json:"url_path"`
	}
	_ = json.Unmarshal(raw, &dashboards)
	for _, d := range dashboards {
		if d.URLPath == dashboardPath {
			return true, nil
		}
	}
	return false, nil
}

// updateDashboard creates or refreshes the dashboard when what it shows
// changed. It only talks to Home Assistant when there's something to do.
func updateDashboard(tracked []string) {
	if enabled, ok := options()["dashboard"].(bool); ok && !enabled {
		return
	}
	if !dashboardMu.TryLock() {
		return // an update is already running
	}
	defer dashboardMu.Unlock()
	var record dashboardRecord
	readJSON("dashboard.json", &record)
	stale := time.Since(dashboardCache.fetched) > time.Hour

	var ws *haSocket
	connect := func() bool {
		if ws == nil {
			ws, _ = dialHomeAssistant()
		}
		return ws != nil
	}
	defer func() {
		if ws != nil {
			ws.Close()
		}
	}()

	if record.Edited {
		// Hand-edited: leave it alone, unless it has been deleted (checked hourly)
		if !stale || !connect() {
			return
		}
		dashboardCache.fetched = time.Now()
		if exists, err := dashboardExists(ws); err != nil || exists {
			return
		}
		record = dashboardRecord{}
		log.Printf("[INFO] The edited Minecraft dashboard was deleted; the add-on looks after it again")
	}
	if stale || dashboardCache.slug == "" {
		if !connect() {
			return // not on Home Assistant (or not reachable yet)
		}
		admins, err := adminUsers(ws)
		if err != nil {
			log.Printf("[WARN] Dashboard: can't list users: %v", err)
			return
		}
		dashboardCache.admins, dashboardCache.slug, dashboardCache.fetched = admins, addonSlug(), time.Now()
	}

	players := roster()
	var config map[string]any
	raw, _ := json.Marshal(buildDashboard(tracked, players, dashboardCache.admins, dashboardCache.slug))
	_ = json.Unmarshal(raw, &config) // the same shape Home Assistant gives back
	want := configHash(config)
	if want == record.Hash {
		return // unchanged since the add-on last saved it
	}
	if !connect() {
		return
	}
	exists, err := dashboardExists(ws)
	if err != nil {
		return
	}
	if !exists {
		if _, err := ws.call(map[string]any{"type": "lovelace/dashboards/create", "url_path": dashboardPath, "title": "Minecraft",
			"icon": "mdi:minecraft", "show_in_sidebar": true, "require_admin": false, "mode": "storage"}); err != nil {
			log.Printf("[WARN] Dashboard: can't create it: %v", err)
			return
		}
		log.Printf("[INFO] Created the Minecraft dashboard")
	} else if record.Hash != "" {
		// Ours before: only replace it if nobody has edited it since
		if currentRaw, err := ws.call(map[string]any{"type": "lovelace/config", "url_path": dashboardPath}); err == nil {
			var current map[string]any
			_ = json.Unmarshal(currentRaw, &current)
			if configHash(current) != record.Hash {
				record.Edited = true
				_ = writeJSON("dashboard.json", record)
				log.Printf("[INFO] The Minecraft dashboard was edited in Home Assistant, so the add-on no longer updates it. Delete the dashboard to get the add-on's version back.")
				return
			}
		}
	}
	if _, err := ws.call(map[string]any{"type": "lovelace/config/save", "url_path": dashboardPath, "config": config}); err != nil {
		log.Printf("[WARN] Dashboard: can't save it: %v", err)
		return
	}
	record.Hash = want
	_ = writeJSON("dashboard.json", record)
	log.Printf("[INFO] Updated the Minecraft dashboard (%d tracked players, %d allowed players)", len(tracked), len(players))
}
