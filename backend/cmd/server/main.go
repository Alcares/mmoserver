package main

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/alcares/mmoserver/backend/internal/bot"
	"github.com/alcares/mmoserver/backend/internal/game"
	"github.com/alcares/mmoserver/backend/internal/logging"
	"github.com/alcares/mmoserver/backend/internal/store"
	"github.com/alcares/mmoserver/backend/internal/transport"
)

// logPath and webClientDir are relative to the repo root the binary runs from.
const (
	logPath      = "backend/server.log"
	webClientDir = "unity/Builds/Web"
	dbFile       = "backend/mmo.db"
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

	// Defaults for private games; a client's CreateGame may override them, within the bounds
	// WorldConfig.sanitize enforces.
	private := game.WorldConfig{
		MinPlayers:     2,
		MaxPlayers:     3,
		StartCountdown: 3 * time.Second,
		Duration:       5 * time.Minute,
		LobbyTTL:       game.DefaultLobbyTTL,
		Logger:         slog.New(slog.NewJSONHandler(tradeLog, nil)),
		RandomEvents:   true,
		CloseWhenEmpty: true,
	}

	// One instance serves every bot: Act only reads the weights and allocates its own scratch.
	if policy, err := bot.LoadMLPPolicy("rl-training/policy.pb"); err != nil {
		slog.Warn("no trained policy; bots run the scripted baseline", "err", err)
	} else {
		private.BotPolicy = policy
	}

	// Matchmade games: the matchmaker picks who plays before they start, so the countdown is only time to get ready
	public := private
	public.IsPublic = true
	public.StartCountdown = 10 * time.Second

	c, err := store.Open(dbFile)
	if err != nil {
		slog.Error("open DB connection", "err", err)
		os.Exit(1)
	}
	defer c.Close()
	accounts := store.NewAccounts(c)

	sessions := transport.NewSessions()
	gameMaster := game.NewMaster(
		rand.New(rand.NewSource(time.Now().UnixNano())),
		func(r game.Result) {
			if r.RatingChanges != nil {
				if err := accounts.ApplyRatingChanges(context.Background(), r.RatingChanges); err != nil {
					slog.Error("save ratings", "game", r.GameID, "err", err)
				}
			}
		},
	)

	go gameMaster.RunMatchmaker(public)

	slog.Info("server initialized")

	upgrader := newUpgrader()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(gameMaster, accounts, sessions, private, upgrader, w, r)
	})
	// Same origin as /ws, so the web client needs no CORS and no configured server address.
	http.Handle("/", webClient(webClientDir))

	slog.Info("listening", "url", "ws://localhost:8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
