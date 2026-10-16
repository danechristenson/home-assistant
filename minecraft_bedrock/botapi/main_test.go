package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// setup points the API at a temporary /data and records console commands
// TestMain installs one fake console for all tests (background kicks from one
// test may still be running when the next starts, so it never changes): it
// records each command, and answers "gamerule" in the server log like the
// real console does
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fake-console")
	if err != nil {
		panic(err)
	}
	script := filepath.Join(dir, "send-command")
	fake := "#!/bin/sh\necho \"$*\" >> \"$MC_TEST_COMMANDS\"\n" +
		"[ \"$*\" = gamerule ] && echo \"[INFO] commandBlockOutput = true, keepInventory = false, pvp = true, doFireTick = true, playersSleepingPercentage = 100, randomTickSpeed = 1\" >> \"$MC_TEST_LOG\"\nexit 0\n"
	if err := os.WriteFile(script, []byte(fake), 0o755); err != nil {
		panic(err)
	}
	sendCommand = script
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func setup(t *testing.T) (http.Handler, func() []string) {
	t.Helper()
	dataDir = t.TempDir()
	live.Lock()
	live.running, live.online, live.xuid = false, map[string]bool{}, map[string]string{}
	live.Unlock()
	stats.Lock()
	stats.rules, stats.playtime, stats.lastTick = map[string]any{}, map[string]*playtime{}, time.Time{}
	stats.difficulty, stats.allowlist, stats.lastJoined, stats.lastJoinedAt, stats.startedAt = "", nil, "", "", ""
	stats.tracked, stats.mqtt = nil, nil
	stats.Unlock()
	commands := filepath.Join(dataDir, "commands")
	os.MkdirAll(filepath.Join(dataDir, "run"), 0o755)
	// The fake console (see TestMain) writes where these say
	os.Setenv("MC_TEST_COMMANDS", commands)
	os.Setenv("MC_TEST_LOG", filepath.Join(dataDir, "run", "bedrock.log"))
	return nil, func() []string {
		raw, _ := os.ReadFile(commands)
		return strings.Split(strings.TrimSpace(string(raw)), "\n")
	}
}

func call(h http.Handler, method, target, body, from, key string) (int, map[string]any) {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.RemoteAddr = from + ":40000"
	if key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	var result map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &result)
	return recorder.Code, result
}

// bot sends the Matrix Helper Bot's request the way it arrives over MQTT
// (the old HTTP paths name the operation, to keep the tests readable)
func bot(_ http.Handler, method, target, body string) (int, map[string]any) {
	req := botRequest{ID: "test-request"}
	if body != "" {
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			return http.StatusBadRequest, nil
		}
	}
	switch {
	case method == "GET":
		req.Op = "list"
	case strings.HasSuffix(target, "/remove"):
		req.Op = "remove"
	default:
		req.Op = "set"
	}
	status, result := answerBot(req)
	// Round-trip through JSON, like the bot receives it
	raw, _ := json.Marshal(result)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return status, decoded
}

