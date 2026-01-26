.PHONY: build run stop test test-e2e clean logs

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
