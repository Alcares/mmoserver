# Unity client conventions

- Unity 6000.6.2f1, URP 2D template, new Input System only, Steam desktop target
- C# proto output generated via `protoc --csharp_out`; game code in `Assets/Scripts/{Networking,Client}`
- Server y is down, Unity world is `(x, -y)`
- Player art is `Assets/Art/Player/{idle,walk}.png` (rows = 8 directions, columns = frames), see `DirectionalAnimationSet`
- Commodity icons are `Assets/Art/Commodities/<name>.png`, mapped by the `CommodityIcons` asset; Game > Commodity Art > Create Missing Icons never overwrites real art
