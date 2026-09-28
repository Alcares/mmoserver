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
)

// logPath and webClientDir are relative to the repo root the binary runs from.
const (
	logPath      = "backend/server.log"
	webClientDir = "unity/Builds/Web"
)

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
		MinPlayers:     2,
		MaxPlayers:     3,
		StartCountdown: 3 * time.Second,
		Duration:       5 * time.Minute,
		LobbyTTL:       game.DefaultLobbyTTL,
		Logger:         slog.New(slog.NewJSONHandler(tradeLog, nil)),
		RandomEvents:   true,
	}

	// One instance serves every bot: Act only reads the weights and allocates its own scratch.
	if policy, err := bot.LoadMLPPolicy("rl-training/policy.pb"); err != nil {
		slog.Warn("no trained policy; bots run the scripted baseline", "err", err)
	} else {
		defaults.BotPolicy = policy
	}

	gameMaster := game.NewMaster(rand.New(rand.NewSource(time.Now().UnixNano())))
	slog.Info("server initialized")

	upgrader := newUpgrader()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(gameMaster, defaults, upgrader, w, r)
	})
	// Same origin as /ws, so the web client needs no CORS and no configured server address.
	http.Handle("/", webClient(webClientDir))

	slog.Info("listening", "url", "ws://localhost:8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
