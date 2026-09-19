package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/alcares/mmoserver/internal/game"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Allow browser connections from localhost during development
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWS(world *game.World, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed: %v", err)
		return
	}

	client := &game.Client{
		Conn: conn,
		Send: make(chan []byte, 32),
	}

	player, err := world.Join(client)
	if err != nil {
		log.Printf("Rejected client: %v", err)
		conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
		)
		conn.Close()
		return
	}

	log.Printf("Player %d (%s) joined at (%.1f, %.1f)", player.ID, player.Name, player.Pos.X, player.Pos.Y)

	// Spin up dedicated reader and writer routines
	go client.WritePump()
	go client.ReadPump(world)
}

func main() {
	world := game.NewWorld(rand.New(rand.NewSource(time.Now().Unix())))

	go world.Run()
	fmt.Println("Server initialized")

	// Static file delivery
	http.Handle("/", http.FileServer(http.Dir("./web")))
	// Serve the proto sources under the same relative paths used by their
	// `import "game/v1/...proto"` statements, so protobufjs can resolve them.
	http.Handle("/game/", http.StripPrefix("/game/", http.FileServer(http.Dir("./api/proto/game"))))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(world, w, r)
	})

	log.Println("Listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
