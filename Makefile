.PHONY: help build run stop test clean logs deploy prod-stop prod-sync lint-js lint-all gosec security

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build     Build tapedeck and tapedeck-cli binaries"
	@echo "  run       Run server via Docker Compose (port 8080)"
	@echo "  stop      Stop server"
	@echo "  logs      View server logs"
	@echo "  test      Run unit tests"
	@echo "  lint-js   Run JavaScript linter (ESLint)"
	@echo "  lint-sec  Run Go security scanner"
	@echo "  lint-all  Run all linters"
	@echo "  clean     Remove build artifacts"
	@echo "  deploy    Deploy to DigitalOcean droplet"
	@echo "  prod-stop Stop production server on DigitalOcean"
	@echo "  prod-sync Sync local ./data/ to production (DESTRUCTIVE)"

# Build the server binary (local dev)
build:
	@mkdir -p bin
	go build -o bin/tapedeck ./cmd/tapedeck
	go build -o bin/tapedeck-cli ./cmd/tapedeck-cli

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

# Run JavaScript linter
lint-js:
	npx eslint cmd/tapedeck/web/*.js

# Run all linters
lint-all: lint-js lint-sec

# Run Go security scanner
lint-sec:
	gosec -quiet ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean -cache

# Deploy to DigitalOcean droplet
SSH_KEY := ~/.ssh/digitalocean_ed25519
DROPLET := root@68.183.125.135
REMOTE_PATH := /opt/tapedeck

deploy:
	@echo "Cross-compiling for Linux (static binary)..."
	@mkdir -p bin/prod
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/prod/tapedeck ./cmd/tapedeck
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/prod/tapedeck-cli ./cmd/tapedeck-cli
	@echo "Syncing to DigitalOcean..."
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		bin/prod/ $(DROPLET):$(REMOTE_PATH)/bin/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		cmd/tapedeck/web/ $(DROPLET):$(REMOTE_PATH)/cmd/tapedeck/web/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		Dockerfile.deploy docker-compose.deploy.yml \
		$(DROPLET):$(REMOTE_PATH)/
	@echo "Rebuilding and restarting on server..."
	ssh -i $(SSH_KEY) $(DROPLET) "cd $(REMOTE_PATH) && docker compose -f docker-compose.deploy.yml build && docker compose -f docker-compose.deploy.yml up -d"
	@echo "Deploy complete! https://tapedeck.us"

# Stop production server on DigitalOcean
prod-stop:
	ssh -i $(SSH_KEY) $(DROPLET) "cd $(REMOTE_PATH) && docker compose -f docker-compose.deploy.yml down"
	@echo "Production server stopped."

# Sync local data to production (one-way, destructive)
prod-sync:
	@echo "WARNING: This will overwrite production data with local data!"
	@echo "Press Ctrl+C to cancel, or wait 5 seconds to continue..."
	@sleep 5
	rsync -avz --delete --checksum -e "ssh -i $(SSH_KEY)" \
		./data/ $(DROPLET):$(REMOTE_PATH)/data/
	@echo "Data sync complete."
