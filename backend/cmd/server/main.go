package main

import (
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	game "github.com/alcares/mmoserver/backend/internal/game"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Allow browser connections from localhost during development
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// handleWS upgrades the connection and hands it to the pumps. The client starts in the
// lobby with no world; ReadPump joins it to one when a CreateGame or JoinGame arrives.
func handleWS(master *game.Master, defaults game.WorldConfig, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed: %v", err)
		return
	}

	client := &game.Client{
		Conn: conn,
		Send: make(chan []byte, game.SendBufferSize),
	}

	go client.WritePump()
	go client.ReadPump(master, defaults)
}

func main() {
	// One JSON object per line, appended across runs: the durable record of every order
	tradeLog, err := os.OpenFile("events.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("Trade log: %v", err)
	}

	// Defaults for every game created on this server; a client's CreateGame may
	// override them, within the bounds WorldConfig.sanitize enforces.
	defaults := game.WorldConfig{
		MinPlayers:     2,
		StartCountdown: 10 * time.Second,
		Duration:       5 * time.Minute,
		LobbyTTL:       game.DefaultLobbyTTL,
		Logger:         slog.New(slog.NewJSONHandler(tradeLog, nil)),
	}

	gameMaster := game.NewMaster(rand.New(rand.NewSource(time.Now().UnixNano())))
	fmt.Println("Server initialized")

	// Static file delivery
	http.Handle("/", http.FileServer(http.Dir("./web")))
	// Serve the proto sources under the same relative paths used by their
	// `import "game/v1/...proto"` statements, so protobufjs can resolve them.
	http.Handle("/game/", http.StripPrefix("/game/", http.FileServer(http.Dir("./backend/api/proto/game"))))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(gameMaster, defaults, w, r)
	})

	log.Println("Listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
