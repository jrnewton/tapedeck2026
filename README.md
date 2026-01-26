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
- **URL**: https://tapedeck.us
- **Droplet**: tapedeck (s-1vcpu-1gb, $6/mo)
- **IP**: 68.183.125.135
- **Region**: nyc1
- **Data path**: /opt/tapedeck/data
- **Reverse proxy**: Caddy (auto HTTPS via Let's Encrypt)
- **Firewall**: UFW (22, 80, 443 only)

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

### Caddy (Reverse Proxy / HTTPS)
```bash
# View Caddy config
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "cat /etc/caddy/Caddyfile"

# Edit Caddy config
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "nano /etc/caddy/Caddyfile"

# Restart Caddy (after config changes)
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "systemctl restart caddy"

# View Caddy logs
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "journalctl -u caddy -f"

# Check certificate status
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "caddy list-certificates"
```

### Firewall (UFW)
```bash
# Check firewall status
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "ufw status"

# Allow a port (if needed)
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "ufw allow 8080/tcp"

# Deny a port
ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
  "ufw deny 8080/tcp"
```

### DigitalOcean CLI (doctl)
```bash
doctl compute droplet list              # List droplets
doctl compute droplet get tapedeck      # Get droplet details
doctl account get                       # Check account status
```
