tapedeck.us built by Claude Code.

## Local Development

```bash
make build    # Build binaries
make run      # Run server via Docker Compose (port 8080)
make stop     # Stop server
make logs     # View server logs
make test     # Run unit tests
make test-e2e # Run E2E tests (requires Docker with Chromium)
make clean    # Remove build artifacts
```

## Production Deployment (DigitalOcean)

### Server Details
- **Droplet**: tapedeck (s-1vcpu-1gb, $6/mo)
- **IP**: 68.183.125.135
- **Region**: nyc1
- **Data path**: /opt/tapedeck/data

### SSH Access
```bash
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135
```

### Deploy Updates
```bash
# From local machine - sync code changes
rsync -avz -e "ssh -i ~/.ssh/digitalocean_ed25519" \
  Dockerfile docker-compose.yml go.mod go.sum cmd internal pkg web \
  root@68.183.125.135:/opt/tapedeck/

# On server - rebuild and restart
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "cd /opt/tapedeck && docker compose up -d --build"
```

### Sync Data (local -> production)
```bash
rsync -avz --progress -e "ssh -i ~/.ssh/digitalocean_ed25519" \
  data/ root@68.183.125.135:/opt/tapedeck/data/
```

### View Logs
```bash
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "docker logs -f tapedeck"
```

### Restart Container
```bash
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "cd /opt/tapedeck && docker compose restart"
```

### Check Status
```bash
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "docker ps && df -h /opt/tapedeck"
```

### DigitalOcean CLI (doctl)
```bash
doctl compute droplet list              # List droplets
doctl compute droplet get tapedeck      # Get droplet details
doctl account get                       # Check account status
```
