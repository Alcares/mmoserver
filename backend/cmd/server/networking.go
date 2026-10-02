package main

import (
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alcares/mmoserver/backend/internal/game"
	"github.com/alcares/mmoserver/backend/internal/store"
	"github.com/alcares/mmoserver/backend/internal/transport"
	"github.com/gorilla/websocket"
)

func newUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
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
func handleWS(master *game.Master, accounts *store.Accounts, sessions *transport.Sessions, defaults game.WorldConfig, upgrader *websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("upgrade failed", "err", err)
		return
	}
	client := transport.NewWebsocketClient(conn)

	go client.WritePump()
	go client.ReadPump(master, accounts, sessions, defaults)
}
