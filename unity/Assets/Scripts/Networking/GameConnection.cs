using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Net.WebSockets;
using System.Runtime.InteropServices;
using System.Threading;
using System.Threading.Tasks;
using AOT;
using Game.V1;
using Google.Protobuf;

namespace Game.Networking
{
    /// <summary>
    /// Wraps a single ClientWebSocket connection to the server's /ws endpoint.
    /// Connect/send/receive all run on background tasks; ReceivedMessages is the
    /// only thread-safe handoff point and must be drained from the main thread
    /// (e.g. a MonoBehaviour's Update), matching Unity's single-threaded API rule.
    /// A web build has neither ClientWebSocket nor threads, so there the browser's own
    /// WebSocket (Plugins/WebGL/WebSocket.jslib) carries it, calling back on the main thread.
    /// </summary>
    public class GameConnection : IDisposable
    {
        public enum State { Disconnected, Connecting, Connected }

        public State CurrentState { get; private set; } = State.Disconnected;
        public readonly ConcurrentQueue<ServerMessage> ReceivedMessages = new();

        public event Action<Exception> OnError;
        public event Action OnDisconnected;

#if !UNITY_WEBGL || UNITY_EDITOR
        // Mirrors the server's own Client.Send buffer size (SendBufferSize in backend/internal/game/send.go);
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

            // Task.Run, not a plain call: started from the main thread, the loops would capture
            // Unity's synchronization context and resume once per frame, capping receives at about
            // one message a frame while the server sends 20+ a second, so a backlog builds.
            var token = _cts.Token;
            _ = Task.Run(() => ReceiveLoop(token));
            _ = Task.Run(() => SendLoop(token));
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
#else
        private delegate void SocketCallback(int id);
        private delegate void MessageCallback(int id, IntPtr data, int length);

        [DllImport("__Internal")]
        private static extern int WsConnect(string url, SocketCallback onOpen, MessageCallback onMessage, SocketCallback onClose);

        [DllImport("__Internal")]
        private static extern void WsSend(int id, byte[] data, int length);

        [DllImport("__Internal")]
        private static extern void WsClose(int id);

        // The jslib calls back into statics, so each live socket's id leads back to its
        // connection; a disposed connection leaves this map and its late callbacks are dropped.
        private static readonly Dictionary<int, GameConnection> Sockets = new();

        private int _id;
        private TaskCompletionSource<bool> _connecting;

        public Task ConnectAsync(string url)
        {
            if (CurrentState != State.Disconnected) return Task.CompletedTask;

            CurrentState = State.Connecting;
            _connecting = new TaskCompletionSource<bool>();
            _id = WsConnect(url, HandleOpen, HandleMessage, HandleClose);
            Sockets[_id] = this;
            return _connecting.Task;
        }

        public void Send(ClientMessage message)
        {
            if (CurrentState != State.Connected) return;
            var payload = message.ToByteArray();
            WsSend(_id, payload, payload.Length);
        }

        [MonoPInvokeCallback(typeof(SocketCallback))]
        private static void HandleOpen(int id)
        {
            if (!Sockets.TryGetValue(id, out var c)) return;
            c.CurrentState = State.Connected;
            c._connecting.TrySetResult(true);
        }

        [MonoPInvokeCallback(typeof(MessageCallback))]
        private static void HandleMessage(int id, IntPtr data, int length)
        {
            if (!Sockets.TryGetValue(id, out var c)) return;
            var bytes = new byte[length];
            Marshal.Copy(data, bytes, 0, length);
            try
            {
                c.ReceivedMessages.Enqueue(ServerMessage.Parser.ParseFrom(bytes));
            }
            catch (Exception e)
            {
                c.OnError?.Invoke(e);
            }
        }

        // The browser reports no reason for a failed connect, only that the socket closed, so a
        // close before open is the connect error and a close after it is the disconnect.
        [MonoPInvokeCallback(typeof(SocketCallback))]
        private static void HandleClose(int id)
        {
            if (!Sockets.Remove(id, out var c)) return;
            var wasOpen = c.CurrentState == State.Connected;
            c.CurrentState = State.Disconnected;
            if (wasOpen)
            {
                c.OnDisconnected?.Invoke();
                return;
            }
            c.OnError?.Invoke(new Exception("websocket connect failed"));
            c._connecting.TrySetResult(false);
        }

        public void Dispose()
        {
            if (Sockets.Remove(_id)) WsClose(_id);
            _connecting?.TrySetResult(false);
            CurrentState = State.Disconnected;
        }
#endif
    }
}
