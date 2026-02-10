.PHONY: build run test lint clean docker-up docker-down migrate seed simulate

# Build the server binary.
build:
	go build -o bin/server ./cmd/server

# Run the server locally.
run:
	go run ./cmd/server

# Run all tests.
test:
	go test -v -race -coverprofile=coverage.out ./...

# Show test coverage report.
coverage: test
	go tool cover -html=coverage.out -o coverage.html

# Run linter.
lint:
	golangci-lint run ./...

# Clean build artifacts.
clean:
	rm -rf bin/ coverage.out coverage.html

# Start all services with Docker Compose.
docker-up:
	docker compose up -d --build

# Stop all services.
docker-down:
	docker compose down

# Stop and remove volumes.
docker-clean:
	docker compose down -v

# Run database migrations (requires running database).
migrate:
	psql $(DATABASE_URL) -f migrations/001_initial.sql

# Seed test data.
seed:
	go run ./scripts/seed.go

# Run the payment simulator.
simulate:
	go run ./scripts/simulator.go

# Format code.
fmt:
	go fmt ./...
	goimports -w .
