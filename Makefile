.PHONY: help build run stop test test-e2e clean logs

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build     Build tapedeck and tapedeck-cli binaries"
	@echo "  run       Run server via Docker Compose (port 8080)"
	@echo "  stop      Stop server"
	@echo "  logs      View server logs"
	@echo "  test      Run unit tests"
	@echo "  test-e2e  Run E2E tests (requires Docker)"
	@echo "  clean     Remove build artifacts"

# Build the server binary
build:
	go build -o tapedeck ./cmd/tapedeck
	go build -o tapedeck-cli ./cmd/tapedeck-cli

# Run the server via Docker Compose
run:
	docker compose build && docker compose up -d

# Stop the server
stop:
	docker compose down

# View server logs
logs:
	docker compose logs -f

# Run unit tests
test:
	go test ./...

# Run E2E tests (requires Docker)
test-e2e:
	docker build -f Dockerfile.test -t tapedeck-test . && docker run --rm tapedeck-test

# Clean build artifacts
clean:
	rm -f tapedeck tapedeck-cli
	go clean -cache
