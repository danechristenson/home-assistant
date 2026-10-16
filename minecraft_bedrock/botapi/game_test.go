package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func panelCall(t *testing.T, method, target, body string) (int, map[string]any) {
	t.Helper()
	return call(panelHandler(), method, target, body, "172.30.32.2", "")
}

func lastCommand(commands func() []string) string {
	got := commands()
	return got[len(got)-1]
}

func TestGameRules(t *testing.T) {
	_, commands := setup(t)
	readLine("[INFO] Stopping server...")
	if code, _ := panelCall(t, "POST", "/api/gamerule", `{"rule":"keepInventory","value":true}`); code != 409 {
		t.Errorf("changed a rule with the server stopped: %d", code)
	}
	_, state := panelCall(t, "GET", "/api/state", "")
	if state["rules"] != nil {
		t.Errorf("rules while stopped: %v", state["rules"])
	}

	readLine("[INFO] Server started.")
	_, state = panelCall(t, "GET", "/api/state", "")
	rules := state["rules"].(map[string]any)
	if rules["keepInventory"] != false || rules["pvp"] != true || rules["playersSleepingPercentage"] != float64(100) {
		t.Errorf("rules: %v", rules)
	}
	if _, offered := rules["randomTickSpeed"]; offered {
		t.Errorf("a rule the panel doesn't offer came through: %v", rules)
	}

	cases := []struct {
		body    string
		status  int
		command string
	}{
		{`{"rule":"keepInventory","value":true}`, 200, "gamerule keepinventory true"},
		{`{"rule":"playersSleepingPercentage","value":50}`, 200, "gamerule playerssleepingpercentage 50"},
		{`{"rule":"playersSleepingPercentage","value":150}`, 400, ""},
		{`{"rule":"playersSleepingPercentage","value":true}`, 400, ""},
		{`{"rule":"keepInventory","value":5}`, 400, ""},
		{`{"rule":"randomTickSpeed","value":100}`, 400, ""},
		{`{"rule":"keepInventory; stop","value":true}`, 400, ""},
	}
	for _, c := range cases {
		code, _ := panelCall(t, "POST", "/api/gamerule", c.body)
		if code != c.status {
			t.Errorf("%s: %d, want %d", c.body, code, c.status)
		}
		if c.command != "" && !contains(commands(), c.command) {
			t.Errorf("%s: console got %v", c.body, commands())
		}
	}
}

func TestDifficultyAndAllowlistAreSaved(t *testing.T) {
	_, commands := setup(t)
	readLine("[INFO] Server started.")
	os.WriteFile(filepath.Join(dataDir, "options.json"), []byte(`{"difficulty":"normal","allow_list":true,"server_name":"Test"}`), 0o644)

	var saved map[string]any
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/addons/self/options" || r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		saved = body["options"]
		w.Write([]byte(`{"result":"ok","data":{}}`))
	}))
	defer supervisor.Close()
	t.Setenv("SUPERVISOR_URL", supervisor.URL)
	t.Setenv("SUPERVISOR_TOKEN", "test-token")

	if code, _ := panelCall(t, "POST", "/api/difficulty", `{"value":"hard"}`); code != 200 {
		t.Fatalf("difficulty: %d", code)
	}
	if !contains(commands(), "difficulty hard") || saved["difficulty"] != "hard" || saved["server_name"] != "Test" {
		t.Errorf("difficulty: console %v, saved %v", commands(), saved)
	}
	if code, _ := panelCall(t, "POST", "/api/difficulty", `{"value":"impossible"}`); code != 400 {
		t.Errorf("bad difficulty: %d", code)
	}
	if code, _ := panelCall(t, "POST", "/api/allowlist", `{"on":false}`); code != 200 || !contains(commands(), "allowlist off") || saved["allow_list"] != false {
		t.Errorf("allowlist off: %d %v %v", code, commands(), saved)
	}
	var local map[string]any
	readJSON("options.json", &local)
	if local["allow_list"] != false || local["difficulty"] != "hard" {
		t.Errorf("local options: %v", local)
	}

	// Without the Supervisor it still changes now, but says it won't last
	t.Setenv("SUPERVISOR_TOKEN", "")
	code, result := panelCall(t, "POST", "/api/allowlist", `{"on":true}`)
	if code != 502 || lastCommand(commands) != "allowlist on" || !strings.Contains(result["error"].(string), "restart") {
		t.Errorf("unsaved change: %d %q %v", code, lastCommand(commands), result)
	}
}

func TestActions(t *testing.T) {
	_, commands := setup(t)
	readLine("[INFO] Server started.")
	cases := []struct {
		body    string
		status  int
		command string
	}{
		{`{"action":"day"}`, 200, "time set day"},
		{`{"action":"night"}`, 200, "time set night"},
		{`{"action":"clear"}`, 200, "weather clear"},
		{`{"action":"say","text":"Dinner in 5\nminutes"}`, 200, "say Dinner in 5 minutes"},
		{`{"action":"say","text":"/op me"}`, 200, "say op me"},
		{`{"action":"say","text":"   "}`, 400, ""},
		{`{"action":"kick","text":"Steve"}`, 409, ""}, // not online
		{`{"action":"explode"}`, 400, ""},
	}
	for _, c := range cases {
		code, _ := panelCall(t, "POST", "/api/action", c.body)
		if code != c.status {
			t.Errorf("%s: %d, want %d", c.body, code, c.status)
		}
		if c.command != "" && lastCommand(commands) != c.command {
			t.Errorf("%s: console got %q", c.body, lastCommand(commands))
		}
	}
	readLine("[INFO] Player connected: Steve, xuid: 444")
	if code, _ := panelCall(t, "POST", "/api/action", `{"action":"kick","text":"steve"}`); code != 200 || !strings.HasPrefix(lastCommand(commands), `kick "steve"`) {
		t.Errorf("kick: %d %q", code, lastCommand(commands))
	}
}
