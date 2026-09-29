# Logging
- Log events
  - [x] Log trades 
  - [x] Log players joining and leaving
  - [x] Log random events starting and ending
  - [x] Log game creation and game ending
- Add monitoring
  - [ ] Loki
  - [ ] Grafana

# Robustness
- [x] End the game when all players left
- [ ] Ping/pong so silently dead connections leave their game

# UI
- [x] Make support for players choosing a name
- [ ] Make the player suit golden if his net worth exceeded 10x his starting cash
- [x] Create a menu accessible on ESC press
  - [x] Log out - back to login screen (needs server changes)
  - [x] Resume (close the menu window)
  - [x] Back to start game screen - should also show after game naturally ended
  - [x] Quick the game entirely (with confirmation)

# Soundtrack (see docs/royalty_free_assets.md)
- Add sound effects for following actions
  - [ ] buy
  - [ ] sell
  - [ ] change multiplier
  - [ ] spawn bot
  - [ ] player joined
  - [ ] player left
  - [ ] station closed
  - [ ] station opened

  - [ ] bot reached goal

# Gameplay
- [x] Cap max players
- [ ] Add power ups spawning in the middle of the arena (player should make a choice to risk camping for the power up)
- Add random events
  - [x] Station closed
  - [x] Station taxed
  - [x] Supply flood
  - [x] Supply shortage
- [x] Increase the frequency of random events as game progresses (multiple can be on at the same time)
- [x] Allow for multiple events at the same time
- [ ] Add a warning countdown (5s) before next random event

# Training
- [ ] Train on GPU instead of CPU (only if the network grows in size enough to justify)
- [ ] Output a graph showing how to training progressed (success rate vs the success rate of the scripted policy)
- [ ] Train a trading model (BOT_TRAINING.md)

# Accessibility
- [x] Make the game playable on web
- [ ] Elo & Leaderboard (Compare every player against every other player exactly once (matrix of duels))
- [ ] Matchmaking once ELO is there

# Persistence (docs/ACCOUNTS.md)
- [x] github.com/sqlc-dev/sqlc for code gen
- [x] SQLite DB
- [x] Account creation proto wiring (username + password)
- [x] Apply migrations on startup
- [x] Unity login and sign-up screen
- [x] Username and password validation
- [ ] Rejoining a game (keep the player for a grace period, reattach on login)
- [ ] Account ID on Player, so Elo keys on the account

# Security
- [ ] Password travel unencrypted - use wss://
- [ ] Limit failed login attempts per connection
