package main

// The Matrix Helper Bot's API: add, re-role and remove players added from
// chat. Only Home Assistant's internal network may connect, with the key from
// the add-on's options.

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

var internal = mustCIDR("172.30.32.0/23")

func mustCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func public(player Player) map[string]any {
	return map[string]any{"name": player.Name, "role": player.Role, "joined": player.Xuid != ""}
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func refuse(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}

// fail answers with an apiError's status and message, or a generic error
func fail(w http.ResponseWriter, err error) {
	var known apiError
	if errors.As(err, &known) {
		refuse(w, known.status, known.message)
		return
	}
	refuse(w, http.StatusInternalServerError, "Could not update the allowlist.")
}

type playerRequest struct {
	Name string  `json:"name"`
	Role *string `json:"role"`
}

// readPlayerRequest validates a gamertag and an optional role
func readPlayerRequest(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	var body playerRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		refuse(w, http.StatusBadRequest, "Bad request.")
		return "", "", false
	}
	name := strings.TrimSpace(body.Name)
	if !gamertag.MatchString(name) {
		refuse(w, http.StatusBadRequest, "That doesn't look like an Xbox gamertag.")
		return "", "", false
	}
	role := ""
	if body.Role != nil {
		role = strings.ToLower(strings.TrimSpace(*body.Role))
		if !validRole(role) {
			refuse(w, http.StatusBadRequest, "Role must be visitor, member or operator.")
			return "", "", false
		}
	}
	return name, role, true
}

func botAuthorized(r *http.Request, key string) bool {
	ip := clientIP(r)
	if ip == nil || !(ip.IsLoopback() || internal.Contains(ip)) {
		return false
	}
	supplied := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(key)) == 1
}

func botHandler(key string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bot/players", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		players := loadPlayers()
		mu.Unlock()
		list := []map[string]any{}
		for _, player := range players {
			if player.source() == fromChat {
				list = append(list, public(player))
			}
		}
		reply(w, http.StatusOK, map[string]any{"players": list})
	})
	mux.HandleFunc("POST /api/bot/players", func(w http.ResponseWriter, r *http.Request) {
		name, role, ok := readPlayerRequest(w, r)
		if !ok {
			return
		}
		player, created, err := setPlayer(name, role, fromChat)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, http.StatusOK, map[string]any{"player": public(player), "created": created})
	})
	mux.HandleFunc("POST /api/bot/players/remove", func(w http.ResponseWriter, r *http.Request) {
		var body playerRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		removed, err := removePlayer(strings.TrimSpace(body.Name), fromChat)
		if errors.Is(err, errNotFound) {
			refuse(w, http.StatusNotFound, "That player wasn't added from chat.")
			return
		}
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, http.StatusOK, map[string]any{"removed": public(removed)})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !botAuthorized(r, key) {
			refuse(w, http.StatusForbidden, "Forbidden.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
