package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix/id"
)

// Minecraft players through the Minecraft Bedrock add-on: added by gamertag;
// their Xbox ID is learned when they first join, and roles apply from then on.

type minecraftPlayer struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Joined bool   `json:"joined"`
}

// The bot talks to the Minecraft add-on over MQTT: a request on
// minecraft_bedrock/bot/request, the reply on minecraft_bedrock/bot/response/<id>.
const (
	mcRequestTopic  = "minecraft_bedrock/bot/request"
	mcResponseTopic = "minecraft_bedrock/bot/response/"
)

var mcPending = struct {
	sync.Mutex
	waiting map[string]chan []byte
}{waiting: map[string]chan []byte{}}

// minecraftMQTT is the one broker connection, for the whole run of the bot
var minecraftMQTT *mqttClient

// startMinecraft connects to the MQTT broker (credentials from the
// Supervisor), retrying until the broker is up
func startMinecraft() {
	address, username, password, err := mqttCredentials()
	for wait := 10 * time.Second; err != nil; address, username, password, err = mqttCredentials() {
		if wait == 10*time.Second {
			warn("Minecraft commands are waiting for MQTT: %v", err)
		}
		time.Sleep(wait)
		if wait < 5*time.Minute {
			wait *= 2
		}
	}
	connectMinecraft(address, username, password)
}

func connectMinecraft(address, username, password string) {
	client := &mqttClient{address: address, username: username, password: password, clientID: "matrix_helper_bot",
		subscriptions: []string{mcResponseTopic + "+"}}
	client.onMessage = func(msg mqttMessage) {
		id := strings.TrimPrefix(msg.topic, mcResponseTopic)
		mcPending.Lock()
		waiting := mcPending.waiting[id]
		mcPending.Unlock()
		if waiting != nil {
			waiting <- msg.payload
		}
	}
	mcPending.Lock()
	minecraftMQTT = client
	mcPending.Unlock()
	go client.Run()
}

func mqttCredentials() (address, username, password string, err error) {
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return "", "", "", fmt.Errorf("no Supervisor token")
	}
	supervisor := os.Getenv("SUPERVISOR_URL")
	if supervisor == "" {
		supervisor = "http://supervisor"
	}
	request, _ := http.NewRequest(http.MethodGet, supervisor+"/services/mqtt", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return "", "", "", err
	}
	defer response.Body.Close()
	var body struct {
		Data struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&body) != nil || body.Data.Host == "" {
		return "", "", "", fmt.Errorf("no MQTT broker (install the Mosquitto broker add-on)")
	}
	return net.JoinHostPort(body.Data.Host, strconv.Itoa(body.Data.Port)), body.Data.Username, body.Data.Password, nil
}

// minecraftCall asks the Minecraft add-on to list, set or remove a player
func (b *Bot) minecraftCall(op, name, role string) (map[string]json.RawMessage, error) {
	mcPending.Lock()
	client := minecraftMQTT
	mcPending.Unlock()
	if client == nil {
		return nil, refuse("I can't reach the Minecraft server add-on (no MQTT broker yet).")
	}
	id := randomHex(12)
	waiting := make(chan []byte, 1)
	mcPending.Lock()
	mcPending.waiting[id] = waiting
	mcPending.Unlock()
	defer func() {
		mcPending.Lock()
		delete(mcPending.waiting, id)
		mcPending.Unlock()
	}()
	request := map[string]any{"id": id, "op": op, "name": name}
	if role != "" {
		request["role"] = role
	}
	raw, _ := json.Marshal(request)
	client.Publish(mcRequestTopic, raw, false)

	select {
	case payload := <-waiting:
		var result map[string]json.RawMessage
		_ = json.Unmarshal(payload, &result)
		var status int
		_ = json.Unmarshal(result["status"], &status)
		if status >= 400 {
			var message string
			_ = json.Unmarshal(result["error"], &message)
			if message == "" {
				message = fmt.Sprintf("The Minecraft server refused (%d).", status)
			}
			return nil, refuse("%s", message)
		}
		return result, nil
	case <-time.After(15 * time.Second):
		warn("The Minecraft add-on didn't answer a %s request", op)
		return nil, refuse("The Minecraft server add-on didn't answer. Is it running?")
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	}
}

