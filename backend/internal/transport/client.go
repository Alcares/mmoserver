package transport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/game"
	"github.com/alcares/mmoserver/backend/internal/store"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// PongWait is how long a connection may go without a pong before it counts as dead; pings go out
// every PingPeriod, so a live client always answers in time however long it sits idle
const PongWait = 30 * time.Second
const PingPeriod = PongWait * 9 / 10

// MaxMessageSize caps an incoming frame in bytes; every ClientMessage is far smaller, so a bigger
// one closes the connection
const MaxMessageSize = 4096

// WebsocketClient represents an active WebSocket connection
type WebsocketClient struct {
	*game.SendQueue
	ID      uint32
	Conn    *websocket.Conn
	World   *game.World
	Session *store.Session
}

// NewWebsocketClient takes no ID: the world assigns one on join, and JoinWorld records it.
func NewWebsocketClient(conn *websocket.Conn) *WebsocketClient {
	return &WebsocketClient{SendQueue: game.NewSendQueue(), Conn: conn}
}

func (c *WebsocketClient) WritePump() {
	defer func(Conn *websocket.Conn) {
		if err := Conn.Close(); err != nil {
		}
	}(c.Conn)

	ping := time.NewTicker(PingPeriod)
	defer ping.Stop()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				// cmd/spectator closes Send on a healthy socket when the recap ends; a close frame lets the
				// client see a clean close instead of an error. The game server only closes Send after the
				// socket has died, where this write just fails.
				_ = c.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WebsocketClient) ReadPump(m *game.Master, a *store.Accounts, s *Sessions, cfg game.WorldConfig) {
	defer c.closeConnection(s)

	c.Conn.SetReadLimit(MaxMessageSize)
	err := c.Conn.SetReadDeadline(time.Now().Add(PongWait))
	if err != nil {
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	for {
		messageType, payload, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		var msg pb.ClientMessage
		if err := proto.Unmarshal(payload, &msg); err != nil {
			continue
		}

		if c.Session == nil {
			c.ReadSession(s, a, &msg)
			continue
		}

		if c.World == nil {
			c.ReadWorld(m, cfg, &msg)
			continue
		}

		// Commands are listed to only after session and world had been created
		c.ReadCommand(&msg)

	}
}

func (c *WebsocketClient) closeConnection(s *Sessions) {
	if c.World != nil {
		c.World.Leave(c.ID)
	}

	if c.Session != nil {
		s.remove(c.Session.AccountID, c)
	}
	close(c.Send)
	c.Conn.Close()
}

func (c *WebsocketClient) ReadSession(s *Sessions, a *store.Accounts, msg *pb.ClientMessage) {
	var session *store.Session

	switch cmd := msg.Cmd.(type) {
	case *pb.ClientMessage_CreateAccount:
		isOk := func(reason pb.AccountCreateRejection) bool {
			return reason == pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
		}

		name, password := cmd.CreateAccount.Name, cmd.CreateAccount.Password
		reason := validateUsername(name)
		if isOk(reason) {
			reason = validatePassword(password)
		}
		if isOk(reason) {
			var err error
			session, err = a.CreateNewAccount(context.Background(), name, password)
			reason = accountCreateRejection(err)
		}
		c.sendMsg(&pb.ServerMessage{
			Msg: &pb.ServerMessage_AccountCreateResult{
				AccountCreateResult: &pb.AccountCreateResult{Rejection: reason},
			},
		})
	case *pb.ClientMessage_Login:
		var err error
		session, err = a.Login(context.Background(), cmd.Login.Name, cmd.Login.Password)
		reason := loginRejection(err)
		c.sendMsg(&pb.ServerMessage{
			Msg: &pb.ServerMessage_LoginResult{
				LoginResult: &pb.LoginResult{Rejection: reason},
			},
		})
	}

	if session != nil {
		s.replace(session.AccountID, c)
		c.Session = session
	}
}

func (c *WebsocketClient) ReadWorld(m *game.Master, cfg game.WorldConfig, msg *pb.ClientMessage) {
	switch cmd := msg.Cmd.(type) {
	case *pb.ClientMessage_CreateGame:
		cfg.IsPublic = false
		world, err := m.Create(createConfig(cfg, cmd.CreateGame))
		if err != nil {
			c.rejectJoin(joinRejection(err))
			return
		}
		if err := c.joinWorld(world); err != nil {
			c.rejectJoin(joinRejection(err))
		}
	case *pb.ClientMessage_JoinGame:
		world, exists := m.GetPrivate(cmd.JoinGame.GameId)
		if !exists {
			c.rejectJoin(pb.JoinRejection_JOIN_REJECTION_GAME_NOT_FOUND)
			return
		}
		if err := c.joinWorld(world); err != nil {
			c.rejectJoin(joinRejection(err))
		}
	case *pb.ClientMessage_FindGame:
		// TODO: move somewhere more appropriate
		cfg.StartCountdown = 10 * time.Second
		if err := c.findGame(m, cfg); err != nil {
			c.rejectJoin(joinRejection(err))
		}
	}
}

// findGame joins a public game. A game can fill up or start between being found and joined,
// so that is retried once, and a matchmade player is not told about a game they never chose.
func (c *WebsocketClient) findGame(m *game.Master, cfg game.WorldConfig) error {
	var err error
	for range 2 {
		var world *game.World
		world, err = m.FindPublicGame(cfg)
		if err != nil {
			return err
		}
		err = c.joinWorld(world)
		if !errors.Is(err, game.ErrGameFull) && !errors.Is(err, game.ErrGameInProgress) {
			return err
		}
	}
	return err
}

func (c *WebsocketClient) ReadCommand(msg *pb.ClientMessage) {
	switch cmd := msg.Cmd.(type) {
	case *pb.ClientMessage_Input:
		c.World.EnqueueMovement(game.PlayerMovementInput{
			PlayerID: c.ID,
			Vx:       float64(cmd.Input.GetVx()),
			Vy:       float64(cmd.Input.GetVy()),
		})
	case *pb.ClientMessage_Trade:
		c.World.EnqueueTrade(game.TradeOrder{
			PlayerID:   c.ID,
			SequenceID: cmd.Trade.GetSequenceId(),
			Intent:     cmd.Trade.GetIntent(),
			Units:      uint64(cmd.Trade.GetUnits()),
			PriceCents: cmd.Trade.GetPriceCents(),
		})
	case *pb.ClientMessage_SpawnBot:
		// Refused in public games and once the lobby closes or fills up; SendSpawnBot documents that it does nothing then
		_, _ = c.World.SpawnBot()
	}
}

// joinRejection maps a create or join failure onto the reason sent to the client
func joinRejection(err error) pb.JoinRejection {
	switch {
	case errors.Is(err, game.ErrGameFull):
		return pb.JoinRejection_JOIN_REJECTION_GAME_FULL
	case errors.Is(err, game.ErrGameInProgress):
		return pb.JoinRejection_JOIN_REJECTION_GAME_IN_PROGRESS
	case errors.Is(err, game.ErrServerFull):
		return pb.JoinRejection_JOIN_REJECTION_SERVER_FULL
	default:
		return pb.JoinRejection_JOIN_REJECTION_UNSPECIFIED
	}
}

// loginRejection maps a login failure onto the reason sent to the client
func loginRejection(err error) pb.LoginRejection {
	switch {
	case err == nil:
		return pb.LoginRejection_LOGIN_REJECTION_UNSPECIFIED
	case errors.Is(err, store.ErrInvalidCredentials):
		return pb.LoginRejection_LOGIN_REJECTION_INVALID_CREDENTIALS
	default:
		slog.Error("login", "err", err)
		return pb.LoginRejection_LOGIN_REJECTION_SERVER_ERROR // DB broken, timeout, ...
	}
}

// accountCreateRejection maps a sign-up failure after validation onto the reason sent to the client
func accountCreateRejection(err error) pb.AccountCreateRejection {
	switch {
	case err == nil:
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_UNSPECIFIED
	case errors.Is(err, store.ErrUsernameTaken):
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_USERNAME_TAKEN
	default:
		slog.Error("create account", "err", err)
		return pb.AccountCreateRejection_ACCOUNT_CREATE_REJECTION_SERVER_ERROR
	}
}

// rejectJoin tells the client why it isn't in a game; the connection stays open in the
// lobby so it can retry, for instance after a mistyped code
func (c *WebsocketClient) rejectJoin(reason pb.JoinRejection) {
	c.sendMsg(&pb.ServerMessage{Msg: &pb.ServerMessage_JoinRejected{JoinRejected: &pb.JoinRejected{Reason: reason}}})
}

func (c *WebsocketClient) sendMsg(data *pb.ServerMessage) {
	payload, err := proto.Marshal(data)
	if err != nil {
		slog.Error("marshall failed", "err", err)
		return
	}

	select {
	case c.Send <- payload:
	default:
		// Buffer full
	}
}

// createConfig overlays the settings a client asked for onto the server's defaults.
// CreateGame carries none yet; when it does, an unset field keeps the default and
// NewWorld's sanitize clamps whatever the client sent.
func createConfig(defaults game.WorldConfig, req *pb.CreateGame) game.WorldConfig {
	cfg := defaults
	_ = req
	return cfg
}

func (c *WebsocketClient) joinWorld(world *game.World) error {
	player, err := world.Join(c, game.Account{AccountID: c.Session.AccountID, Name: c.Session.Name, Rating: c.Session.Rating})
	if err != nil {
		return err
	}

	c.ID = player.ID
	c.World = world

	return nil
}
