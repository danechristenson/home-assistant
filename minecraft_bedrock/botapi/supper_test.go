package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSupperTime(t *testing.T) {
	_, commands := setup(t)
	supper.Lock()
	supper.minute, supper.rejoinDelay = 40*time.Millisecond, 5*time.Millisecond
	supper.Unlock()
	t.Cleanup(func() {
		cancelSupper()
		time.Sleep(50 * time.Millisecond) // let background kicks finish
		supper.Lock()
		supper.minute, supper.rejoinDelay = time.Minute, 3*time.Second
		supper.Unlock()
	})
	os.WriteFile(filepath.Join(dataDir, "allowlist.json"), []byte(`[{"name":"Kid","xuid":"1"}]`), 0o644)
	stats.tracked = []string{"Kid"}
	readLine("[INFO] Server started.")
	readLine("[INFO] Player connected: KidGamer, xuid: 1") // tracked through the allowlist name
	readLine("[INFO] Player connected: Visitor, xuid: 2")

	if err := startSupper(); err != nil {
		t.Fatal(err)
	}
	if state := supperState(); state["state"] != "countdown" || state["minutes_left"] != 5 {
		t.Errorf("countdown state: %v", state)
	}
	_ = startSupper() // a second press does nothing
	time.Sleep(6 * supper.minute)

	var says, kicks []string
	for _, c := range commands() {
		if strings.HasPrefix(c, "say ") {
			says = append(says, c)
		}
		if strings.HasPrefix(c, "kick ") {
			kicks = append(kicks, c)
		}
	}
	if len(says) != 6 || !strings.Contains(says[0], "5 minutes") || !strings.Contains(says[4], "1 minute!") {
		t.Errorf("messages: %q", says)
	}
	if len(kicks) != 1 || !strings.HasPrefix(kicks[0], `kick "kidgamer"`) {
		t.Errorf("only the tracked player should be kicked: %q", kicks)
	}
	if state := supperState(); state["state"] != "lockout" {
		t.Errorf("after the countdown: %v", state)
	}

	// Rejoining during the lockout: kicked again
	readLine("[INFO] Player disconnected: KidGamer, xuid: 1, pfid: x")
	readLine("[INFO] Player connected: KidGamer, xuid: 1")
	time.Sleep(20 * time.Millisecond)
	if last := commands()[len(commands())-1]; !strings.HasPrefix(last, `kick "KidGamer"`) {
		t.Errorf("rejoin not kicked: %q", last)
	}

	// After the lockout they can play
	time.Sleep(2 * supper.minute)
	before := len(commands())
	readLine("[INFO] Player connected: KidGamer, xuid: 1")
	time.Sleep(20 * time.Millisecond)
	for _, c := range commands()[before:] {
		if strings.HasPrefix(c, "kick") {
			t.Errorf("kicked after the lockout: %q", c)
		}
	}
}

func TestSupperCancel(t *testing.T) {
	_, commands := setup(t)
	supper.Lock()
	supper.minute = 40 * time.Millisecond
	supper.Unlock()
	t.Cleanup(func() {
		cancelSupper()
		supper.Lock()
		supper.minute = time.Minute
		supper.Unlock()
	})
	readLine("[INFO] Server started.")
	readLine("[INFO] Player connected: Kid, xuid: 1")
	_ = startSupper()
	time.Sleep(supper.minute + 10*time.Millisecond)
	cancelSupper()
	time.Sleep(6 * supper.minute)
	for _, c := range commands() {
		if strings.HasPrefix(c, "kick") {
			t.Errorf("kicked after cancelling: %q", c)
		}
	}
	if last := commands()[len(commands())-1]; !strings.Contains(last, "Supper time is off") {
		t.Errorf("no all-clear message: %q", last)
	}
	if state := supperState(); state["state"] != "off" {
		t.Errorf("state: %v", state)
	}
}

func TestPlayerCommandsFromHomeAssistant(t *testing.T) {
	setup(t)
	os.WriteFile(filepath.Join(dataDir, "allowlist.json"), []byte(`[{"name":"Old Friend","xuid":"5"}]`), 0o644)
	handleCommand(mqttMessage{topic: topicCommand + "add_player", payload: []byte("New Kid")})
	handleCommand(mqttMessage{topic: topicCommand + "add_player", payload: []byte(`bad"name`)})
	handleCommand(mqttMessage{topic: topicCommand + "player/old_friend/role", payload: []byte("operator")})
	handleCommand(mqttMessage{topic: topicCommand + "player/new_kid/role", payload: []byte("superuser")})
	var state map[string]any
	json.Unmarshal(statePayload(), &state)
	roster := state["roster"].(map[string]any)
	if len(roster) != 2 || roster["old_friend"].(map[string]any)["role"] != "operator" || roster["new_kid"].(map[string]any)["role"] != "member" {
		t.Errorf("roster: %v", roster)
	}
	handleCommand(mqttMessage{topic: topicCommand + "player/new_kid/remove", payload: []byte("PRESS")})
	json.Unmarshal(statePayload(), &state)
	if roster := state["roster"].(map[string]any); len(roster) != 1 {
		t.Errorf("after remove: %v", roster)
	}
}

func TestDashboard(t *testing.T) {
	config := buildDashboard([]string{"Robin", "Sam"}, []rosterEntry{{Slug: "old_friend", Name: "Old Friend", Role: "member"}}, []string{"admin-id"}, "abc_minecraft_bedrock")
	raw, _ := json.Marshal(config)
	text := string(raw)
	for _, want := range []string{
		"sensor.minecraft_bedrock_robin_playtime_today", "sensor.minecraft_bedrock_sam_playtime",
		"select.minecraft_bedrock_player_old_friend_role", "button.minecraft_bedrock_player_old_friend_remove",
		"button.minecraft_bedrock_supper_start", "text.minecraft_bedrock_add_player", "switch.minecraft_bedrock_rule_keepinventory",
		"number.minecraft_bedrock_rule_playerssleepingpercentage", `"/config/app/abc_minecraft_bedrock/info"`, `{"user":"admin-id"}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("dashboard is missing %s", want)
		}
	}
	views := config["views"].([]any)
	if _, restricted := views[0].(map[string]any)["visible"]; restricted {
		t.Errorf("Reports should be visible to everyone")
	}
	// Every entity it uses exists
	ids := map[string]bool{}
	for _, e := range entities([]string{"Robin", "Sam"}, []rosterEntry{{Slug: "old_friend", Name: "Old Friend"}}) {
		ids[e.component+".minecraft_bedrock_"+e.id] = true
	}
	for _, match := range regexpEntity.FindAllString(text, -1) {
		if !ids[match] {
			t.Errorf("dashboard uses %s, which isn't published", match)
		}
	}
}

var regexpEntity = regexp.MustCompile(`(?:sensor|binary_sensor|switch|select|number|button|text|event)\.minecraft_bedrock_[a-z0-9_]+`)
