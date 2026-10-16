// minecraft-bot-api: lets the Matrix Helper Bot add, promote and remove
// players by Xbox gamertag.
//
// Bedrock keeps allowlist.json and permissions.json in /data; this edits them
// and has the running server reload them through its console (send-command).
// A gamertag is enough for the allowlist: Bedrock fills in the Xbox ID (xuid)
// when the player first joins and logs "Player connected: <name>, xuid: <id>".
// permissions.json is keyed by xuid only, so this watches the log for that
// line and applies a waiting operator role then. Players added here are also
// kept in players.json, with the role they should have.
//
// Only Home Assistant's internal network may connect, with the key from the
// add-on's options.
package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
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
	gamertag  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]| [A-Za-z0-9]){0,15}(?:#[0-9]{1,5})?$`)
	connected = regexp.MustCompile(`Player connected: (.+?), xuid: (\d+)`)
	internal  = mustCIDR("172.30.32.0/23")

	mu sync.Mutex
)

type Player struct {
	Name string `json:"name"`
	Xuid string `json:"xuid,omitempty"`
	Role string `json:"role"`
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func mustCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

func path(name string) string { return filepath.Join(dataDir, name) }

// ----- files -----

func readJSON(file string, into any) {
	raw, err := os.ReadFile(path(file))
	if err == nil {
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

// apply writes one player's change (old and/or new) into Bedrock's
// allowlist.json and permissions.json, then has the server reload them
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
	matches := func(entry map[string]any) bool {
		name, _ := entry["name"].(string)
		xuid, _ := entry["xuid"].(string)
		for _, n := range names {
			if strings.EqualFold(n, name) {
				return true
			}
		}
		for _, x := range xuids {
			if x == xuid {
				return true
			}
		}
		return false
	}

	var allowlist, kept []map[string]any
	readJSON("allowlist.json", &allowlist)
	for _, entry := range allowlist {
		if !matches(entry) {
			kept = append(kept, entry)
		}
	}
	if new != nil {
		entry := map[string]any{"ignoresPlayerLimit": false, "name": new.Name}
		if new.Xuid != "" {
			entry["xuid"] = new.Xuid
		}
		kept = append(kept, entry)
	}
	if kept == nil {
		kept = []map[string]any{}
	}
	if err := writeJSON("allowlist.json", kept); err != nil {
		return err
	}

	var permissions, keptPermissions []map[string]any
	readJSON("permissions.json", &permissions)
	for _, entry := range permissions {
		if xuid, _ := entry["xuid"].(string); !contains(xuids, xuid) {
			keptPermissions = append(keptPermissions, entry)
		}
	}
	if new != nil && new.Xuid != "" {
		keptPermissions = append(keptPermissions, map[string]any{"xuid": new.Xuid, "permission": new.Role})
	}
	if keptPermissions == nil {
		keptPermissions = []map[string]any{}
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

// ----- the server log -----

func recordConnection(name, xuid string) {
	mu.Lock()
	defer mu.Unlock()
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
				if match := connected.FindStringSubmatch(line); match != nil {
					recordConnection(strings.TrimSpace(match[1]), match[2])
				}
			}
		}
		file.Close()
	}
}

// ----- HTTP -----

func public(player Player) map[string]any {
	return map[string]any{"name": player.Name, "role": player.Role, "joined": player.Xuid != ""}
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func refuse(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}

func authorized(r *http.Request, key string) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !(ip.IsLoopback() || internal.Contains(ip)) {
		return false
	}
	supplied := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(key)) == 1
}

func handler(key string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bot/players", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		players := loadPlayers()
		mu.Unlock()
		list := []map[string]any{}
		for _, player := range players {
			list = append(list, public(player))
		}
		reply(w, http.StatusOK, map[string]any{"players": list})
	})
	mux.HandleFunc("POST /api/bot/players", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string  `json:"name"`
			Role *string `json:"role"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		name := strings.TrimSpace(body.Name)
		if !gamertag.MatchString(name) {
			refuse(w, http.StatusBadRequest, "That doesn't look like an Xbox gamertag.")
			return
		}
		role := ""
		if body.Role != nil {
			role = strings.ToLower(strings.TrimSpace(*body.Role))
			if role != "member" && role != "operator" {
				refuse(w, http.StatusBadRequest, "Role must be member or operator.")
				return
			}
		}
		mu.Lock()
		defer mu.Unlock()
		players := loadPlayers()
		i := find(players, name)
		var old *Player
		if i >= 0 {
			copied := players[i]
			old = &copied
			players[i].Name = name
		} else {
			players = append(players, Player{Name: name, Role: "member"})
			i = len(players) - 1
		}
		// No role: keep an existing player's role (new players are members)
		if role != "" {
			players[i].Role = role
		}
		if err := writeJSON("players.json", players); err != nil {
			refuse(w, http.StatusInternalServerError, "Could not save the player list.")
			return
		}
		if err := apply(old, &players[i]); err != nil {
			refuse(w, http.StatusInternalServerError, "Could not update the allowlist.")
			return
		}
		log.Printf("[INFO] Chat: %s is allowed (%s)", name, players[i].Role)
		reply(w, http.StatusOK, map[string]any{"player": public(players[i]), "created": old == nil})
	})
	mux.HandleFunc("POST /api/bot/players/remove", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, http.StatusBadRequest, "Bad request.")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		players := loadPlayers()
		i := find(players, strings.TrimSpace(body.Name))
		if i < 0 {
			refuse(w, http.StatusNotFound, "That player wasn't added from chat.")
			return
		}
		removed := players[i]
		players = append(players[:i], players[i+1:]...)
		if err := writeJSON("players.json", players); err != nil {
			refuse(w, http.StatusInternalServerError, "Could not save the player list.")
			return
		}
		if err := apply(&removed, nil); err != nil {
			refuse(w, http.StatusInternalServerError, "Could not update the allowlist.")
			return
		}
		console(`kick "` + removed.Name + `" Removed from the allowlist`)
		log.Printf("[INFO] Chat: %s was removed", removed.Name)
		reply(w, http.StatusOK, map[string]any{"removed": public(removed)})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, key) {
			refuse(w, http.StatusForbidden, "Forbidden.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func main() {
	log.SetFlags(0)
	key := os.Getenv("BOT_KEY")
	if len(key) < 16 {
		log.Fatal("[ERROR] BOT_KEY must be at least 16 characters")
	}
	address := envOr("BOT_API_ADDRESS", ":8790")
	go watchLog()
	server := &http.Server{Addr: address, Handler: handler(key), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("[INFO] Matrix Helper Bot access on port %s (Home Assistant's internal network only)", strings.TrimPrefix(address, ":"))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[ERROR] Bot access stopped: %v", err)
	}
}
