package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlaytime(t *testing.T) {
	setup(t)
	stats.tracked = []string{"Cool Kid", "Never Plays"}
	base := time.Date(2026, 10, 16, 23, 50, 0, 0, time.Local)
	stats.lastTick = base
	readLine("[INFO] Server started.")
	stats.lastTick = base
	// Joining counts nothing from before the join
	stats.Lock()
	tickLocked(base)
	stats.Unlock()
	live.Lock()
	live.online["cool kid"] = true
	live.Unlock()
	stats.Lock()
	playtimeOf("Cool Kid")
	tickLocked(base.Add(5 * time.Minute)) // 23:55
	tickLocked(base.Add(9 * time.Minute)) // 23:59
	entry := stats.playtime["cool kid"]
	if entry.TotalSec != 9*60 || entry.TodaySec != 9*60 {
		t.Errorf("before midnight: total %d today %d", entry.TotalSec, entry.TodaySec)
	}
	tickLocked(base.Add(13 * time.Minute)) // 00:03 the next day: today restarts
	if entry.TotalSec != 13*60 || entry.TodaySec != 4*60 {
		t.Errorf("after midnight: total %d today %d (the 4 minutes across midnight count for the new day)", entry.TotalSec, entry.TodaySec)
	}
	tickLocked(base.Add(13*time.Minute + 2*time.Hour)) // a clock jump / sleep: not counted
	if entry.TotalSec != 13*60 {
		t.Errorf("clock jump counted: %d", entry.TotalSec)
	}
	stats.Unlock()

	var state map[string]any
	json.Unmarshal(statePayload(), &state)
	players := state["players"].(map[string]any)
	if p := players["cool_kid"].(map[string]any); p["online"] != true || p["total_min"] != float64(13) {
		t.Errorf("tracked player: %v", p)
	}
	if p := players["never_plays"].(map[string]any); p["online"] != false || p["total_min"] != float64(0) || p["last_seen"] != nil {
		t.Errorf("never played: %v", p)
	}
	if state["online_count"] != float64(1) || state["online"].([]any)[0] != "Cool Kid" {
		t.Errorf("online: %v %v", state["online_count"], state["online"])
	}
	var saved map[string]*playtime
	readJSON("playtime.json", &saved)
	if saved["cool kid"] == nil || saved["cool kid"].TotalSec != 13*60 {
		t.Errorf("playtime not saved: %v", saved)
	}
}

func TestJoinLeaveAndInGameChanges(t *testing.T) {
	setup(t)
	readLine("[INFO] Server started.")
	readLine("[INFO] Player connected: Steve, xuid: 1")
	readLine("[INFO] Game rule keepinventory has been updated to true")
	readLine("[INFO] Game rule playerssleepingpercentage has been updated to 30")
	readLine("[INFO] Set game difficulty to HARD")
	readLine("[INFO] Turned off the allowlist")
	var state map[string]any
	json.Unmarshal(statePayload(), &state)
	rules := state["rules"].(map[string]any)
	if rules["keepInventory"] != true || rules["playersSleepingPercentage"] != float64(30) {
		t.Errorf("rules: %v", rules)
	}
	if state["difficulty"] != "hard" || state["allowlist"] != false || state["last_joined"] != "Steve" || state["running"] != true {
		t.Errorf("state: %v", state)
	}
	readLine("[INFO] Player disconnected: Steve, xuid: 1, pfid: x")
	json.Unmarshal(statePayload(), &state)
	if state["online_count"] != float64(0) {
		t.Errorf("still online: %v", state["online"])
	}
}

func TestWorldSeed(t *testing.T) {
	setup(t)
	os.MkdirAll(filepath.Join(dataDir, "worlds", "Home"), 0o755)
	raw := []byte("\x0a\x00\x00junk")
	raw = append(raw, "\x04\x0a\x00RandomSeed"...)
	seed := make([]byte, 8)
	binary.LittleEndian.PutUint64(seed, uint64(18446744073709551615-123)) // -124
	raw = append(raw, seed...)
	os.WriteFile(filepath.Join(dataDir, "worlds", "Home", "level.dat"), raw, 0o644)
	if got := worldSeed("Home"); got != "-124" {
		t.Errorf("seed: %q", got)
	}
	if got := worldSeed("Missing"); got != "" {
		t.Errorf("missing world: %q", got)
	}
}

