# Unity client conventions

- Unity 6000.6.2f1, URP 2D template, new Input System only, Steam desktop target
- C# proto output generated via `protoc --csharp_out`; game code in `Assets/Scripts/{Networking,Client}`
- Never edit `Assets/Scripts/Generated/` by hand; edit `backend/api/proto/` and run `make proto` from the repo root
- After editing client code, recompile through the Unity MCP server; `make client-linux` / `make client` build players and need the Editor closed
- Server y is down, Unity world is `(x, -y)`
- The client identifies itself as `WorldSnapshot.players[0]`
- Cents arrive over the wire; format as dollars only for display (`GameState.MoneyCents`). Order-size multipliers come from `PriceQuote.orders`, never hard-coded
- Deadlines (phase timer, receipt toast) use `Time.realtimeSinceStartup`, not `Time.time`: the frame clock is clamped by `maximumDeltaTime` and falls behind while the window is unfocused. Animation phase keeps using the frame clock
- Player art is `Assets/Art/Player/{idle,walk}.png` (rows = 8 directions, columns = frames), see `DirectionalAnimationSet`
- Commodity icons are `Assets/Art/Commodities/<name>.png`, mapped by the `CommodityIcons` asset; Game > Commodity Art > Create Missing Icons never overwrites real art
