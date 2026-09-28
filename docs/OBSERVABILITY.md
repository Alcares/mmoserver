# Observability (proposal)

Ships the server's logs to Loki and reads them in Grafana, the two open `TODO.md` monitoring items.
No Go changes: the server already writes everything we want to files, and an agent tails them.

## What gets shipped

| File | Written by | Format | Contents |
|---|---|---|---|
| `backend/events.jsonl` | `WorldConfig.Logger`, through `World.log` | JSON, one object per line | per-game records: `join`, `leave`, `trade`, `event_start`, `event_end`; each carries `game`, `tick`, `phase` |
| `backend/server.log` | `slog.Default()` via `logging.New` | slog text (logfmt) | the server's own view: startup, `game started` / `game ended` from `Master`, errors |

## Design

```
server ──writes──▶ /backend/events.jsonl, /backend/server.log   (shared volume)
                               │
Alloy ──tails, parses──▶ Loki ◀──queries── Grafana
```

- **Alloy tails the files; the server never talks to Loki.** A slog Loki handler would tie the
  game loop to Loki's availability, and the Docker Loki logging driver only sees stdout/stderr,
  which `events.jsonl` never reaches. `events.jsonl` stays the durable record; Alloy resumes
  from its saved position after a restart.
- **Docker Compose, not Kubernetes.** `Master` keeps every game in memory, so there is one
  server process and nothing to scale out. Revisit when several game servers sit behind a
  lobby that routes game codes to them; the Alloy and Grafana config carry over unchanged.
- **Few labels.** Each distinct label combination is a Loki stream, so only low-cardinality
  fields become labels: `job` (`events` / `server`), `msg` and `level`. `game`, `player`,
  `name` and `tick` stay in the line and are filtered at query time:
  ```logql
  {job="events", msg="trade"} | json | game="Y5KUU6" | player="1"
  ```

## Files

| File | Change |
|---|---|
| `docker-compose.yml` (new) | `server`, `loki`, `alloy`, `grafana`; a named volume on the server's `/backend`, mounted read-only into Alloy; persistent volumes for Loki and Grafana data |
| `alloy/config.alloy` (new) | `local.file_match` on both files, `loki.process` with `stage.json` for `events` and `stage.logfmt` for `server`, `stage.labels` for `msg` and `level`, `loki.write` to `http://loki:3100` |
| `grafana/provisioning/datasources/loki.yml` (new) | Loki as the default data source, so it exists on first start |
| `loki/config.yml` (new) | retention, e.g. 30 days; the image's default config works without it, but disk grows forever |
| `Caddyfile` | a `grafana.{$DOMAIN}` block proxying to Grafana, only if it should be reachable from outside |
| `.env` | `GF_SECURITY_ADMIN_PASSWORD`, so a public Grafana is not on `admin/admin` |
| `Makefile` | `observability-up` / `observability-down` |

The server needs no change: in the image its working directory is `/`, so it writes
`/backend/events.jsonl` and `/backend/server.log`, which is where the shared volume mounts.

## Steps

1. **Local.** Compose file, Alloy and Grafana provisioning. Done when Grafana's Explore shows
   `{job="events"}` filling up while a game is played against the Compose `server`.
2. **Retention.** `loki/config.yml` with a retention period, and a check that old chunks go.
3. **Production.** Depends on the open questions below: move the box to `docker compose up -d`,
   add the Caddy block and the admin password.
4. **Dashboards** (separate work). Candidates: active games over time, games ended by reason,
   trades per minute per commodity, join refusals by reason.

## Open questions

- **How does production run today?** CI pushes to `ghcr.io`, but the repo doesn't say how the
  box starts the image. If it's a manual `docker run`, step 3 is switching that box to Compose,
  which is the biggest change here.
- **Raspberry Pi as the host?** The server cross-compiles to `linux/arm64`, but CI builds
  amd64 only; `.github/workflows/docker.yml` needs `setup-qemu-action` and
  `platforms: linux/amd64,linux/arm64`. Loki, Alloy and Grafana idle at roughly 300–500 MB,
  fine on a Pi 5 or a 4 GB+ Pi 4; they need an SSD rather than the SD card.
- **Is the home connection behind CGNAT?** If so, Caddy cannot get certificates through
  port forwarding, and a Cloudflare Tunnel or Tailscale Funnel replaces that part.
- **Should Grafana be public at all,** or only reachable over SSH or Tailscale?
- **Game end reason.** `finish()` is called for the lobby TTL, the round timer and everyone
  leaving, and nothing records which. `events.jsonl` has no game-end record at all today,
  only `server.log`'s `game ended` from `Master`. A `game_end` record with a `reason` would
  make the "games ended by reason" panel possible.
