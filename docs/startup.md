# Startup Guide

vitrine can be started in several ways depending on the context: local
development, production bare-metal, Docker, or theme development. This page
lists every option.

## Quick reference

| Method | Command | Best for |
|---|---|---|
| Bare-metal (go run) | `make run` | Quick iteration, debugging |
| Static binary | `make build && ./vitrine` | Production bare-metal |
| Go test | `go test -race ./...` | CI / local test runs |
| Dev mode /c/demo | see [Dev mode](#dev-mode) below | Theme/HMTL development |
| Docker (build from source) | `make docker-up` | Trying out, local Docker dev |
| Docker (pre-built image) | see [Docker prod](#docker-production) below | Production Docker |
| systemd service | see [systemd](#systemd-service) below | Production bare-metal |
| Behind nginx | see [nginx](#reverse-proxy-nginx) below | TLS termination in production |

## Bare-metal

### go run (no binary)

```sh
make run
```

Equivalent to `go run ./cmd/vitrine`. Compiles and starts in one step. Good for
iterating on Go code since any compile error shows immediately.

### Static binary

```sh
make build
./vitrine
```

Produces `./vitrine` — a `CGO_ENABLED=0` static binary (~19 MiB). Needs no Go
toolchain at the target. Copy it to a server and run.

### Environment

All three startup methods above rely on environment variables (or a `.env`
file sourced in your shell):

```sh
export VITRINE_WEBHOOK_SECRET="$(openssl rand -hex 32)"
export VITRINE_BASE_URL="http://localhost:8080"
./vitrine
```

See [Configuration](../README.md#configuration) in the README for the full
list of variables.

## Dev mode

Theme development without a database, secret, or curl:

```sh
VITRINE_DEV=true VITRINE_DEMO_PAYLOAD=testdata/example_payload.json make run
```

- Templates and static assets are re-read from disk on every request (hot
  reload). Change CSS/HTML, refresh the browser.
- The board is available at `http://localhost:8080/c/demo`. No store, no
  authentication.
- The demo payload file is re-read on every request, so changing the JSON
  takes effect on refresh too.

The same works with any theme — set `VITRINE_THEME` and point the working
directory at the repo root so the `themes/<name>/` paths resolve:

```sh
VITRINE_THEME=frost VITRINE_DEV=true VITRINE_DEMO_PAYLOAD=testdata/example_payload.json make run
```

Equivalent manual command:

```sh
VITRINE_DEV=true VITRINE_DEMO_PAYLOAD=testdata/example_payload.json \
  VITRINE_WEBHOOK_SECRET="dev-secret-thirty-two-chars-minimum!!" \
  VITRINE_BASE_URL=http://localhost:8080 \
  go run ./cmd/vitrine
```

## Docker

### Development / build from source

```sh
# One-time setup
cp .env.example .env
# Edit .env: set VITRINE_WEBHOOK_SECRET and VITRINE_BASE_URL

make docker-up
```

Builds the Docker image from scratch (multi-stage, final image ~10 MiB) and
starts the container. The SQLite database lives in a named volume
(`vitrine-data`) mounted at `/data`. The `.env` file sits outside `docker/`
and is read by compose automatically.

Stop with `make docker-down`.

### Production / pre-built image

```sh
docker compose -f docker/docker-compose.prod.yml up -d
```

This compose file pulls `ghcr.io/stiriacus/vitrine:latest` instead of
building. No Go toolchain on the host.

**About the `latest` tag:** CI pushes `latest` on every push to `main`
(see `ci.yml` line 126). It's a rolling channel, not a stable point
release. If you need a fixed version, tag a commit with `v*` in git
and use the corresponding semver tag on GHCR
(e.g. `ghcr.io/stiriacus/vitrine:v1.2.3`).

To mount additional themes:

```yaml
volumes:
  - ./my-themes:/themes:ro
```

Set `VITRINE_THEMES_DIR=/themes` and `VITRINE_THEME=<name>` in `.env`.

### Manual Docker run

```sh
docker run -d --name vitrine \
  -p 8080:8080 \
  -v vitrine-data:/data \
  -e VITRINE_WEBHOOK_SECRET="$(openssl rand -hex 32)" \
  -e VITRINE_BASE_URL=https://boards.example.com \
  ghcr.io/stiriacus/vitrine:latest
```

## systemd service

Run the static binary as a daemon:

```ini
# /etc/systemd/system/vitrine.service
[Unit]
Description=vitrine
After=network.target

[Service]
User=vitrine
Group=vitrine
WorkingDirectory=/opt/vitrine
EnvironmentFile=/opt/vitrine/vitrine.env
ExecStart=/opt/vitrine/vitrine
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
```

With an env file at `/opt/vitrine/vitrine.env`:

```ini
VITRINE_ADDR=127.0.0.1:8080
VITRINE_DB_PATH=/opt/vitrine/vitrine.db
VITRINE_WEBHOOK_SECRET=<32+ characters>
VITRINE_BASE_URL=https://boards.example.com
VITRINE_THEME=plain
VITRINE_LOG_LEVEL=info
```

```sh
systemctl daemon-reload
systemctl enable --now vitrine
```

## Reverse proxy (nginx)

vitrine does not terminate TLS. Put nginx in front:

```nginx
server {
    listen 443 ssl http2;
    server_name boards.example.com;

    ssl_certificate     /etc/letsencrypt/live/boards.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/boards.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## CI / tests

```sh
go test -race ./...    # run all tests
make test              # same thing
make vet               # go vet ./...
make golden            # rewrite theme golden files
```

The CI pipeline (`.github/workflows/ci.yml`) runs `go vet`, `go test -race`,
`golangci-lint`, `govulncheck`, and a multi-platform Docker build.

## Loading external themes

Regardless of startup method, external themes (those beyond the embedded
`"plain"` theme) are loaded at startup from `VITRINE_THEMES_DIR`. Each
subdirectory must contain `layout.html`, `board.html`, `notfound.html`, and
`i18n/`. Once loaded, themes can be assigned per-customer via the `"theme"`
field in the webhook payload.

- **Dev mode:** set `VITRINE_DEV=true` and the configured theme loads with
  per-request template reload from `themes/<name>/` (or `$VITRINE_THEMES_DIR` if set).
- **Production:** set `VITRINE_THEMES_DIR=/themes` and themes are loaded once
  at startup. No hot-reload. The `VITRINE_THEME` variable controls the
  fallback theme used when a customer has no specific assignment.
