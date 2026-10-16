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
)

const testKey = "0123456789abcdef0123"

// setup points the API at a temporary /data and records console commands
func setup(t *testing.T) (http.Handler, func() []string) {
	t.Helper()
	dataDir = t.TempDir()
	commands := filepath.Join(dataDir, "commands")
	script := filepath.Join(dataDir, "send-command")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$*\" >> "+commands+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sendCommand = script
	return handler(testKey), func() []string {
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

func bot(h http.Handler, method, target, body string) (int, map[string]any) {
	return call(h, method, target, body, "172.30.33.5", testKey)
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

func TestAccess(t *testing.T) {
	h, _ := setup(t)
	cases := []struct {
		from, key string
		want      int
	}{
		{"172.30.33.5", testKey, 200},
		{"127.0.0.1", testKey, 200},
		{"172.30.33.5", "wrong-key-wrong-key", 403},
		{"172.30.33.5", "", 403},
		{"192.168.1.50", testKey, 403},
		{"10.1.0.2", testKey, 403},
	}
	for _, c := range cases {
		if got, _ := call(h, "GET", "/api/bot/players", "", c.from, c.key); got != c.want {
			t.Errorf("from %s with key %q: got %d, want %d", c.from, c.key, got, c.want)
		}
	}
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
	if got := commands(); !strings.HasPrefix(got[len(got)-1], `kick "Steve"`) {
		t.Errorf("console: %v", got)
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
}
