# Logging
- Add monitoring
  - [ ] Loki
  - [ ] Grafana

# Robustness
- [x] Ping/pong so silently dead connections leave their game
- [ ] Rate limiting

# Game Feel
- [ ] Make the player suit golden if his net worth exceeded 10x his starting cash

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
- [ ] Rework core game loop and improve skill expression potential
  - [ ] Add a warning countdown (5s) before next random event
  - (maybe) [ ] Add power ups spawning in the middle of the arena (player should make a choice to risk camping for the power up)

# Training
- [ ] Train on GPU instead of CPU (only if the network grows in size enough to justify)
- [ ] Output a graph showing how to training progressed (success rate vs the success rate of the scripted policy)
- [ ] Train a trading model (BOT_TRAINING.md)

# Competitiveness
- [ ] Elo & Leaderboard (Compare every player against every other player exactly once (matrix of duels))
- [ ] Matchmaking once ELO is there (Players are gathered into a match keeping ELOs close to each other)

# Persistence (docs/ACCOUNTS.md)
- [ ] Rejoining a game (keep the player for a grace period, reattach on login)
- [ ] Account ID on Player, so Elo keys on the account

# Security
- [ ] Password travel unencrypted - use wss://
- [ ] Limit failed login attempts per connection
