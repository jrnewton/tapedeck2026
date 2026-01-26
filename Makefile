.PHONY: help build run stop test test-e2e clean logs deploy

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
	@echo "  deploy    Deploy to DigitalOcean droplet"

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

# Deploy to DigitalOcean droplet
SSH_KEY := ~/.ssh/digitalocean_ed25519
DROPLET := root@68.183.125.135
REMOTE_PATH := /opt/tapedeck

deploy:
	@echo "Syncing code to DigitalOcean..."
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		Dockerfile docker-compose.yml go.mod go.sum \
		$(DROPLET):$(REMOTE_PATH)/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		cmd/ $(DROPLET):$(REMOTE_PATH)/cmd/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		internal/ $(DROPLET):$(REMOTE_PATH)/internal/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		pkg/ $(DROPLET):$(REMOTE_PATH)/pkg/
	@echo "Rebuilding and restarting on server..."
	ssh -i $(SSH_KEY) $(DROPLET) "cd $(REMOTE_PATH) && docker compose build --no-cache && docker compose up -d"
	@echo "Deploy complete! https://tapedeck.us"
