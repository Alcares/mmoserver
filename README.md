# MMOServer

multiplayer trading game. you walk around a map between trading stations and buy low sell high, whoever has the most money when the timer runs out wins. there are bots too, trained with RL.

- `backend/` - go server. authoritative, 20 ticks/s, protobuf over websockets
- `unity/` - the actual client (unity 6)
- `web/` - crappy browser client, server hosts it
- `rl-training/` - python/PPO stuff that trains the bots against a go sim of the game
- `docs/` - design notes, read them if you care

## Setup

everything goes through the `Makefile`, read it

### train the bots
needs `uv`. first time do `make proto-py`, then

```
make train
```

takes a few minutes. pass flags with `ARGS`, like `make train ARGS="--timesteps 500000"`. it spits out `rl-training/policy.pb` (what the server bots use) and `rl-training/snapshots/` (a copy of the policy every so often, for spectating later). no policy.pb = bots just use the scripted one, fine too

### run the game
```
make server-start
```

runs on :8080 in the background, logs in `backend/server.log`. `make server-stop` to kill it. changed a `.proto`? `make proto` first

### build the client
close the unity editor first or it won't work

```
make client-linux   # or make client for linux + mac
```

ends up in `unity/Builds/`. `make run` = start the server + open the linux build. or just press play in the editor, whatever

### play
open the client (or http://localhost:8080 for the browser one). someone hits "create a new game" and gets a code, everyone else joins with the code. needs 5 players to start, press B in the lobby to add bots if you have no friends. WASD to move, E buy, Q sell, T changes order size. ctrl+R to go back to the menu

### spectate a finished training
watch the snapshots in order and see the bot go from useless to not useless

```
make server-stop    # spectator also wants :8080
make spectator
./backend/bin/spectator    # from the repo root
```

then open the unity client, it connects by itself. starts playing when you connect. `,` / `.` = slower / faster (up to 32x). the green circle is where the bot is trying to go. log in `backend/spectator.log`. settings are the constants at the bottom of `backend/cmd/spectator/main.go`, no flags, deal with it
