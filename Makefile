# buktio — developer Makefile
# Single Go module at repo root (github.com/buktio/buktio); web is a separate Node project.

GO            ?= go
PNPM          ?= pnpm
GARAGE_IMAGE  ?= dxflrs/garage:v2.3.0
BIN_DIR       ?= bin
COMPOSE       ?= docker compose -f deploy/docker-compose/docker-compose.yml
DATABASE_URL  ?= postgres://buktio:buktio@localhost:5432/buktio?sslmode=disable

.DEFAULT_GOAL := help

## help: list available targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

## build: build the api and cli binaries
.PHONY: build
build: build-api build-cli build-s3proxy

## build-api: build the OSS REST API server
.PHONY: build-api
build-api:
	$(GO) build -o $(BIN_DIR)/buktio-api ./apps/api/cmd/server

## build-cli: build the product CLI
.PHONY: build-cli
build-cli:
	$(GO) build -o $(BIN_DIR)/buktio ./cmd/buktio

## build-s3proxy: build the S3 traffic-metering proxy
.PHONY: build-s3proxy
build-s3proxy:
	$(GO) build -o $(BIN_DIR)/buktio-s3proxy ./cmd/buktio-s3proxy

## tidy: sync go.mod/go.sum
.PHONY: tidy
tidy:
	$(GO) mod tidy

## test: run unit tests (mocked Garage)
.PHONY: test
test:
	$(GO) test ./... -race -short

## test-integration: run integration tests against a real pinned Garage (testcontainers)
.PHONY: test-integration
test-integration:
	GARAGE_IMAGE=$(GARAGE_IMAGE) $(GO) test ./... -race -tags=integration -run Integration -timeout 15m

## lint: run go vet + gofmt check
.PHONY: lint
lint:
	$(GO) vet ./...
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

## migrate: apply database migrations (requires golang-migrate)
.PHONY: migrate
migrate:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" up

## migrate-down: roll back the last migration
.PHONY: migrate-down
migrate-down:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" down 1

## web-install: install web dependencies
.PHONY: web-install
web-install:
	cd apps/web && $(PNPM) install

## web-dev: run the Next.js dev server
.PHONY: web-dev
web-dev:
	cd apps/web && $(PNPM) dev

## gen-env: generate deploy/docker-compose/.env with random secrets (once)
.PHONY: gen-env
gen-env:
	bash deploy/docker-compose/gen-env.sh

## up: build & start the FULL stack (caddy, web, api, postgres, garage)
.PHONY: up
up: gen-env
	$(COMPOSE) up -d --build

## down: stop the full stack (keep volumes)
.PHONY: down
down:
	$(COMPOSE) down

## ps: show stack status
.PHONY: ps
ps:
	$(COMPOSE) ps

# --- Local dev: infra (Postgres + Garage) in Docker, API/web run locally ---
COMPOSE_DEV ?= docker compose -f deploy/docker-compose/docker-compose.dev.yml

## dev-up: start ONLY the dev infra (Postgres :5432, Garage :3900/:3903)
.PHONY: dev-up
dev-up: gen-env
	$(COMPOSE_DEV) up -d

## dev-down: stop the dev infra (keep volumes)
.PHONY: dev-down
dev-down:
	$(COMPOSE_DEV) down

## dev-logs: tail dev infra logs
.PHONY: dev-logs
dev-logs:
	$(COMPOSE_DEV) logs -f

## api-local: run the Go API locally against the dev infra
.PHONY: api-local
api-local:
	bash scripts/dev-api.sh
