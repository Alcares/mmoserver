using System;
using Game.V1;
using UnityEngine;

namespace Game.Networking
{
    /// <summary>
    /// MonoBehaviour front for GameConnection: owns the connection's lifetime,
    /// drains its received-message queue once per frame on the main thread, and
    /// fans messages out by their ServerMessage.MsgOneofCase.
    /// </summary>
    public class GameClient : MonoBehaviour
    {
        private const string UsernameKey = "account.username";

        [SerializeField] private string host = "127.0.0.1";
        [SerializeField] private int port = 8080;
        [SerializeField] private bool connectOnStart = true;

        public GameConnection Connection { get; private set; }
        public bool IsConnected => Connection != null && Connection.CurrentState == GameConnection.State.Connected;

        /// <summary>Whether this connection is logged in. The server ties a login to one
        /// connection, so every new connection starts logged out.</summary>
        public bool LoggedIn { get; private set; }
        /// <summary>The account last logged in to, kept across launches so the login form can
        /// offer it. Empty until the first successful login or sign-up.</summary>
        public string Username { get; private set; } = "";
        /// <summary>True between sending a login or sign-up and the server's answer.</summary>
        public bool AwaitingAccountReply { get; private set; }

        // The password stays in memory only, never on disk: it lets Reconnect log the new
        // connection in again. A failed login forgets it.
        private string _pendingUsername;
        private string _password;

        public event Action<WorldSnapshot> OnWorldSnapshot;
        public event Action<MarketState> OnMarketState;
        public event Action<InitialGameState> OnInitialState;
        public event Action<PlayerInventory> OnPlayerInventory;
        public event Action<TradeReceipt> OnTradeReceipt;
        public event Action<PowerUpSpawned> OnPowerUpSpawned;
        public event Action<PowerUpDespawned> OnPowerUpDespawned;
        public event Action<RandomEventOccurred> OnRandomEventOccurred;
        public event Action<RandomEventEnded> OnRandomEventEnded;
        public event Action<GameStatus> OnGameStatus;
        public event Action<JoinRejected> OnJoinRejected;
        public event Action<GameOver> OnGameOver;
        /// <summary>Only the spectator sends this, as each episode starts: the goal and which training snapshot plays.</summary>
        public event Action<Episode> OnEpisode;
        /// <summary>Only the spectator sends this: the playback speed now applied for every viewer.</summary>
        public event Action<PlaybackSpeed> OnPlaybackSpeed;
        /// <summary>Only the spectator sends this: every snapshot has played, and it hangs up next.</summary>
        public event Action<SpectatingOver> OnSpectatingOver;
        public event Action<LoginResult> OnLoginResult;
        public event Action<AccountCreateResult> OnAccountCreateResult;
        /// <summary>Raised by LogOut, so the account panel can start over.</summary>
        public event Action OnLoggedOut;

#if UNITY_WEBGL && !UNITY_EDITOR
        // A web build connects back to whatever served its page, over wss when the page is https.
        private string Url
        {
            get
            {
                var page = new Uri(Application.absoluteURL);
                return $"{(page.Scheme == "https" ? "wss" : "ws")}://{page.Authority}/ws";
            }
        }
#else
        private string Url => $"ws://{host}:{port}/ws";
#endif

        private void Awake()
        {
            Username = PlayerPrefs.GetString(UsernameKey, "");
        }

        private async void Start()
        {
            Connection = NewConnection();

            if (connectOnStart)
            {
                await Connection.ConnectAsync(Url);
            }
        }

        /// <summary>Drops this connection and opens a new one, which is what puts the client back
        /// in the lobby: the server binds a connection to one game for its whole lifetime
        /// (nothing clears WebsocketClient.World), so CreateGame on the old socket is ignored.
        /// Messages still queued on the old connection go with it, since Update only drains
        /// whichever connection this field points at. The login goes with the old connection
        /// too, so the new one logs in again with the password from this launch.</summary>
        public async void Reconnect()
        {
            Connection?.Dispose();
            Connection = NewConnection();
            LoggedIn = false;
            AwaitingAccountReply = false;
            await Connection.ConnectAsync(Url);

            if (_password != null) SendLogin(Username, _password);
        }

        /// <summary>Logs out by dropping the connection, the only way the server ends a login, and
        /// forgets the password so the new connection stays logged out. The username stays
        /// remembered for the login form.</summary>
        public void LogOut()
        {
            _password = null;
            Reconnect();
            OnLoggedOut?.Invoke();
        }

        private GameConnection NewConnection()
        {
            var connection = new GameConnection();
            connection.OnError += e => Debug.LogError($"[GameClient] error: {e}");
            connection.OnDisconnected += () => Debug.Log("[GameClient] disconnected");
            return connection;
        }

        private void Update()
        {
            if (Connection == null) return;

            while (Connection.ReceivedMessages.TryDequeue(out var message))
            {
                Dispatch(message);
            }
        }

