package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/alcares/mmoserver/internal/game"
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

	world.Mu.Lock()
	player, err := world.AddPlayer(client)
	world.Mu.Unlock()

	if err != nil {
		log.Printf("Rejected client: %v", err)
		conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
		)
		conn.Close()
		return
	}

	log.Printf("Player %d (%s) joined at (%d, %d)", player.ID, player.Name, player.Pos.X, player.Pos.Y)

	// Spin up dedicated reader and writer routines
	go client.WritePump()
	go client.ReadPump(world)
}

func main() {
	world := game.NewWorld()

	go world.Run()
	fmt.Println("Server initialized")

	// Static file delivery
	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.HandleFunc("/game.proto", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./api/proto/game/v1/game.proto")
	})

	// Pass world to handleWS
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(world, w, r)
	})

	log.Println("Listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

}
