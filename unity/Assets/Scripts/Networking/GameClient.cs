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
        [SerializeField] private string host = "127.0.0.1";
        [SerializeField] private int port = 8080;
        [SerializeField] private bool connectOnStart = true;

        public GameConnection Connection { get; private set; }
        public bool IsConnected => Connection != null && Connection.CurrentState == GameConnection.State.Connected;

        public event Action<WorldSnapshot> OnWorldSnapshot;
        public event Action<MarketState> OnMarketState;
        public event Action<InitialGameState> OnInitialState;
        public event Action<PlayerInventory> OnPlayerInventory;
        public event Action<TradeReceipt> OnTradeReceipt;
        public event Action<PowerUpSpawned> OnPowerUpSpawned;
        public event Action<PowerUpDespawned> OnPowerUpDespawned;
        public event Action<GameStatus> OnGameStatus;
        public event Action<JoinRejected> OnJoinRejected;
        public event Action<GameOver> OnGameOver;

        private async void Start()
        {
            Connection = new GameConnection();
            Connection.OnError += e => Debug.LogError($"[GameClient] error: {e}");
            Connection.OnDisconnected += () => Debug.Log("[GameClient] disconnected");

            if (connectOnStart)
            {
                await Connection.ConnectAsync($"ws://{host}:{port}/ws");
            }
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
                case ServerMessage.MsgOneofCase.GameStatus:
                    OnGameStatus?.Invoke(message.GameStatus);
                    break;
                case ServerMessage.MsgOneofCase.JoinRejected:
                    OnJoinRejected?.Invoke(message.JoinRejected);
                    break;
                case ServerMessage.MsgOneofCase.GameOver:
                    OnGameOver?.Invoke(message.GameOver);
                    break;
            }
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
            Connection?.Send(new ClientMessage { JoinGame = new JoinGame { Id = code } });
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
