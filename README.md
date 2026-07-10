# ideal-tribble

`ideal-tribble` is the backend for **Wally**, a web application that helps a
private Padel club track pairing analytics, match history, and player stats.

It pulls match data from [Playtomic](https://github.com/rafa-garcia/go-playtomic-api),
filters for club matches, and surfaces it through an authenticated web
dashboard.

> **History:** Wally began as a Slack bot. The Slack integration and the
> match-processing/notification machinery have been retired — this repo is now
> the web application and the data pipeline it needs. See git history if you
> need the old bot code.

## Features

- **Web dashboard** — pairing analytics, opponent breakdowns, head-to-head
  records, and match history, behind session-based auth with optional TOTP 2FA.
- **Playtomic fetch** — pulls bookings, filters for club matches, discovers new
  members, and records pairing matches for analytics. Runs on a manual button
  and hourly via an in-app scheduler.
- **Manual match entry** — record matches played outside Playtomic, with
  player/venue autocomplete and alias linking to unify statistics.
- **User administration** — admin UI for managing login users; `tribble-admin`
  CLI to bootstrap the first admin.

## Technology stack

- **Language:** Go
- **Web UI:** Templ templates, htmx, Pico CSS (assets embedded in the binary)
- **Database:** Turso (libsql) in production; local SQLite for development
- **Migrations:** Goose (run at startup)
- **Scheduling:** in-app cron ([robfig/cron](https://github.com/robfig/cron))
- **Local dev:** Air (hot reload)
- **Deployment:** [Kamal 2](https://kamal-deploy.org) → Docker (distroless) on a
  Hetzner VPS; TLS via kamal-proxy (Let's Encrypt HTTP-01)
- **CI/CD:** GitHub Actions
- **Testing:** Go standard library + Testify

## Local development

### Prerequisites

1. [Go](https://golang.org/doc/install)
2. [Air](https://github.com/air-verse/air) (optional, for hot reload)

### Setup

```bash
cp .env.example .env   # then fill in values (see comments in the file)
```

For a fully local run, leave `TURSO_PRIMARY_URL` empty — the app uses a local
SQLite file named by `DB_NAME` and applies migrations automatically.

### Run

```bash
air            # hot-reloading dev server
# or
make run       # build and run once
```

The server listens on `PORT` (default `8080`).

## Deployment

Infrastructure (the VPS, firewall, Docker, DNS) is provisioned separately by the
**utiger-infra** repo. This repo owns only the application, deployed with Kamal.

- **Host:** `box.utiger.dk` (shared) — see `config/deploy.yml`
- **Public URL:** `https://wally.utiger.dk`, served by kamal-proxy, which
  auto-issues a Let's Encrypt cert via HTTP-01. The `wally` DNS record must stay
  **unproxied** (gray cloud) in Cloudflare for the challenge to reach the origin.
- **Trigger:** push to `main` runs `.github/workflows/deploy.yml`, which tests,
  builds the image, pushes to GHCR, and runs `kamal`.

### Required GitHub Actions secrets

| Secret | Purpose |
| :-- | :-- |
| `KAMAL_SSH_PRIVATE_KEY` | SSH key Kamal uses to reach the box (its public key is in the box's `authorized_keys`) |
| `KAMAL_SSH_KNOWN_HOSTS` | Pinned host keys for the box (`ssh-keyscan box.utiger.dk`) |
| `TURSO_PRIMARY_URL` | Turso database URL |
| `TURSO_AUTH_TOKEN` | Turso auth token |
| `TENANT_ID` | Playtomic tenant ID |
| `WEB_SESSION_SECRET` | Session cookie signing key (32+ chars) |
| `WEB_TOTP_ENCRYPTION_KEY` | TOTP encryption key (exactly 32 chars) |

`KAMAL_REGISTRY_PASSWORD` is supplied automatically from the workflow's
`GITHUB_TOKEN` (GHCR).

### First deploy

1. Run the **Deploy** workflow manually with the `setup` command (or push to
   `main` — the workflow defaults to `setup`, which is idempotent).
2. Bootstrap the first admin user. `tribble-admin` talks to the remote Turso
   DB, so run it locally (no need to be on the box):
   ```bash
   make admin   # builds ./tribble-admin
   TURSO_PRIMARY_URL=... TURSO_AUTH_TOKEN=... \
     ./tribble-admin create-user --email admin@example.com --password '<pw>' --admin
   ```
3. Log in at `https://wally.utiger.dk` and optionally enable TOTP 2FA.

## Health

`GET /health` returns `200 OK` when the database is reachable and `503`
otherwise. kamal-proxy uses it to gate zero-downtime deploys and restart
unhealthy containers.

## Testing

```bash
go test ./...
```

Tests also run in CI on every push to `main`.

## License

MIT — see [LICENSE](LICENSE).
