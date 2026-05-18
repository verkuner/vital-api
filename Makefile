BINARY=bin/api
CMD=cmd/api/main.go
MODULE=$(shell go list -m)

.PHONY: run dev build test test-cover test-integration test-race \
        lint fmt migrate-up migrate-down migrate-create migrate-version \
        sqlc seed swagger \
        docker-build docker-up docker-down \
        encore-run encore-test encore-build \
        deploy deploy-api deploy-infra \
        signoz-up signoz-down signoz-configs

# ── Encore (local/dev) ───────────────────────────────────────────────────────

encore-run:
	encore run

encore-test:
	encore test ./...

encore-build:
	encore build docker --config=deployments/docker/infra.config.json vital-api:latest

# ── Standalone (production) ──────────────────────────────────────────────────

run:
	go run $(CMD)

dev:
	air -c .air.toml

build:
	go build -ldflags="-s -w" -o $(BINARY) $(CMD)

# ── Test ─────────────────────────────────────────────────────────────────────

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-integration:
	go test -race -count=1 -tags=integration ./...

test-race:
	go test -race -count=1 ./...

# ── Lint & Format ─────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

fmt:
	goimports -w .
	@test -z "$$(goimports -l .)" || (echo "unformatted files remain" && exit 1)

# ── Database ──────────────────────────────────────────────────────────────────

MIGRATIONS_PATH=internal/database/migrations
DB_URL ?= $(shell grep DATABASE_URL .env | cut -d '=' -f2-)

migrate-up:
	migrate -database "$(DB_URL)" -path $(MIGRATIONS_PATH) up

migrate-down:
	migrate -database "$(DB_URL)" -path $(MIGRATIONS_PATH) down 1

migrate-version:
	migrate -database "$(DB_URL)" -path $(MIGRATIONS_PATH) version

migrate-create:
	@test -n "$(NAME)" || (echo "Usage: make migrate-create NAME=description" && exit 1)
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME)

# ── sqlc ─────────────────────────────────────────────────────────────────────

sqlc:
	sqlc generate -f db/sqlc.yaml

# ── Seed ─────────────────────────────────────────────────────────────────────

seed:
	go run scripts/seed.go

# ── Swagger ───────────────────────────────────────────────────────────────────

swagger:
	swag init -g $(CMD) -o docs/swagger

# ── Docker ───────────────────────────────────────────────────────────────────

docker-build:
	docker build -f deployments/docker/Dockerfile -t vital-api:local .

docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# ── Deploy (VM: ubuntu@101.42.46.218) ────────────────────────────────────────

deploy:
	@bash scripts/deploy.sh full

deploy-api:
	@bash scripts/deploy.sh api

deploy-infra:
	@bash scripts/deploy.sh infra

# Local self-hosted SigNoz stack (not used in production).
signoz-up:
	docker compose -f deployments/docker/docker-compose.signoz.yml up -d

signoz-down:
	docker compose -f deployments/docker/docker-compose.signoz.yml down

# Downloads ClickHouse config files required by docker-compose.signoz.yml (run once).
signoz-configs:
	@SIGNOZ_TAG=v0.122.0 DEPLOY_DIR=deployments/docker BASE=https://raw.githubusercontent.com/SigNoz/signoz/$${SIGNOZ_TAG}/deploy/docker/clickhouse-setup; \
	mkdir -p $${DEPLOY_DIR}/clickhouse/user_scripts; \
	echo "Downloading ClickHouse configs (SigNoz $${SIGNOZ_TAG})..."; \
	curl -fsSL "$${BASE}/clickhouse/config.xml"          -o $${DEPLOY_DIR}/clickhouse/config.xml; \
	curl -fsSL "$${BASE}/clickhouse/users.xml"           -o $${DEPLOY_DIR}/clickhouse/users.xml; \
	curl -fsSL "$${BASE}/clickhouse/custom-function.xml" -o $${DEPLOY_DIR}/clickhouse/custom-function.xml; \
	curl -fsSL "$${BASE}/clickhouse/cluster.xml"         -o $${DEPLOY_DIR}/clickhouse/cluster.xml; \
	curl -fsSL "$${BASE}/otel-collector-config.yaml"     -o $${DEPLOY_DIR}/signoz-otel-collector-config.yaml; \
	echo "Done. Files written to $${DEPLOY_DIR}/clickhouse/ and $${DEPLOY_DIR}/signoz-otel-collector-config.yaml"
