package main

// Players and Bedrock's player files.
//
// Bedrock keeps allowlist.json ({name, xuid}) and permissions.json ({xuid,
// permission}) in /data. A gamertag is enough for the allowlist: Bedrock
// fills in the Xbox ID (xuid) when the player first joins and logs "Player
// connected: <name>, xuid: <id>". permissions.json is keyed by xuid only, so
// a role chosen before someone's first join waits in players.json and is
// applied when the log shows them joining.
//
// players.json records players added or changed through this service, with
// where from: "chat" (the Matrix Helper Bot) or "panel" (the sidebar). The
// bot may only change players added from chat; the panel may change anyone.

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	dataDir     = envOr("DATA_DIR", "/data")
	sendCommand = envOr("SEND_COMMAND", "/usr/local/bin/send-command")

	// Xbox gamertags: letters, numbers and single spaces, optionally with the
	// #1234 suffix newer gamertags carry
	gamertag     = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]| [A-Za-z0-9]){0,15}(?:#[0-9]{1,5})?$`)
	connected    = regexp.MustCompile(`Player connected: (.+?), xuid: (\d+)`)
	disconnected = regexp.MustCompile(`Player disconnected: (.+?), xuid: (\d+)`)

	roles = []string{"visitor", "member", "operator"}

	mu sync.Mutex
	// What the log says right now (since this start)
	live = struct {
		sync.Mutex
		running bool
		online  map[string]bool
	}{online: map[string]bool{}}
)

const (
	fromChat  = "chat"
	fromPanel = "panel"
)

type Player struct {
	Name   string `json:"name"`
	Xuid   string `json:"xuid,omitempty"`
	Role   string `json:"role"`
	Source string `json:"source,omitempty"` // "chat" (default) or "panel"
}