        private void Dispatch(ServerMessage message)
        {
            switch (message.MsgCase)
            {
                case ServerMessage.MsgOneofCase.WorldSnapshot:
                    OnWorldSnapshot?.Invoke(message.WorldSnapshot);
                    break;
                case ServerMessage.MsgOneofCase.MarketState:
                    OnMarketState?.Invoke(message.MarketState);
                    break;
                case ServerMessage.MsgOneofCase.InitialState:
                    OnInitialState?.Invoke(message.InitialState);
                    break;
                case ServerMessage.MsgOneofCase.PlayerInventory:
                    OnPlayerInventory?.Invoke(message.PlayerInventory);
                    break;
                case ServerMessage.MsgOneofCase.Trade:
                    OnTradeReceipt?.Invoke(message.Trade);
                    break;
                case ServerMessage.MsgOneofCase.PowerUpSpawned:
                    OnPowerUpSpawned?.Invoke(message.PowerUpSpawned);
                    break;
                case ServerMessage.MsgOneofCase.PowerUpDespawned:
                    OnPowerUpDespawned?.Invoke(message.PowerUpDespawned);
                    break;
                case ServerMessage.MsgOneofCase.RandomEventOccurred:
                    OnRandomEventOccurred?.Invoke(message.RandomEventOccurred);
                    break;
                case ServerMessage.MsgOneofCase.RandomEventEnded:
                    OnRandomEventEnded?.Invoke(message.RandomEventEnded);
                    break;
                case ServerMessage.MsgOneofCase.GameStatus:
                    OnGameStatus?.Invoke(message.GameStatus);
                    break;
                case ServerMessage.MsgOneofCase.JoinRejected:
                    OnJoinRejected?.Invoke(message.JoinRejected);
                    break;
                case ServerMessage.MsgOneofCase.GameOver:
                    OnGameOver?.Invoke(message.GameOver);
                    break;
                case ServerMessage.MsgOneofCase.Episode:
                    OnEpisode?.Invoke(message.Episode);
                    break;
                case ServerMessage.MsgOneofCase.PlaybackSpeed:
                    OnPlaybackSpeed?.Invoke(message.PlaybackSpeed);
                    break;
                case ServerMessage.MsgOneofCase.SpectatingOver:
                    OnSpectatingOver?.Invoke(message.SpectatingOver);
                    break;
                case ServerMessage.MsgOneofCase.LoginResult:
                    SettleAccountReply(message.LoginResult.Rejection == LoginRejection.Unspecified);
                    OnLoginResult?.Invoke(message.LoginResult);
                    break;
                case ServerMessage.MsgOneofCase.AccountCreateResult:
                    SettleAccountReply(message.AccountCreateResult.Rejection == AccountCreateRejection.Unspecified);
                    OnAccountCreateResult?.Invoke(message.AccountCreateResult);
                    break;
            }
        }

        // A success logs this connection in and remembers the name for the next launch; a
        // failure forgets the password so Reconnect doesn't retry it.
        private void SettleAccountReply(bool ok)
        {
            AwaitingAccountReply = false;
            LoggedIn = ok;
            if (!ok)
            {
                _password = null;
                return;
            }

            Username = _pendingUsername;
            PlayerPrefs.SetString(UsernameKey, Username);
            PlayerPrefs.Save();
        }

        /// <summary>Logs this connection in; the server answers with LoginResult.</summary>
        public void SendLogin(string username, string password)
        {
            if (!IsConnected) return;
            BeginAccountRequest(username, password);
            Connection.Send(new ClientMessage { Login = new Login { Name = username, Password = password } });
        }

        /// <summary>Creates an account and logs this connection in to it; the server answers
        /// with AccountCreateResult.</summary>
        public void SendCreateAccount(string username, string password)
        {
            if (!IsConnected) return;
            BeginAccountRequest(username, password);
            Connection.Send(new ClientMessage { CreateAccount = new CreateAccount { Name = username, Password = password } });
        }

        private void BeginAccountRequest(string username, string password)
        {
            _pendingUsername = username;
            _password = password;
            AwaitingAccountReply = true;
        }

        /// <summary>Asks the server for a new game; it answers with InitialGameState carrying the join code.</summary>
        public void SendCreateGame()
        {
            Connection?.Send(new ClientMessage { CreateGame = new CreateGame() });
        }

        /// <summary>Joins an existing game by its code. The server trims and upper-cases it,
        /// and answers with either InitialGameState or JoinRejected.</summary>
        public void SendJoinGame(string code)
        {
            Connection?.Send(new ClientMessage { JoinGame = new JoinGame { GameId = code } });
        }

        /// <summary>Asks the server to add a policy-driven bot to the game this client is in.
        /// The name travels for later; the server ignores it today. Join only accepts new
        /// players before the round starts, so this does nothing once the phase is Running.</summary>
        public void SendSpawnBot(string name)
        {
            Connection?.Send(new ClientMessage { SpawnBot = new SpawnBot { Name = name } });
        }

        /// <summary>Asks the spectator to replay at this multiple of real time. It clamps the
        /// value and answers every viewer with the speed it applied; a game server ignores it.</summary>
        public void SendPlaybackSpeed(float speed)
        {
            Connection?.Send(new ClientMessage { SetPlaybackSpeed = new PlaybackSpeed { Speed = speed } });
        }

        public void SendMovement(float vx, float vy)
        {
            Connection?.Send(new ClientMessage
            {
                Input = new MovementCommand { Vx = vx, Vy = vy }
            });
        }

        public void SendTrade(TradeRequest trade)
        {
            Connection?.Send(new ClientMessage { Trade = trade });
        }

        private void OnDestroy()
        {
            Connection?.Dispose();
        }
    }
}
