// The browser side of GameConnection's web build: one JS WebSocket per id, calling back into
// the C# statics passed to WsConnect. Binary frames only, like the server sends.
var WebSocketBridge = {
  $WsState: { sockets: {}, nextId: 1 },

  WsConnect: function (urlPtr, onOpen, onMessage, onClose) {
    var id = WsState.nextId++;
    var ws = new WebSocket(UTF8ToString(urlPtr));
    ws.binaryType = "arraybuffer";
    ws.onopen = function () {
      {{{ makeDynCall('vi', 'onOpen') }}}(id);
    };
    ws.onmessage = function (e) {
      if (!(e.data instanceof ArrayBuffer)) return;
      var bytes = new Uint8Array(e.data);
      var ptr = _malloc(bytes.length);
      // After _malloc, which may have grown the heap and replaced HEAPU8.
      HEAPU8.set(bytes, ptr);
      {{{ makeDynCall('viii', 'onMessage') }}}(id, ptr, bytes.length);
      _free(ptr);
    };
    ws.onclose = function () {
      delete WsState.sockets[id];
      {{{ makeDynCall('vi', 'onClose') }}}(id);
    };
    WsState.sockets[id] = ws;
    return id;
  },

  WsSend: function (id, ptr, length) {
    var ws = WsState.sockets[id];
    if (ws && ws.readyState === WebSocket.OPEN) ws.send(HEAPU8.slice(ptr, ptr + length));
  },

  // Closed from C#, which has already forgotten the id, so no callbacks fire after this.
  WsClose: function (id) {
    var ws = WsState.sockets[id];
    if (!ws) return;
    delete WsState.sockets[id];
    ws.onopen = ws.onmessage = ws.onclose = null;
    ws.close();
  },
};

autoAddDeps(WebSocketBridge, "$WsState");
mergeInto(LibraryManager.library, WebSocketBridge);