func (p Player) source() string {
	if p.Source == "" {
		return fromChat
	}
	return p.Source
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func path(name string) string { return filepath.Join(dataDir, name) }

func validRole(role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// ----- files -----

func readJSON(file string, into any) {
	if raw, err := os.ReadFile(path(file)); err == nil {
		_ = json.Unmarshal(raw, into)
	}
}

func writeJSON(file string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary := path(file) + ".new"
	if err := os.WriteFile(temporary, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path(file))
}

func loadPlayers() []Player {
	var players []Player
	readJSON("players.json", &players)
	return players
}

func find(players []Player, name string) int {
	for i, player := range players {
		if strings.EqualFold(player.Name, name) {
			return i
		}
	}
	return -1
}

type allowEntry = map[string]any

func entryName(entry allowEntry) string { name, _ := entry["name"].(string); return name }
func entryXuid(entry allowEntry) string { xuid, _ := entry["xuid"].(string); return xuid }

// allowlisted finds a player on Bedrock's allowlist by name
func allowlisted(name string) (allowEntry, bool) {
	var allowlist []allowEntry
	readJSON("allowlist.json", &allowlist)
	for _, entry := range allowlist {
		if strings.EqualFold(entryName(entry), name) {
			return entry, true
		}
	}
	return nil, false
}

// apply writes one player's change (old and/or new) into allowlist.json and
// permissions.json, then has the server reload them
func apply(old, new *Player) error {
	var names, xuids []string
	for _, player := range []*Player{old, new} {
		if player != nil {
			names = append(names, player.Name)
			if player.Xuid != "" {
				xuids = append(xuids, player.Xuid)
			}
		}
	}
	matches := func(entry allowEntry) bool {
		for _, n := range names {
			if strings.EqualFold(n, entryName(entry)) {
				return true
			}
		}
		for _, x := range xuids {
			if x == entryXuid(entry) {
				return true
			}
		}
		return false
	}

	var allowlist []allowEntry
	readJSON("allowlist.json", &allowlist)
	kept := []allowEntry{}
	for _, entry := range allowlist {
		if !matches(entry) {
			kept = append(kept, entry)
		}
	}
	if new != nil {
		entry := allowEntry{"ignoresPlayerLimit": false, "name": new.Name}
		if new.Xuid != "" {
			entry["xuid"] = new.Xuid
		}
		kept = append(kept, entry)
	}
	if err := writeJSON("allowlist.json", kept); err != nil {
		return err
	}

	var permissions []allowEntry
	readJSON("permissions.json", &permissions)
	keptPermissions := []allowEntry{}
	for _, entry := range permissions {
		if !contains(xuids, entryXuid(entry)) {
			keptPermissions = append(keptPermissions, entry)
		}
	}
	if new != nil && new.Xuid != "" {
		keptPermissions = append(keptPermissions, allowEntry{"xuid": new.Xuid, "permission": new.Role})
	}
	if err := writeJSON("permissions.json", keptPermissions); err != nil {
		return err
	}

	console("allowlist reload")
	console("permission reload")
	return nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// console sends a command to the running server; if it isn't running, the
// files are read at the next start anyway
func console(command string) {
	if err := exec.Command(sendCommand, command).Run(); err != nil {
		log.Printf("[WARN] Could not send '%s' to the server (not running?): %v", strings.Fields(command)[0], err)
	}
}

// setPlayer adds a player or changes their role. role "" keeps an existing
// player's role (new players are members). With source "chat", only players
// added from chat may be changed.
func setPlayer(name, role, source string) (Player, bool, error) {
	mu.Lock()
	defer mu.Unlock()
	players := loadPlayers()
	i := find(players, name)
	if source == fromChat {
		if i >= 0 && players[i].source() != fromChat {
			return Player{}, false, errManaged
		}
		if _, listed := allowlisted(name); i < 0 && listed {
			return Player{}, false, errManaged
		}
	}
	var old *Player
	if i >= 0 {
		copied := players[i]
		old = &copied
		players[i].Name = name
	} else {
		player := Player{Name: name, Role: "member", Source: source}
		// Someone already on the allowlist (e.g. from before): keep their
		// Xbox ID and current role
		if entry, listed := allowlisted(name); listed {
			player.Name = entryName(entry)
			player.Xuid = entryXuid(entry)
			if current := permissionOf(player.Xuid); current != "" {
				player.Role = current
			}
			copied := player
			old = &copied
		}
		players = append(players, player)
		i = len(players) - 1
	}
	if role != "" {
		players[i].Role = role
	}
	if err := writeJSON("players.json", players); err != nil {
		return Player{}, false, err
	}
	if err := apply(old, &players[i]); err != nil {
		return Player{}, false, err
	}
	log.Printf("[INFO] %s: %s is allowed (%s)", sourceName(source), players[i].Name, players[i].Role)
	return players[i], old == nil, nil
}

// removePlayer takes someone off the allowlist. With source "chat", only
// players added from chat may be removed.
func removePlayer(name, source string) (Player, error) {
	mu.Lock()
	defer mu.Unlock()
	players := loadPlayers()
	i := find(players, name)
	var removed Player
	switch {
	case i >= 0 && source == fromChat && players[i].source() != fromChat:
		return Player{}, errManaged
	case i >= 0:
		removed = players[i]
		players = append(players[:i], players[i+1:]...)
		if err := writeJSON("players.json", players); err != nil {
			return Player{}, err
		}
	case source == fromChat:
		if _, listed := allowlisted(name); listed {
			return Player{}, errManaged
		}
		return Player{}, errNotFound
	default:
		entry, listed := allowlisted(name)
		if !listed {
			return Player{}, errNotFound
		}
		removed = Player{Name: entryName(entry), Xuid: entryXuid(entry), Role: permissionOf(entryXuid(entry))}
	}
	if err := apply(&removed, nil); err != nil {
		return Player{}, err
	}
	console(`kick "` + removed.Name + `" Removed from the allowlist`)
	log.Printf("[INFO] %s: %s was removed", sourceName(source), removed.Name)
	return removed, nil
}

func sourceName(source string) string {
	if source == fromPanel {
		return "Panel"
	}
	return "Chat"
}

func permissionOf(xuid string) string {
	if xuid == "" {
		return ""
	}
	var permissions []allowEntry
	readJSON("permissions.json", &permissions)
	for _, entry := range permissions {
		if entryXuid(entry) == xuid {
			role, _ := entry["permission"].(string)
			return role
		}
	}
	return ""
}

type apiError struct {
	status  int
	message string
}

func (e apiError) Error() string { return e.message }

var (
	errManaged  = apiError{409, "That player is managed in the Minecraft panel in Home Assistant, not from chat."}
	errNotFound = apiError{404, "That player isn't on the allowlist."}
)

// ----- the server log -----

func recordConnection(name, xuid string) {
	mu.Lock()
	defer mu.Unlock()
	seen := map[string]string{}
	readJSON("seen.json", &seen)
	seen[strings.ToLower(name)] = time.Now().UTC().Format(time.RFC3339)
	_ = writeJSON("seen.json", seen)

	players := loadPlayers()
	i := find(players, name)
	if i < 0 || players[i].Xuid == xuid {
		return
	}
	old := players[i]
	players[i].Xuid = xuid
	if err := writeJSON("players.json", players); err == nil {
		_ = apply(&old, &players[i])
		log.Printf("[INFO] %s joined for the first time; their %s role now applies", name, players[i].Role)
	}
}

func readLine(line string) {
	switch {
	case strings.Contains(line, "Server started."):
		live.Lock()
		live.running, live.online = true, map[string]bool{}
		live.Unlock()
	case strings.Contains(line, "Stopping server") || strings.Contains(line, "Quit correctly"):
		live.Lock()
		live.running, live.online = false, map[string]bool{}
		live.Unlock()
	}
	if match := connected.FindStringSubmatch(line); match != nil {
		name := strings.TrimSpace(match[1])
		live.Lock()
		live.online[strings.ToLower(name)] = true
		live.Unlock()
		recordConnection(name, match[2])
	} else if match := disconnected.FindStringSubmatch(line); match != nil {
		live.Lock()
		delete(live.online, strings.ToLower(strings.TrimSpace(match[1])))
		live.Unlock()
	}
}

func watchLog() {
	logFile := path("run/bedrock.log")
	var offset int64
	for {
		time.Sleep(2 * time.Second)
		file, err := os.Open(logFile)
		if err != nil {
			offset = 0
			continue
		}
		if info, err := file.Stat(); err == nil && info.Size() < offset {
			offset = 0 // a new start truncated it
		}
		if _, err := file.Seek(offset, io.SeekStart); err == nil {
			reader := bufio.NewReader(file)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break // keep a partial line for next time
				}
				offset += int64(len(line))
				readLine(line)
			}
		}
		file.Close()
	}
}
