package main

import (
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/alcares/mmoserver/backend/internal/bot"
	"github.com/alcares/mmoserver/backend/internal/game"
	"github.com/alcares/mmoserver/backend/internal/logging"
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
		slog.Error("upgrade failed", "err", err)
		return
	}
	client := game.NewWebsocketClient(conn)

	go client.WritePump()
	go client.ReadPump(master, defaults)
}

// logPath is relative to the repo root the binary runs from.
const logPath = "backend/server.log"

func main() {
	logger, logFile, err := logging.New(logPath)
	if err != nil {
		slog.Error("open log", "path", logPath, "err", err)
		os.Exit(1)
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	// One JSON object per line, appended across runs: the durable record of every order
	tradeLog, err := os.OpenFile("backend/events.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("open trade log", "err", err)
		os.Exit(1)
	}

	// Defaults for every game created on this server; a client's CreateGame may
	// override them, within the bounds WorldConfig.sanitize enforces.
	defaults := game.WorldConfig{
		MinPlayers:     5,
		StartCountdown: 3 * time.Second,
		Duration:       5 * time.Minute,
		LobbyTTL:       game.DefaultLobbyTTL,
		Logger:         slog.New(slog.NewJSONHandler(tradeLog, nil)),
	}

	// One instance serves every bot: Act only reads the weights and allocates its own scratch.
	if policy, err := bot.LoadMLPPolicy("rl-training/policy.pb"); err != nil {
		slog.Warn("no trained policy; bots run the scripted baseline", "err", err)
	} else {
		defaults.BotPolicy = policy
	}

	gameMaster := game.NewMaster(rand.New(rand.NewSource(time.Now().UnixNano())))
	slog.Info("server initialized")

	// Static file delivery
	http.Handle("/", http.FileServer(http.Dir("./web")))
	// Serve the proto sources under the same relative paths used by their
	// `import "game/v1/...proto"` statements, so protobufjs can resolve them.
	http.Handle("/game/", http.StripPrefix("/game/", http.FileServer(http.Dir("./backend/api/proto/game"))))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(gameMaster, defaults, w, r)
	})

	slog.Info("listening", "url", "http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
