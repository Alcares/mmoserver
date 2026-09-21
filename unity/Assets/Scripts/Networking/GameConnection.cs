using System;
using System.Collections.Concurrent;
using System.IO;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;
using Game.V1;
using Google.Protobuf;

namespace Game.Networking
{
    /// <summary>
    /// Wraps a single ClientWebSocket connection to the server's /ws endpoint.
    /// Connect/send/receive all run on background tasks; ReceivedMessages is the
    /// only thread-safe handoff point and must be drained from the main thread
    /// (e.g. a MonoBehaviour's Update), matching Unity's single-threaded API rule.
    /// </summary>
    public class GameConnection : IDisposable
    {
        public enum State { Disconnected, Connecting, Connected }

        public State CurrentState { get; private set; } = State.Disconnected;
        public readonly ConcurrentQueue<ServerMessage> ReceivedMessages = new();

        public event Action<Exception> OnError;
        public event Action OnDisconnected;

        // Mirrors the server's own Client.Send buffer size (backend/internal/game/client.go);
        // outgoing sends are non-blocking and dropped when this fills up.
        private const int MaxOutgoingQueue = 32;

        private ClientWebSocket _socket;
        private CancellationTokenSource _cts;
        private readonly ConcurrentQueue<byte[]> _outgoing = new();

        public async Task ConnectAsync(string url)
        {
            if (CurrentState != State.Disconnected) return;

            CurrentState = State.Connecting;
            _socket = new ClientWebSocket();
            _cts = new CancellationTokenSource();

            try
            {
                await _socket.ConnectAsync(new Uri(url), _cts.Token);
                CurrentState = State.Connected;
            }
            catch (Exception e)
            {
                CurrentState = State.Disconnected;
                OnError?.Invoke(e);
                return;
            }

            _ = ReceiveLoop(_cts.Token);
            _ = SendLoop(_cts.Token);
        }

        public void Send(ClientMessage message)
        {
            if (CurrentState != State.Connected) return;
            if (_outgoing.Count >= MaxOutgoingQueue) return;
            _outgoing.Enqueue(message.ToByteArray());
        }

        private async Task SendLoop(CancellationToken token)
        {
            try
            {
                while (!token.IsCancellationRequested && _socket.State == WebSocketState.Open)
                {
                    if (_outgoing.TryDequeue(out var payload))
                    {
                        await _socket.SendAsync(new ArraySegment<byte>(payload), WebSocketMessageType.Binary, true, token);
                    }
                    else
                    {
                        await Task.Delay(10, token);
                    }
                }
            }
            catch (OperationCanceledException) { }
            catch (Exception e) { OnError?.Invoke(e); }
        }

        private async Task ReceiveLoop(CancellationToken token)
        {
            var buffer = new byte[8192];
            try
            {
                while (!token.IsCancellationRequested && _socket.State == WebSocketState.Open)
                {
                    using var ms = new MemoryStream();
                    WebSocketReceiveResult result;
                    do
                    {
                        result = await _socket.ReceiveAsync(new ArraySegment<byte>(buffer), token);
                        if (result.MessageType == WebSocketMessageType.Close)
                        {
                            CurrentState = State.Disconnected;
                            OnDisconnected?.Invoke();
                            return;
                        }
                        ms.Write(buffer, 0, result.Count);
                    } while (!result.EndOfMessage);

                    ReceivedMessages.Enqueue(ServerMessage.Parser.ParseFrom(ms.ToArray()));
                }
            }
            catch (OperationCanceledException) { }
            catch (Exception e)
            {
                CurrentState = State.Disconnected;
                OnError?.Invoke(e);
            }
        }

        public void Dispose()
        {
            _cts?.Cancel();
            _socket?.Dispose();
            CurrentState = State.Disconnected;
        }
    }
}
