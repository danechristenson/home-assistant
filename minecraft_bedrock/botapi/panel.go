package main

// The sidebar panel (Home Assistant ingress): server status, settings and
// every allowed player, with add, change-role and remove. Only Home
// Assistant's ingress proxy may connect; Home Assistant only shows the panel
// to administrators.

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

const ingressProxy = "172.30.32.2"

//go:embed panel.html
var panelPage []byte

type panelRow struct {
	Name     string `json:"name"`
	Role     string `json:"role"`    // their role now
	Pending  bool   `json:"pending"` // role waits for their first join
	Default  bool   `json:"default"` // no role of their own: the server default
	Joined   bool   `json:"joined"`  // Xbox ID known
	Source   string `json:"source"`  // "chat", "panel" or "" (added before)
	Online   bool   `json:"online"`
	LastSeen string `json:"lastSeen"` // RFC 3339, or ""
}

func panelAuthorized(r *http.Request) bool {
	ip := clientIP(r)
	return ip != nil && (ip.IsLoopback() || ip.String() == ingressProxy)
}

func panelState() map[string]any {
	options := map[string]any{}
	if raw, err := os.ReadFile(path("options.json")); err == nil {
		_ = json.Unmarshal(raw, &options)
	}
	delete(options, "helper_bot_key") // older versions' option
	defaultRole, _ := options["default_player_permission_level"].(string)

	mu.Lock()
	players := loadPlayers()
	var allowlist []allowEntry
	readJSON("allowlist.json", &allowlist)
	seen := map[string]string{}
	readJSON("seen.json", &seen)
	mu.Unlock()
	live.Lock()
	running := live.running
	online := []string{}
	onlineSet := map[string]bool{}
	for name := range live.online {
		onlineSet[name] = true
	}
	live.Unlock()

	rows := []panelRow{}
	for _, entry := range allowlist {
		name, xuid := entryName(entry), entryXuid(entry)
		row := panelRow{Name: name, Joined: xuid != "", Online: onlineSet[strings.ToLower(name)], LastSeen: seen[strings.ToLower(name)]}
		if i := find(players, name); i >= 0 {
			row.Source = players[i].source()
			row.Role = players[i].Role
			row.Pending = xuid == "" && players[i].Xuid == ""
		} else if role := permissionOf(xuid); role != "" {
			row.Role = role
		}
		if row.Role == "" {
			row.Role, row.Default = defaultRole, true
		}
		if row.Online {
			online = append(online, name)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(a, b int) bool { return strings.ToLower(rows[a].Name) < strings.ToLower(rows[b].Name) })
	sort.Strings(online)
	return map[string]any{
		"server": map[string]any{
			"running": running,
			"version": os.Getenv("VERSION"),
			"online":  online,
		},
		"settings": options,
		"players":  rows,
	}
}

// fullState adds what the server says about its game rules (asking takes a
// moment, so only the page's full refreshes include it)
func fullState() map[string]any {
	state := panelState()
	state["rules"] = readRules()
	state["ruleList"] = gameRules
	return state
}

func panelHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(panelPage)
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusOK, fullState())
	})
	mux.HandleFunc("POST /api/gamerule", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Rule  string `json:"rule"`
			Value any    `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		if err := setRule(body.Rule, body.Value); err != nil {
			fail(w, err)
			return
		}
		log.Printf("[INFO] Panel: game rule %s set to %v", body.Rule, body.Value)
		reply(w, http.StatusOK, fullState())
	})
	mux.HandleFunc("POST /api/difficulty", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		if err := setDifficulty(strings.ToLower(body.Value)); err != nil {
			fail(w, err)
			return
		}
		log.Printf("[INFO] Panel: difficulty set to %s", body.Value)
		reply(w, http.StatusOK, fullState())
	})
	mux.HandleFunc("POST /api/allowlist", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		if err := setAllowlistOn(body.On); err != nil {
			fail(w, err)
			return
		}
		log.Printf("[INFO] Panel: only allowed players turned %s", map[bool]string{true: "on", false: "off"}[body.On])
		reply(w, http.StatusOK, fullState())
	})
	mux.HandleFunc("POST /api/action", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action string `json:"action"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		if err := action(body.Action, body.Text); err != nil {
			fail(w, err)
			return
		}
		log.Printf("[INFO] Panel: %s", body.Action)
		reply(w, http.StatusOK, panelState())
	})
	mux.HandleFunc("POST /api/players", func(w http.ResponseWriter, r *http.Request) {
		name, role, ok := readPlayerRequest(w, r)
		if !ok {
			return
		}
		if _, _, err := setPlayer(name, role, fromPanel); err != nil {
			fail(w, err)
			return
		}
		reply(w, http.StatusOK, fullState())
	})
	mux.HandleFunc("POST /api/players/remove", func(w http.ResponseWriter, r *http.Request) {
		var body playerRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		if _, err := removePlayer(strings.TrimSpace(body.Name), fromPanel); err != nil {
			if errors.Is(err, errNotFound) {
				refuse(w, http.StatusNotFound, "That player isn't on the allowlist.")
				return
			}
			fail(w, err)
			return
		}
		reply(w, http.StatusOK, fullState())
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !panelAuthorized(r) {
			http.Error(w, "Open this from the Home Assistant sidebar.", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