func TestCommandsFromHomeAssistant(t *testing.T) {
	_, commands := setup(t)
	readLine("[INFO] Server started.")
	readLine("[INFO] Player connected: Steve, xuid: 1")
	readLine("[INFO] Player connected: Alex, xuid: 2")
	cases := []struct{ topic, payload, want string }{
		{"rule/keepInventory", "ON", "gamerule keepinventory true"},
		{"rule/pvp", "OFF", "gamerule pvp false"},
		{"rule/playersSleepingPercentage", "40", "gamerule playerssleepingpercentage 40"},
		{"difficulty", "Easy", "difficulty easy"},
		{"allowlist", "OFF", "allowlist off"},
		{"day", "PRESS", "time set day"},
		{"say", "Bedtime in 10 minutes", "say Bedtime in 10 minutes"},
	}
	for _, c := range cases {
		handleCommand(mqttMessage{topic: topicCommand + c.topic, payload: []byte(c.payload)})
		if !contains(commands(), c.want) {
			t.Errorf("%s %q: console got %v", c.topic, c.payload, commands())
		}
	}
	handleCommand(mqttMessage{topic: topicCommand + "kick_all", payload: []byte("PRESS")})
	got := commands()
	kicked := 0
	for _, c := range got {
		if c == `kick "steve" Kicked from Home Assistant` || c == `kick "alex" Kicked from Home Assistant` {
			kicked++
		}
	}
	if kicked != 2 {
		t.Errorf("kick everyone: %v", got)
	}
	before := len(commands())
	handleCommand(mqttMessage{topic: topicCommand + "rule/randomTickSpeed", payload: []byte("100")})
	handleCommand(mqttMessage{topic: topicCommand + "difficulty", payload: []byte("impossible")})
	for _, c := range commands()[before:] {
		if c != "gamerule" { // only the rules refresh
			t.Errorf("a bad command reached the console: %q", c)
		}
	}
}

func TestDiscoveryTemplates(t *testing.T) {
	list := entities([]string{"Cool Kid"}, []rosterEntry{{Slug: "best_friend", Name: "Best Friend", Role: "member"}})
	seen := map[string]bool{}
	for _, e := range list {
		if seen[e.id] {
			t.Errorf("duplicate entity id %s", e.id)
		}
		seen[e.id] = true
		if e.config["name"] == "" {
			t.Errorf("%s has no name", e.id)
		}
	}
	for _, id := range []string{"server_running", "players_online", "bedrock_version", "seed", "activity", "allowlist", "difficulty", "kick_everyone", "message",
		"rule_keepinventory", "rule_playerssleepingpercentage", "cool_kid_playing", "cool_kid_playtime_today", "cool_kid_playtime", "cool_kid_last_seen",
		"supper_start", "supper_cancel", "supper", "add_player", "player_best_friend_role", "player_best_friend_remove"} {
		if !seen[id] {
			t.Errorf("missing entity %s", id)
		}
	}
}

// Tracked players can be allowlist labels: "Robin" counts whoever joins with
// the Xbox ID of the allowlist entry named Robin, whatever their gamertag
func TestTrackedByAllowlistLabel(t *testing.T) {
	setup(t)
	os.WriteFile(filepath.Join(dataDir, "allowlist.json"), []byte(`[{"name":"Robin","xuid":"999"},{"name":"Sam","xuid":"888"}]`), 0o644)
	stats.tracked = []string{"Robin", "Sam"}
	base := time.Now().Add(-10 * time.Minute)
	readLine("[INFO] Server started.")
	stats.lastTick = base
	readLine("[INFO] Player connected: RealGamer123, xuid: 999")
	stats.Lock()
	stats.lastTick = base
	tickLocked(base.Add(6 * time.Minute))
	stats.Unlock()
	var state map[string]any
	json.Unmarshal(statePayload(), &state)
	players := state["players"].(map[string]any)
	if robin := players["robin"].(map[string]any); robin["online"] != true || robin["total_min"] != float64(6) || robin["last_seen"] == nil {
		t.Errorf("Robin: %v", robin)
	}
	if sam := players["sam"].(map[string]any); sam["online"] != false || sam["total_min"] != float64(0) {
		t.Errorf("Sam: %v", sam)
	}
	if state["last_joined"] != "RealGamer123" {
		t.Errorf("last joined should be the real gamertag: %v", state["last_joined"])
	}
}
