# Project conventions

## Commands
- Run: `make run` (server on :8080, serves `web/` as static files)
- Regenerate protobuf: `make proto` (regenerates both Go and C# output)
- Clean generated code: `make clean`
- No tests or linter yet

## Stack
- Go 1.26, gorilla/websocket, google.golang.org/protobuf
- Protobuf schemas in `api/proto/game/`, Go output in `gen/go/`
- Browser client in `web/`, plain JS, loads the `.proto` at runtime with protobufjs (no JS codegen)
- Unity client in `unity-client/` (Unity 6000.6.2f1, URP 2D template, new Input System only, Steam desktop target), C# proto output generated
  via `protoc --csharp_out`.

## Rules
- Never edit `gen/` or `unity-client/Assets/Scripts/Generated/` by hand; edit `api/proto/` and run `make proto`
- One `oneof` envelope per direction (`ClientMessage`, `ServerMessage`); add new messages as oneof members
- Only `World.Run` mutates game state; `ReadPump`s only push onto queues, `WritePump`s only drain `Client.Send`
- All sends are non-blocking; drop the message when a buffer is full
- Each tick sends every client a `WorldSnapshot` with only the players inside its FOV radius, found through `SpatialGrid`
- Trading: proto messages and `tradeQueue` exist, execution is still a TODO in `World.Run`
