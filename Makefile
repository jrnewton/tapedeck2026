.PHONY: help build run stop logs test lint-go lint-js lint clean deploy prod-stop

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build     Build tapedeck and tapedeck-cli binaries"
	@echo "  run       Run server via Docker Compose (port 8080)"
	@echo "  stop      Stop server"
	@echo "  logs      View server logs"
	@echo "  test      Run unit tests"
	@echo "  lint-go   Run go vet and govulncheck"
	@echo "  lint-js   Run JavaScript linter (ESLint)"
	@echo "  lint      Run all linters"
	@echo "  clean     Remove build artifacts"
	@echo "  deploy    Deploy to DigitalOcean droplet"
	@echo "  prod-stop Stop production server on DigitalOcean"

# Build the server binary (local dev and production)
build: lint test
	@mkdir -p bin
	go build -o bin/tapedeck ./cmd/tapedeck
	go build -o bin/tapedeck-cli ./cmd/tapedeck-cli

	@echo "Cross-compiling for Linux (static binary)..."
	@mkdir -p bin/prod
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/prod/tapedeck ./cmd/tapedeck
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/prod/tapedeck-cli ./cmd/tapedeck-cli

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

# Run go vet and vulnerability check
lint-go:
	go mod tidy
	go vet ./...
	go tool govulncheck ./...

# Run JavaScript linter
lint-js:
	npx eslint --max-warnings 0 cmd/tapedeck/web/*.js

# Run all linters
lint: lint-go lint-js

# Clean build artifacts
clean:
	rm -rf bin/
	go clean -cache

# DigitalOcean access
SSH_KEY := ~/.ssh/digitalocean_ed25519
DROPLET := root@68.183.125.135
REMOTE_PATH := /opt/tapedeck

deploy: clean build
	@echo "Syncing to DigitalOcean..."
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		bin/prod/ $(DROPLET):$(REMOTE_PATH)/bin/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		cmd/tapedeck/web/ $(DROPLET):$(REMOTE_PATH)/cmd/tapedeck/web/
	rsync -avz --checksum -e "ssh -i $(SSH_KEY)" \
		Dockerfile.deploy docker-compose.deploy.yml Caddyfile .env \
		$(DROPLET):$(REMOTE_PATH)/
	@echo "Rebuilding and restarting on server..."
	ssh -i $(SSH_KEY) $(DROPLET) "cd $(REMOTE_PATH) && docker compose -f docker-compose.deploy.yml up -d --build"
	@echo "Deploy complete! https://tapedeck.us"

# Stop production server on DigitalOcean
prod-stop:
	ssh -i $(SSH_KEY) $(DROPLET) "cd $(REMOTE_PATH) && docker compose -f docker-compose.deploy.yml down"
	@echo "Production server stopped."

# Sync local data to production (one-way, destructive)
#prod-sync:
#	@echo "WARNING: This will overwrite production data with local data!"
#	@echo "Press Ctrl+C to cancel, or wait 5 seconds to continue..."
#	@sleep 5
#	rsync -avz --delete --checksum -e "ssh -i $(SSH_KEY)" \
#		./data/ $(DROPLET):$(REMOTE_PATH)/data/
#	@echo "Data sync complete."
