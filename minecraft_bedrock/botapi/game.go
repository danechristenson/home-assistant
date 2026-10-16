package main

// Game settings the panel changes while the server runs, through the
// server console:
//
//   - game rules (saved in the world, so they survive restarts);
//   - difficulty and "Only allowed players", which are also add-on options
//     (applied at every start), so those are saved through the Supervisor
//     too, or a restart would undo them;
//   - one-off actions: time, weather, a message to everyone, kicking.
//
// The game mode can't change live (the server has no defaultgamemode
// command), so it stays a Configuration-tab setting.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// gameRule is a rule the panel offers; Bedrock's console takes the name in
// lower case and reports it in camel case
type gameRule struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Help  string `json:"help"`
	Min   int    `json:"min,omitempty"` // number rules
	Max   int    `json:"max,omitempty"`
}

var gameRules = []gameRule{
	{Name: "keepInventory", Label: "Keep inventory", Help: "Players keep their items when they die"},
	{Name: "showCoordinates", Label: "Show coordinates", Help: "Shows your position on screen"},
	{Name: "pvp", Label: "Player vs player", Help: "Players can hurt each other"},
	{Name: "mobGriefing", Label: "Mob griefing", Help: "Creepers and Endermen can damage builds"},
	{Name: "doFireTick", Label: "Fire spreads", Help: "Fire spreads and burns blocks"},
	{Name: "tntExplodes", Label: "TNT explodes", Help: "TNT can be lit and breaks blocks"},
	{Name: "doDayLightCycle", Label: "Day and night", Help: "Off freezes the time of day"},
	{Name: "doWeatherCycle", Label: "Weather changes", Help: "Off keeps the current weather"},
	{Name: "doMobSpawning", Label: "Mobs spawn", Help: "Animals and monsters appear"},
	{Name: "playersSleepingPercentage", Label: "Players needed to sleep (%)", Help: "Share of players who must sleep to skip the night", Min: 0, Max: 100},
}

var ruleListLine = regexp.MustCompile(`commandBlockOutput = \S+`)

func findRule(name string) (gameRule, bool) {
	for _, rule := range gameRules {
		if strings.EqualFold(rule.Name, name) {
			return rule, true
		}
	}
	return gameRule{}, false
}

func serverRunning() bool {
	live.Lock()
	defer live.Unlock()
	return live.running
}

// readRules asks the server for its game rules and reads its answer from the
// log; nil if it doesn't answer (e.g. it isn't running)
func readRules() map[string]any {
	if !serverRunning() {
		return nil
	}
	logFile := path("run/bedrock.log")
	start := int64(0)
	if info, err := os.Stat(logFile); err == nil {
		start = info.Size()
	}
	if err := console("gamerule"); err != nil {
		return nil
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		file, err := os.Open(logFile)
		if err != nil {
			continue
		}
		_, _ = file.Seek(start, io.SeekStart)
		raw, _ := io.ReadAll(io.LimitReader(file, 1<<20))
		file.Close()
		for _, line := range strings.Split(string(raw), "\n") {
			if ruleListLine.MatchString(line) {
				return parseRules(line)
			}
		}
	}
	return nil
}

func parseRules(line string) map[string]any {
	if i := strings.Index(line, "commandBlockOutput"); i > 0 {
		line = line[i:]
	}
	values := map[string]any{}
	for _, part := range strings.Split(line, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), " = ")
		if !ok {
			continue
		}
		rule, offered := findRule(name)
		if !offered {
			continue
		}
		switch value = strings.TrimSpace(value); {
		case value == "true" || value == "false":
			values[rule.Name] = value == "true"
		default:
			if number, err := strconv.Atoi(value); err == nil {
				values[rule.Name] = number
			}
		}
	}
	return values
}

// setRule changes a game rule; value is a bool, or a number for number rules
func setRule(name string, value any) error {
	rule, ok := findRule(name)
	if !ok {
		return apiError{400, "That isn't a rule the panel offers."}
	}
	var text string
	switch v := value.(type) {
	case bool:
		if rule.Max != 0 {
			return apiError{400, "That rule needs a number."}
		}
		text = strconv.FormatBool(v)
	case float64:
		if rule.Max == 0 || v != float64(int(v)) || int(v) < rule.Min || int(v) > rule.Max {
			return apiError{400, fmt.Sprintf("Use a whole number from %d to %d.", rule.Min, rule.Max)}
		}
		text = strconv.Itoa(int(v))
	default:
		return apiError{400, "Bad value."}
	}
	if !serverRunning() {
		return errNotRunning
	}
	return console("gamerule " + strings.ToLower(rule.Name) + " " + text)
}

var errNotRunning = apiError{409, "The server isn't running, so it can't change that now."}

var difficulties = []string{"peaceful", "easy", "normal", "hard"}

func setDifficulty(value string) error {
	if !contains(difficulties, value) {
		return apiError{400, "Difficulty must be peaceful, easy, normal or hard."}
	}
	if !serverRunning() {
		return errNotRunning
	}
	if err := console("difficulty " + value); err != nil {
		return err
	}
	return saveOption("difficulty", value)
}

func setAllowlistOn(on bool) error {
	if !serverRunning() {
		return errNotRunning
	}
	command := "allowlist off"
	if on {
		command = "allowlist on"
	}
	if err := console(command); err != nil {
		return err
	}
	return saveOption("allow_list", on)
}

// errNotSaved: applied now, but a restart would undo it
var errNotSaved = apiError{502, "Changed now, but it couldn't be saved to the add-on's settings, so a restart would undo it. Change it on the Configuration tab too."}

// saveOption stores one add-on option through the Supervisor (and in the
// local copy the panel reads), so the next start keeps it
func saveOption(key string, value any) error {
	options := map[string]any{}
	if raw, err := os.ReadFile(path("options.json")); err == nil {
		_ = json.Unmarshal(raw, &options)
	}
	options[key] = value
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return errNotSaved
	}
	body, _ := json.Marshal(map[string]any{"options": options})
	request, err := http.NewRequest(http.MethodPost, envOr("SUPERVISOR_URL", "http://supervisor")+"/addons/self/options", bytes.NewReader(body))
	if err != nil {
		return errNotSaved
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return errNotSaved
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errNotSaved
	}
	return writeJSON("options.json", options)
}

// action runs a one-off console command: day, night, clear, say, kick
func action(name, text string) error {
	if !serverRunning() {
		return errNotRunning
	}
	switch name {
	case "day":
		return console("time set day")
	case "night":
		return console("time set night")
	case "clear":
		return console("weather clear")
	case "say":
		message := strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return ' '
			}
			return r
		}, strings.TrimSpace(text))
		message = strings.TrimLeft(message, "/")
		if message == "" {
			return apiError{400, "Type a message first."}
		}
		if len([]rune(message)) > 200 {
			return apiError{400, "Keep the message under 200 characters."}
		}
		return console("say " + message)
	case "kick":
		player := strings.TrimSpace(text)
		if !gamertag.MatchString(player) {
			return apiError{400, "That doesn't look like an Xbox gamertag."}
		}
		live.Lock()
		playing := live.online[strings.ToLower(player)]
		live.Unlock()
		if !playing {
			return apiError{409, player + " isn't playing right now."}
		}
		return console(`kick "` + player + `" Kicked from the Home Assistant panel`)
	}
	return apiError{400, "Unknown action."}
}