func file(t *testing.T, name string) any {
	t.Helper()
	var value any
	raw, err := os.ReadFile(filepath.Join(dataDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func asJSON(s string) any {
	var value any
	_ = json.Unmarshal([]byte(s), &value)
	return value
}

func TestOperatorAppliesOnFirstJoin(t *testing.T) {
	h, commands := setup(t)
	if code, _ := bot(h, "POST", "/api/bot/players", `{"name":"Cool Kid 99","role":"operator"}`); code != 200 {
		t.Fatalf("add: %d", code)
	}
	if got := file(t, "allowlist.json"); !reflect.DeepEqual(got, asJSON(`[{"ignoresPlayerLimit":false,"name":"Cool Kid 99"}]`)) {
		t.Errorf("allowlist before join: %v", got)
	}
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[]`)) {
		t.Errorf("permissions before join should wait for the xuid: %v", got)
	}
	recordConnection("cool kid 99", "2535400000000001")
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"2535400000000001","permission":"operator"}]`)) {
		t.Errorf("permissions after join: %v", got)
	}
	if got := file(t, "allowlist.json"); !reflect.DeepEqual(got, asJSON(`[{"ignoresPlayerLimit":false,"name":"Cool Kid 99","xuid":"2535400000000001"}]`)) {
		t.Errorf("allowlist after join: %v", got)
	}
	if got := commands(); !reflect.DeepEqual(got[len(got)-2:], []string{"allowlist reload", "permission reload"}) {
		t.Errorf("console: %v", got)
	}

	// Adding again without a role keeps them an operator; "member" demotes
	_, result := bot(h, "POST", "/api/bot/players", `{"name":"cool kid 99"}`)
	if result["player"].(map[string]any)["role"] != "operator" || result["created"] != false {
		t.Errorf("re-add: %v", result)
	}
	bot(h, "POST", "/api/bot/players", `{"name":"Cool Kid 99","role":"member"}`)
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"2535400000000001","permission":"member"}]`)) {
		t.Errorf("after demotion: %v", got)
	}
}

func TestRemoveKeepsOtherPlayers(t *testing.T) {
	h, commands := setup(t)
	os.WriteFile(filepath.Join(dataDir, "allowlist.json"), []byte(`[{"name":"Dad","xuid":"111"}]`), 0o644)
	os.WriteFile(filepath.Join(dataDir, "permissions.json"), []byte(`[{"xuid":"111","permission":"operator"}]`), 0o644)
	bot(h, "POST", "/api/bot/players", `{"name":"Steve"}`)
	recordConnection("Steve", "222")
	if code, _ := bot(h, "POST", "/api/bot/players/remove", `{"name":"steve"}`); code != 200 {
		t.Fatalf("remove: %d", code)
	}
	if got := file(t, "allowlist.json"); !reflect.DeepEqual(got, asJSON(`[{"name":"Dad","xuid":"111"}]`)) {
		t.Errorf("allowlist: %v", got)
	}
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"111","permission":"operator"}]`)) {
		t.Errorf("permissions: %v", got)
	}
	if got := commands(); strings.HasPrefix(got[len(got)-1], "kick") {
		t.Errorf("kicked someone who isn't online: %v", got)
	}
	// Someone who is online gets kicked
	bot(h, "POST", "/api/bot/players", `{"name":"Alex"}`)
	readLine("[INFO] Player connected: Alex, xuid: 333")
	bot(h, "POST", "/api/bot/players/remove", `{"name":"alex"}`)
	if got := commands(); !strings.HasPrefix(got[len(got)-1], `kick "Alex"`) {
		t.Errorf("didn't kick an online player: %v", got)
	}
	if code, _ := bot(h, "POST", "/api/bot/players/remove", `{"name":"steve"}`); code != 404 {
		t.Errorf("second remove: %d", code)
	}
}

func TestGamertags(t *testing.T) {
	h, _ := setup(t)
	for _, name := range []string{"Steve", "Cool Kid 99", "Gamer#1234", "x"} {
		if code, _ := bot(h, "POST", "/api/bot/players", `{"name":"`+name+`"}`); code != 200 {
			t.Errorf("%q refused", name)
		}
	}
	for _, name := range []string{"", strings.Repeat("a", 30), `bad\"quote`, "two  spaces", "x;stop", "Name#12a", "trailing "} {
		body, _ := json.Marshal(map[string]string{"name": name})
		if code, _ := bot(h, "POST", "/api/bot/players", string(body)); code != 400 && name != "trailing " {
			t.Errorf("%q accepted", name)
		}
	}
	if code, _ := bot(h, "POST", "/api/bot/players", `{"name":"Steve","role":"admin"}`); code != 400 {
		t.Errorf("bad role accepted")
	}
	if code, result := bot(h, "POST", "/api/bot/players", `{"name":"Steve","role":"visitor"}`); code != 200 || result["player"].(map[string]any)["role"] != "visitor" {
		t.Errorf("visitor role: %d %v", code, result)
	}
}

