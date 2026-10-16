// minecraft-bot-api: the add-on's web panel, Home Assistant entities and the
// Matrix Helper Bot's access to players.
//
//   - Panel (Home Assistant ingress, port 8791): server status, settings and
//     every allowed player, with add, change-role and remove.
//   - Home Assistant entities over MQTT discovery (ha.go), and the add-on's
//     dashboard (dashboard.go).
//   - The Matrix Helper Bot's requests, over MQTT too (botapi.go): add,
//     re-role and remove players added from chat.
//
// Player logic is in players.go.
package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

func serve(name, address string, handler http.Handler) {
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[ERROR] %s stopped: %v", name, err)
	}
}

func main() {
	log.SetFlags(0)
	go watchLog()

	var tracked []string
	if list, ok := options()["tracked_players"].([]any); ok {
		for _, item := range list {
			if name, ok := item.(string); ok && gamertag.MatchString(strings.TrimSpace(name)) {
				tracked = append(tracked, strings.TrimSpace(name))
			}
		}
	}
	go startHomeAssistant(tracked)

	serve("The sidebar panel", envOr("PANEL_ADDRESS", ":8791"), panelHandler())
}
