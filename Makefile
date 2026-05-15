.PHONY: build test lint run infra-up infra-down infra-reset db-migrate health test-integration benchmark

build:
	go build -o ./bin/zm ./cmd/zm

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
