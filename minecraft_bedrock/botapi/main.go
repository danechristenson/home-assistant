// minecraft-bot-api: the add-on's sidebar panel, and the Matrix Helper Bot's
// access to players.
//
//   - Panel (Home Assistant ingress, port 8791): server status, settings and
//     every allowed player, with add, change-role and remove.
//   - Bot API (port 8790, only with BOT_KEY): add, re-role and remove players
//     added from chat.
//
// Player logic is in players.go.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
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

	if key := os.Getenv("BOT_KEY"); len(key) >= 16 {
		address := envOr("BOT_API_ADDRESS", ":8790")
		log.Printf("[INFO] Matrix Helper Bot access on port %s (Home Assistant's internal network only)", address[1:])
		go serve("Matrix Helper Bot access", address, botHandler(key))
	}

	serve("The sidebar panel", envOr("PANEL_ADDRESS", ":8791"), panelHandler())
}