// roleWords are the ways people name Bedrock's roles
var roleWords = map[string]string{
	"visitor": "visitor", "visitors": "visitor", "guest": "visitor",
	"member": "member", "members": "member", "user": "member", "player": "member", "normal": "member",
	"operator": "operator", "operators": "operator", "op": "operator", "ops": "operator",
}

// splitRole reads a gamertag and an optional role from the end: "Steve as
// operator", "Steve a user", "Steve to visitor". With connector set, a role
// only counts after one of those words, so gamertags ending in a role word
// ("Best Player") stay whole.
func splitRole(words []string, connector bool) (string, string) {
	n := len(words)
	if n < 2 {
		return strings.Join(words, " "), ""
	}
	role, ok := roleWords[strings.ToLower(words[n-1])]
	if !ok {
		return strings.Join(words, " "), ""
	}
	rest, connected := words[:n-1], false
	for len(rest) > 0 {
		last := strings.ToLower(rest[len(rest)-1])
		if last != "a" && last != "an" && last != "as" && last != "to" {
			break
		}
		rest, connected = rest[:len(rest)-1], true
	}
	if len(rest) == 0 || (connector && !connected) {
		return strings.Join(words, " "), ""
	}
	return strings.Join(rest, " "), role
}

func (b *Bot) minecraft(sender id.UserID, room id.RoomID, words []string) error {
	if !b.helpers[sender].Minecraft {
		return refuse("You don't have permission to manage Minecraft players.")
	}
	if len(words) == 0 {
		return refuse("Tell me what to do, like: %s add Gamertag. Say \"%s help\" for all commands.", b.name, b.name)
	}
	action := strings.ToLower(words[0])
	gamertag := strings.TrimSpace(strings.Join(words[1:], " "))
	if action != "list" && gamertag == "" {
		return refuse("Which player? Use their Xbox gamertag, like: %s %s Gamertag", b.name, action)
	}

	switch action {
	case "list":
		result, err := b.minecraftCall("list", "", "")
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
		// "add Steve as operator"; without a role, an existing player keeps
		// theirs and new players are members
		name, role := splitRole(words[1:], true)
		return b.setPlayer(sender, room, name, role)

	case "make", "set", "change":
		name, role := splitRole(words[1:], false)
		if role == "" {
			return refuse("Make them what? Like: %s make %s a member (or an operator, or a visitor).", b.name, gamertag)
		}
		return b.setPlayer(sender, room, name, role)

	case "op", "operator":
		return b.setPlayer(sender, room, gamertag, "operator")

	case "member", "deop", "user":
		return b.setPlayer(sender, room, gamertag, "member")

	case "visitor":
		return b.setPlayer(sender, room, gamertag, "visitor")

	case "remove", "delete", "kick":
		result, err := b.minecraftCall("remove", gamertag, "")
		if err != nil {
			return err
		}
		var removed minecraftPlayer
		_ = json.Unmarshal(result["removed"], &removed)
		info("%s removed Minecraft player %s", sender, removed.Name)
		b.say(room, fmt.Sprintf("%s removed %s from the Minecraft server.", sender, removed.Name))
		return nil
	}
	return refuse("I don't know '%s'. Say \"%s help\" for the Minecraft commands.", words[0], b.name)
}

func (b *Bot) setPlayer(sender id.UserID, room id.RoomID, gamertag, role string) error {
	result, err := b.minecraftCall("set", gamertag, role)
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

	roleText := map[string]string{"operator": "an operator", "member": "a member", "visitor": "a visitor"}[player.Role]
	var message string
	switch {
	case created && !player.Joined:
		message = fmt.Sprintf("%s can join the Minecraft server now, as %s once they first join.", player.Name, roleText)
	case created:
		message = fmt.Sprintf("%s can join the Minecraft server now, as %s.", player.Name, roleText)
	case role == "":
		message = fmt.Sprintf("%s was already allowed on the Minecraft server (as %s).", player.Name, roleText)
	case !player.Joined:
		message = fmt.Sprintf("%s will be %s the first time they join.", player.Name, roleText)
	default:
		message = fmt.Sprintf("%s is %s on the Minecraft server now.", player.Name, roleText)
	}
	b.say(room, fmt.Sprintf("%s: %s", sender, message))
	return nil
}
