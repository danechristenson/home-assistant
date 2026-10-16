package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"maunium.net/go/mautrix/id"
)

// The Minecraft add-on's bot API (minecraftserver/web/bot_api.py in the
// fork): players are added by gamertag; their Xbox ID is learned when they
// first join, and operator status applies from then on.

type minecraftPlayer struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Joined bool   `json:"joined"`
}

var minecraftClient = &http.Client{Timeout: 15 * time.Second}

func (b *Bot) minecraftCall(method, path string, body any) (map[string]json.RawMessage, error) {
	var payload *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		payload = bytes.NewReader(raw)
	} else {
		payload = bytes.NewReader(nil)
	}
	request, err := http.NewRequestWithContext(b.ctx, method, strings.TrimRight(b.options.MinecraftURL, "/")+path, payload)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+b.options.MinecraftKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := minecraftClient.Do(request)
	if err != nil {
		warn("Could not reach the Minecraft add-on at %s: %v", b.options.MinecraftURL, err)
		return nil, refuse("I can't reach the Minecraft server add-on right now. Is it running?")
	}
	defer response.Body.Close()
	var result map[string]json.RawMessage
	_ = json.NewDecoder(response.Body).Decode(&result)
	switch {
	case response.StatusCode == http.StatusForbidden:
		warn("The Minecraft add-on refused the bot: check 'minecraft_key' (copy it from the Minecraft panel)")
		return nil, refuse("The Minecraft server didn't accept my key. Ask the server admin to check the bot's settings.")
	case response.StatusCode >= 400:
		var message string
		_ = json.Unmarshal(result["error"], &message)
		if message == "" {
			message = fmt.Sprintf("The Minecraft server refused (HTTP %d).", response.StatusCode)
		}
		return nil, refuse("%s", message)
	}
	return result, nil
}

func (b *Bot) minecraft(sender id.UserID, room id.RoomID, words []string) error {
	if !b.helpers[sender].Minecraft {
		return refuse("You don't have permission to manage Minecraft players.")
	}
	if len(words) == 0 {
		return refuse("Tell me what to do, like: !minecraft add Gamertag. Type !help for all commands.")
	}
	action := strings.ToLower(words[0])
	gamertag := strings.TrimSpace(strings.Join(words[1:], " "))
	if action != "list" && gamertag == "" {
		return refuse("Which player? Use their Xbox gamertag, like: !minecraft %s Gamertag", action)
	}

	switch action {
	case "list":
		result, err := b.minecraftCall(http.MethodGet, "/api/bot/players", nil)
		if err != nil {
			return err
		}
		var players []minecraftPlayer
		_ = json.Unmarshal(result["players"], &players)
		if len(players) == 0 {
			b.say(room, "Nobody has been added from chat yet.")
			return nil
		}
		lines := []string{"Players added from chat:"}
		for _, player := range players {
			line := fmt.Sprintf("%s — %s", player.Name, player.Role)
			if !player.Joined {
				line += " (hasn't joined yet)"
			}
			lines = append(lines, line)
		}
		b.say(room, strings.Join(lines, "\n"))
		return nil

	case "add", "allow":
		// Keep an operator an operator if they're added again
		return b.setPlayer(sender, room, gamertag, "")

	case "op", "operator":
		return b.setPlayer(sender, room, gamertag, "operator")

	case "member", "deop":
		return b.setPlayer(sender, room, gamertag, "member")

	case "remove", "delete", "kick":
		result, err := b.minecraftCall(http.MethodPost, "/api/bot/players/remove", map[string]string{"name": gamertag})
		if err != nil {
			return err
		}
		var removed minecraftPlayer
		_ = json.Unmarshal(result["removed"], &removed)
		info("%s removed Minecraft player %s", sender, removed.Name)
		b.say(room, fmt.Sprintf("%s removed %s from the Minecraft server.", sender, removed.Name))
		return nil
	}
	return refuse("I don't know '!minecraft %s'. Type !help for the Minecraft commands.", words[0])
}

func (b *Bot) setPlayer(sender id.UserID, room id.RoomID, gamertag, role string) error {
	body := map[string]string{"name": gamertag}
	if role != "" {
		body["role"] = role
	}
	result, err := b.minecraftCall(http.MethodPost, "/api/bot/players", body)
	if err != nil {
		return err
	}
	var player minecraftPlayer
	var created bool
	_ = json.Unmarshal(result["player"], &player)
	_ = json.Unmarshal(result["created"], &created)
	if player.Name == "" {
		return errors.New("the Minecraft add-on gave an empty reply")
	}
	info("%s set Minecraft player %s to %s", sender, player.Name, player.Role)

	var message string
	switch {
	case player.Role == "operator" && !player.Joined:
		message = fmt.Sprintf("%s can join the Minecraft server now, and becomes an operator the first time they do.", player.Name)
	case player.Role == "operator":
		message = fmt.Sprintf("%s is now an operator on the Minecraft server.", player.Name)
	case role == "member":
		message = fmt.Sprintf("%s is a normal player on the Minecraft server now.", player.Name)
	case created:
		message = fmt.Sprintf("%s can join the Minecraft server now.", player.Name)
	default:
		message = fmt.Sprintf("%s was already allowed on the Minecraft server.", player.Name)
	}
	b.say(room, fmt.Sprintf("%s: %s", sender, message))
	return nil
}
