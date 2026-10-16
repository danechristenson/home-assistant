package main

// The Matrix Helper Bot's requests (over MQTT): add, re-role and remove
// players added from chat. Also the JSON helpers the panel uses.

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
)

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

// botRequest is one request from the Matrix Helper Bot, over MQTT:
// {"id": "...", "op": "list|set|remove", "name": "...", "role": "..."}.
// The reply goes to minecraft_bedrock/bot/response/<id> as
// {"status": <HTTP-style code>, ...}. Only players added from chat can be
// changed this way.
type botRequest struct {
	ID   string  `json:"id"`
	Op   string  `json:"op"`
	Name string  `json:"name"`
	Role *string `json:"role"`
}

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func answerBot(req botRequest) (int, map[string]any) {
	switch req.Op {
	case "list":
		mu.Lock()
		players := loadPlayers()
		mu.Unlock()
		list := []map[string]any{}
		for _, player := range players {
			if player.source() == fromChat {
				list = append(list, public(player))
			}
		}
		return http.StatusOK, map[string]any{"players": list}
	case "set":
		name := strings.TrimSpace(req.Name)
		if !gamertag.MatchString(name) {
			return http.StatusBadRequest, map[string]any{"error": "That doesn't look like an Xbox gamertag."}
		}
		role := ""
		if req.Role != nil {
			role = strings.ToLower(strings.TrimSpace(*req.Role))
			if !validRole(role) {
				return http.StatusBadRequest, map[string]any{"error": "Role must be visitor, member or operator."}
			}
		}
		player, created, err := setPlayer(name, role, fromChat)
		if err != nil {
			return errorReply(err)
		}
		return http.StatusOK, map[string]any{"player": public(player), "created": created}
	case "remove":
		removed, err := removePlayer(strings.TrimSpace(req.Name), fromChat)
		if errors.Is(err, errNotFound) {
			return http.StatusNotFound, map[string]any{"error": "That player wasn't added from chat."}
		}
		if err != nil {
			return errorReply(err)
		}
		return http.StatusOK, map[string]any{"removed": public(removed)}
	}
	return http.StatusBadRequest, map[string]any{"error": "Unknown request."}
}

func errorReply(err error) (int, map[string]any) {
	var known apiError
	if errors.As(err, &known) {
		return known.status, map[string]any{"error": known.message}
	}
	return http.StatusInternalServerError, map[string]any{"error": "Could not update the allowlist."}
}

// handleBotMessage answers a request published on minecraft_bedrock/bot/request
func handleBotMessage(client *mqttClient, payload []byte) {
	var req botRequest
	if json.Unmarshal(payload, &req) != nil || !requestID.MatchString(req.ID) {
		return
	}
	status, body := answerBot(req)
	body["status"] = status
	raw, _ := json.Marshal(body)
	client.Publish(topicBase+"/bot/response/"+req.ID, raw, false)
}
