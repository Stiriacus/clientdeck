# vitrine

A self-hosted Go HTTP service that renders a product recommendation board per
customer. Content is populated exclusively via JSON over the API (n8n or
`curl`): there is no admin UI, no manual maintenance, no CMS.

A customer gets a URL like `https://boards.example.com/c/acme-corp-a8f9b2`: a
product overview tabbable by category with a comparison table, star ratings,
and affiliate links, in the embedded default theme "plain".

Details on the contract (request/response schema, error codes) are in
[`docs/api.md`](docs/api.md), the n8n integration in
[`docs/n8n-setup.md`](docs/n8n-setup.md), the theme system in
[`docs/theming.md`](docs/theming.md).

## 5-Minute Quickstart

```sh
git clone https://github.com/Stiriacus/vitrine.git
cd vitrine
go build -o vitrine ./cmd/vitrine

export VITRINE_WEBHOOK_SECRET="$(openssl rand -hex 32)"
export VITRINE_BASE_URL="http://localhost:8080"
./vitrine
```

In a second terminal, create a board:

```sh
curl -X POST http://localhost:8080/api/v1/views \
  -H "X-Webhook-Secret: $VITRINE_WEBHOOK_SECRET" \
  -H "Content-Type: application/json" \
  --data @testdata/example_payload.json
```

The response contains a `url`. Open it in a browser to view the board.

A second push with the same `customer_id` **replaces** the content at the
**same URL**; it does not append or add products. The entire board is
overwritten with the new payload each time. Restarting the process doesn't
change that (the SQLite file is preserved).

## Configuration

All variables use the `VITRINE_` prefix:

| Var | Default | Required | Meaning |
|---|---|---|---|
| `VITRINE_ADDR` | `:8080` | – | Listen address |
| `VITRINE_DB_PATH` | `./vitrine.db` | – | Path to the SQLite file |
| `VITRINE_WEBHOOK_SECRET` | – | **yes** | Shared secret for `X-Webhook-Secret`, min. 32 characters |
| `VITRINE_BASE_URL` | – | **yes** | Base URL for the `url` in the webhook response, e.g. `https://boards.example.com` |
| `VITRINE_THEME` | `plain` | – | Theme name (only `plain` is embedded; other names require `VITRINE_DEV=true`) |
| `VITRINE_LOG_LEVEL` | `info` | – | `debug`, `info`, `warn`, or `error` |
| `VITRINE_DEV` | `false` | – | Dev mode, see below |
| `VITRINE_DEMO_PAYLOAD` | – | – | Path to a JSON file, only together with `VITRINE_DEV=true` |

`vitrine` aborts with a meaningful error message and exit code 1 if a
required field is missing or invalid.

### Dev mode (theme development)

```sh
VITRINE_DEV=true VITRINE_DEMO_PAYLOAD=testdata/example_payload.json make run
```

With `VITRINE_DEV=true`, templates and `static/` assets are re-read from
disk (`themes/<name>/`) on every request instead of from the embedded
binary. Change CSS/HTML, reload in the browser, done. With
`VITRINE_DEMO_PAYLOAD` also set, the board is available at `/c/demo`
without a database, `curl`, or secret. The file is re-read and validated on
every request. Both switches are intended exclusively for local theme
development and should never be set in production.

## Operations

`vitrine` is a single static binary (`CGO_ENABLED=0`, no Node/npm/build
step anywhere in the project) and needs no external dependencies besides its
SQLite file. It doesn't terminate TLS itself. A reverse proxy in front of it
handles that.

Two deployment paths are supported: **Docker** (simpler, especially for trying
things out) and **bare-metal** (systemd + nginx).

### Docker

All Docker files live in the `docker/` directory.

```sh
# Copy and edit the environment file
cp .env.example .env
# Set VITRINE_WEBHOOK_SECRET and VITRINE_BASE_URL in .env

# Build and start
make docker-up
```

This builds the image from scratch (multi-stage, final image is `scratch`-based
at ~10 MiB), mounts `/data` for the SQLite database, and exposes port 8080.
Stop with `make docker-down`.

The compose file reads `../.env` for environment variables. Keep that file
outside the `docker/` directory so it's not accidentally committed.

### systemd unit (example)

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

```sh
# /opt/vitrine/vitrine.env
VITRINE_ADDR=127.0.0.1:8080
VITRINE_DB_PATH=/opt/vitrine/vitrine.db
VITRINE_WEBHOOK_SECRET=<32+ characters, via openssl rand -hex 32>
VITRINE_BASE_URL=https://boards.example.com
```

`vitrine` shuts down gracefully on `SIGTERM`/`SIGINT` (open requests are
still served before the process exits). Works with `systemctl
stop`/`restart` without extra flags.

### Reverse proxy snippet (nginx, TLS terminated here)

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

## Non-Goals

- No admin UI, no CMS. Content is populated exclusively via the JSON API.
- No user accounts; the slug itself is the capability for read access.
- No Node/npm/build step. The theme is hand-written CSS based on
  `docs/design/tokens.css`.
- Deliberately deferred past v1: readback/delete endpoints, board TTL, PDF
  export, click tracking, secret rotation, i18n.

## Development

```sh
make build   # static binary
make test    # go test -race ./...
make vet     # go vet ./...
make golden  # rewrite golden files after an intentional theme change
make run     # go run ./cmd/vitrine
```

## License

[GNU AGPL v3](LICENSE).
