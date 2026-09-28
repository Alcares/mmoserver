package main

import (
	"log/slog"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alcares/mmoserver/backend/internal/bot"
	"github.com/alcares/mmoserver/backend/internal/game"
	"github.com/alcares/mmoserver/backend/internal/logging"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Native clients send no Origin. Browsers may connect from a page this host served (the web
	// client, or a proxy in front that keeps Host) or from localhost, for Unity's Build And Run.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	},
}

// webClient serves the Unity web build. Its compressed files are named *.gz and have to go out
// with Content-Encoding and the type of the file inside, or the Unity loader refuses them.
func webClient(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, ok := strings.CutSuffix(r.URL.Path, ".gz"); ok {
			contentType := mime.TypeByExtension(path.Ext(name))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", contentType)
			// FileServer leaves out Content-Length once Content-Encoding is set, and the loader
			// wants it for its progress bar. Only for a whole file: a range has its own length.
			info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path.Clean("/"+r.URL.Path))))
			if err == nil && r.Header.Get("Range") == "" {
				w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
			}
		}
		files.ServeHTTP(w, r)
	})
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

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(gameMaster, defaults, w, r)
	})
	// Same origin as /ws, so the web client needs no CORS and no configured server address.
	http.Handle("/", webClient(webClientDir))

	slog.Info("listening", "url", "ws://localhost:8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