// Players added before (or in the panel) can't be changed from chat: a
// Minecraft helper mustn't be able to demote or remove the owner's account
func TestChatCannotTouchOtherPlayers(t *testing.T) {
	h, _ := setup(t)
	os.WriteFile(filepath.Join(dataDir, "allowlist.json"), []byte(`[{"name":"Owner","xuid":"111"}]`), 0o644)
	os.WriteFile(filepath.Join(dataDir, "permissions.json"), []byte(`[{"xuid":"111","permission":"operator"}]`), 0o644)
	for _, body := range []string{`{"name":"owner"}`, `{"name":"Owner","role":"member"}`} {
		if code, _ := bot(h, "POST", "/api/bot/players", body); code != 409 {
			t.Errorf("chat changed an existing player with %s: %d", body, code)
		}
	}
	if code, _ := bot(h, "POST", "/api/bot/players/remove", `{"name":"Owner"}`); code != 409 {
		t.Errorf("chat removed an existing player: %d", code)
	}
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"111","permission":"operator"}]`)) {
		t.Errorf("permissions changed: %v", got)
	}
	// Players the panel adds are off limits to chat too
	panel := panelHandler()
	call(panel, "POST", "/api/players", `{"name":"Guest","role":"visitor"}`, "172.30.32.2", "")
	if code, _ := bot(h, "POST", "/api/bot/players/remove", `{"name":"Guest"}`); code != 409 {
		t.Errorf("chat removed a panel player: %d", code)
	}
	_, list := bot(h, "GET", "/api/bot/players", "")
	if len(list["players"].([]any)) != 0 {
		t.Errorf("bot list shows non-chat players: %v", list)
	}
}

func TestPanel(t *testing.T) {
	setup(t)
	panel := panelHandler()
	for _, from := range []string{"192.168.1.50", "172.30.33.5", "10.1.0.2"} {
		if code, _ := call(panel, "GET", "/api/state", "", from, ""); code != 403 {
			t.Errorf("panel answered %s: %d", from, code)
		}
	}
	if code, _ := call(panel, "GET", "/", "", "172.30.32.2", ""); code != 200 {
		t.Errorf("panel page: %d", code)
	}

	os.WriteFile(filepath.Join(dataDir, "options.json"), []byte(`{"server_name":"Test","default_player_permission_level":"visitor","helper_bot_key":"secret-secret-secret"}`), 0o644)
	os.WriteFile(filepath.Join(dataDir, "allowlist.json"), []byte(`[{"name":"Owner","xuid":"111"},{"name":"Old Friend","xuid":"222"}]`), 0o644)
	os.WriteFile(filepath.Join(dataDir, "permissions.json"), []byte(`[{"xuid":"111","permission":"operator"}]`), 0o644)

	// Change an existing player's role: applies at once (their xuid is known)
	if code, _ := call(panel, "POST", "/api/players", `{"name":"old friend","role":"member"}`, "172.30.32.2", ""); code != 200 {
		t.Fatalf("set role: %d", code)
	}
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"111","permission":"operator"},{"xuid":"222","permission":"member"}]`)) {
		t.Errorf("permissions: %v", got)
	}
	call(panel, "POST", "/api/players", `{"name":"New Kid","role":"operator"}`, "172.30.32.2", "")

	_, state := call(panel, "GET", "/api/state", "", "172.30.32.2", "")
	if _, shown := state["settings"].(map[string]any)["helper_bot_key"]; shown {
		t.Errorf("panel shows an old version's bot key: %v", state["settings"])
	}
	rows := map[string]map[string]any{}
	for _, row := range state["players"].([]any) {
		r := row.(map[string]any)
		rows[r["name"].(string)] = r
	}
	if r := rows["Owner"]; r["role"] != "operator" || r["source"] != "" || r["joined"] != true {
		t.Errorf("Owner: %v", r)
	}
	if r := rows["Old Friend"]; r["role"] != "member" || r["source"] != "panel" {
		t.Errorf("Old Friend: %v", r)
	}
	if r := rows["New Kid"]; r["role"] != "operator" || r["pending"] != true || r["joined"] != false {
		t.Errorf("New Kid: %v", r)
	}

	// Remove someone who was added before
	if code, _ := call(panel, "POST", "/api/players/remove", `{"name":"Owner"}`, "172.30.32.2", ""); code != 200 {
		t.Errorf("remove: %d", code)
	}
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"222","permission":"member"}]`)) {
		t.Errorf("permissions after remove: %v", got)
	}

	// The log tells the panel who's online and last seen
	readLine("[INFO] Server started.")
	readLine("[INFO] Player connected: New Kid, xuid: 333")
	_, state = call(panel, "GET", "/api/state", "", "172.30.32.2", "")
	if online := state["server"].(map[string]any)["online"].([]any); len(online) != 1 || online[0] != "New Kid" {
		t.Errorf("online: %v", online)
	}
	if got := file(t, "permissions.json"); !reflect.DeepEqual(got, asJSON(`[{"xuid":"222","permission":"member"},{"xuid":"333","permission":"operator"}]`)) {
		t.Errorf("first-join role: %v", got)
	}
	readLine("[INFO] Player disconnected: New Kid, xuid: 333, pfid: x")
	_, state = call(panel, "GET", "/api/state", "", "172.30.32.2", "")
	if online := state["server"].(map[string]any)["online"].([]any); len(online) != 0 {
		t.Errorf("still online: %v", online)
	}
}
