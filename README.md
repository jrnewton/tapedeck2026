tapedeck.us built by Claude Code.

## Local Development

```bash
make build    # Build binaries
make run      # Run server via Docker Compose (port 8080)
make stop     # Stop server
make logs     # View server logs
make test     # Run unit tests
make clean    # Remove build artifacts
```

## Android Emulator
Run this to enable http://localhost:8080 access in emulator
```
%USERPROFILE%\AppData\Local\Android\Sdk\platform-tools\adb.exe reverse tcp:8080 tcp:8080
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
  Dockerfile docker-compose.yml .env.example Caddyfile.oauth \
  go.mod go.sum cmd internal pkg web \
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

### OAuth2 Authentication (Admin API Protection)

Admin API endpoints (POST/DELETE/PATCH to /api/*) are protected via OAuth2 Proxy with Google authentication.

#### Initial Setup

1. **Create Google OAuth credentials**:
   - Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
   - Create OAuth 2.0 Client ID (Web application)
   - Add authorized redirect URI: `https://tapedeck.us/oauth2/callback`

2. **Create .env file on server**:
   ```bash
   ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135
   cd /opt/tapedeck
   cp .env.example .env
   nano .env  # Fill in your Google OAuth credentials
   ```

3. **Generate cookie secret**:
   ```bash
   openssl rand -base64 32
   ```

4. **Update Caddyfile** (use Caddyfile.oauth as reference):
   ```bash
   ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
     "cat /opt/tapedeck/Caddyfile.oauth > /etc/caddy/Caddyfile && systemctl restart caddy"
   ```

5. **Start oauth2-proxy**:
   ```bash
   ssh -i ~/.ssh/digitalocean_ed25519 root@68.183.125.135 \
     "cd /opt/tapedeck && docker compose up -d"
   ```

#### How It Works

- **Public access**: GET requests to /api/* work without authentication
- **Protected actions**: POST/DELETE/PATCH to /api/* require Google login
- **Allowed users**: Only emails listed in OAUTH2_PROXY_AUTHENTICATED_EMAILS can access
- **Session**: Cookie-based, persists across browser restarts

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
