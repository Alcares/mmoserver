# MMOServer
Multiplayer trading game. You walk around a map between trading stations and buy low sell high, whoever has the most money when the timer runs out wins. There are bots too, trained with RL.

- `backend/` - go server. authoritative, 20 ticks/s, protobuf over websockets
- `unity/` - the actual client (unity 6)
- `rl-training/` - python/PPO stuff that trains the bots against a Go sim of the game
- `docs/` - design notes


## Setup
Everything goes through the `Makefile`, read it

### Train the bots
Needs `uv`. First time do `make proto-py` or `make-proto`, then
```
make train
```
Takes a few minutes. Pass flags with `ARGS`, like `make train ARGS="--timesteps 500000"`.
It spits out `rl-training/policy.pb` (what the server bots use) and `rl-training/snapshots/` (a copy of the policy every so often, for spectating later). 
No policy.pb = bots just use the scripted one, fine too

### Run the game
```
make server-start
```
Runs on :8080 in the background, logs in `backend/server.log`. `make server-stop` to kill it. Changed a `.proto`? `make proto` first.

### Build the client
Close the unity editor first or it won't work.
```
make client-linux   # or make client for linux + mac
```
Build ends up in `unity/Builds/`. `make run` = start the server + open the linux build.

### Play
Open the client. Someone hits "create a new game" and gets a code, everyone else joins with the code. needs minPlayers to start, press B in the lobby to add bots if you have no friends.
WASD to move, E buy, Q sell, T changes order size. ctrl+R to go back to the menu.

### Spectate a finished training
Watch the snapshots in order and see the bot go from useless to not useless.
```
make server-stop    # spectator also wants :8080
make spectator
./backend/bin/spectator    # from the repo root
```
Then open the unity client, it connects by itself. 
Starts playing when you connect. `,` / `.` = slower / faster (up to 32x).
The green circle is where the bot is trying to go. 
Logs in `backend/spectator.log`. 
Settings are the constants at the bottom of `backend/cmd/spectator/main.go`.
