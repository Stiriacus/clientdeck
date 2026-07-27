.PHONY: run build test vet golden docker-build docker-up docker-down

run:
	go run ./cmd/vitrine

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o vitrine ./cmd/vitrine

test:
	go test -race ./...

vet:
	go vet ./...

golden:
	go test ./themes/... -update

# ── Docker ────────────────────────────────────────────────────

docker-build:
	docker build -t vitrine:latest -f docker/Dockerfile .

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down
