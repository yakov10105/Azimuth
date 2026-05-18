.PHONY: build test lint run infra-up infra-down infra-reset db-migrate health test-integration benchmark orchestrator-up orchestrator-down orchestrator-test

build:
	go build -ldflags "-X github.com/azimuth/azimuth/internal/cli.Version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o ./bin/zm ./cmd/zm

test:
	go test ./internal/... ./cmd/...

test-integration:
	go test -tags integration ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/zm

infra-up:
	docker compose up -d --wait
	@echo "All services healthy"

infra-down:
	docker compose down

infra-reset:
	docker compose down -v
	docker compose up -d --wait

db-migrate:
	migrate -path ./migrations -database "$$POSTGRES_DSN" up

health:
	@./scripts/health_check.sh

benchmark:
	go test -bench=. -benchmem ./...

orchestrator-up:
	docker compose up -d --wait orchestrator
	@echo "Orchestrator healthy at http://localhost:8000"

orchestrator-down:
	docker compose stop orchestrator

orchestrator-test:
	cd orchestrator && python -m pytest tests/ -v
