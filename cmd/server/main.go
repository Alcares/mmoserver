package main

import (
	"fmt"
	"log"
	"net/http"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"github.com/alcares/mmoserver/internal/game"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var upgrader = websocket.Upgrader{
	// Allow browser connections from localhost during development
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func buildInitialState(world *game.World) *pb.ServerMessage_InitialState {
	stations := make([]*pb.TradingStation, 0, len(world.Stations))
	for _, s := range world.Stations {
		stations = append(stations, s.ToProto())
	}

	return &pb.ServerMessage_InitialState{
		InitialState: &pb.InitialGameState{StationLayout: stations},
	}
}

func handleWS(world *game.World, initialState *pb.ServerMessage_InitialState, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed: %v", err)
		return
	}

	// Marshal the initial state before registering the player, so a failure
	// here touches neither the world state nor the connection.
	payload, err := proto.Marshal(&pb.ServerMessage{Msg: initialState})
	if err != nil {
		log.Printf("Marshal initial state: %v", err)
		conn.Close()
		return
	}

	client := &game.Client{
		Conn: conn,
		Send: make(chan []byte, 32),
	}

	world.Mu.Lock()
	player, err := world.AddPlayer(client)
	if err == nil {
		// Enqueue under the world lock: the tick loop only sends snapshots
		// while holding the same lock, so FIFO ordering guarantees the
		// initial state hits the wire before any snapshot for this player.
		client.Send <- payload

		// The starting balance only exists once AddPlayer assigns it, so this
		// has to be built per-connection rather than reused like initialState.
		inventoryPayload, err := proto.Marshal(&pb.ServerMessage{
			Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
		})
		if err != nil {
			log.Printf("Marshal initial inventory: %v", err)
		} else {
			client.Send <- inventoryPayload
		}
	}
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

	log.Printf("Player %d (%s) joined at (%.1f, %.1f)", player.ID, player.Name, player.Pos.X, player.Pos.Y)

	// Spin up dedicated reader and writer routines
	go client.WritePump()
	go client.ReadPump(world)
}

func main() {
	world := game.NewWorld()
	initialState := buildInitialState(world)

	go world.Run()
	fmt.Println("Server initialized")

	// Static file delivery
	http.Handle("/", http.FileServer(http.Dir("./web")))
	// Serve the proto sources under the same relative paths used by their
	// `import "game/v1/...proto"` statements, so protobufjs can resolve them.
	http.Handle("/game/", http.StripPrefix("/game/", http.FileServer(http.Dir("./api/proto/game"))))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(world, initialState, w, r)
	})

	log.Println("Listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
