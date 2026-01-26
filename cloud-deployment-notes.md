# Deployment Notes for Tapedeck

## Overview

Cloudflare doesn't offer Docker container hosting (Workers/Pages are serverless/edge-based with no persistent filesystem). This document evaluates alternatives for hosting a Docker container with persistent storage.

### Audio File Sizes (from actual downloads)

| File                               | Size   |
|------------------------------------|--------|
| Aural_Fixation (2hr show)          | 114 MB |
| Backpacks_and_Magazines (1hr show) | 59 MB  |
| Backwoods (2hr show)               | 114 MB |
| Calling_the_Cranes (1hr show)      | 59 MB  |
| Sewersounds (1hr show)             | 59 MB  |
| James_Dean_Death_Car (2hr show)    | 114 MB |

**Average: ~90 MB per show** (used in all forecasts below)

---

## DigitalOcean Droplet (Recommended)

### Pricing Summary

#### Smallest Droplet Plans

| Plan          | vCPU | RAM    | SSD   | Bandwidth | Cost  |
|---------------|------|--------|-------|-----------|-------|
| s-1vcpu-512mb | 1    | 512 MB | 10 GB | 500 GB    | $4/mo |
| s-1vcpu-1gb   | 1    | 1 GB   | 25 GB | 1 TB      | $6/mo |

#### Additional Storage & Bandwidth
- **Block Storage (SSD)**: $0.10/GB/month
- **Block Storage (HDD)**: $0.02/GB/month
- **Snapshots**: $0.05/GB/month
- **Extra bandwidth**: $0.01/GB (after allowance)
- **Ingress**: Free

### Tapedeck Monthly Forecast (10 downloads/week, 3 listens/week)

| Item                                     | Calculation                | Cost   |
|------------------------------------------|----------------------------|--------|
| **Droplet** (s-1vcpu-1gb)                | 1GB RAM, 25GB SSD included | $6.00  |
| **Ingress** (10 shows/wk x 4 wks x 90MB) | 3.6 GB - free              | $0.00  |
| **Egress** (3 shows/wk x 4 wks x 90MB)   | 1.1 GB - included in 1TB   | $0.00  |
| **Storage** (first 25GB)                 | included in droplet        | $0.00  |
| **Total**                                |                            | **$6/mo** |

### Storage Growth Projection

The $6 droplet includes 25GB SSD. Once you exceed that, add HDD block storage:

| Timeframe  | Total Storage | Extra Block Storage (HDD) | Total Cost |
|------------|---------------|---------------------------|------------|
| 0-6 months | ~22 GB        | 0 GB (fits in droplet)    | $6/mo      |
| 6-9 months | ~35 GB        | 10 GB x $0.02             | $6.20/mo   |
| 12 months  | ~45 GB        | 20 GB x $0.02             | $6.40/mo   |
| 24 months  | ~90 GB        | 65 GB x $0.02             | $7.30/mo   |

### Sources
- https://www.digitalocean.com/pricing/droplets
- https://docs.digitalocean.com/platform/billing/bandwidth/

---

## Fly.io (Alternative)

### Why Fly.io?

Fly.io is a good fit for "Docker + persistent volume" hosting with minimal setup.

## Fly.io Pricing Summary

### Persistent Volumes
- **$0.15/GB/month** for provisioned capacity
- Billed hourly, even when the attached VM is stopped

### Volume Snapshots (new Jan 2026)
- **$0.08/GB/month** for snapshot storage
- **First 10GB free** per month
- Daily snapshots enabled by default (5-day retention)
- Charged on actual data written, not provisioned size
- Can be disabled if you don't need backups

### Bandwidth
- **Ingress: Free**
- **Egress (outbound): $0.02/GB** in North America & Europe
- Higher in other regions (up to $0.12/GB in Africa/India)

### Other costs
- **IPv4 address: $2/month** per app (IPv6 is free)
- Compute: smallest VM (~$2-3/month for shared CPU)

### Tapedeck Monthly Forecast (10 downloads/week, 3 listens/week)

| Item                                     | Calculation              | Cost   |
|------------------------------------------|--------------------------|--------|
| **Compute** (shared-cpu-1x, 256MB)       | flat                     | $2.00  |
| **IPv4 address**                         | flat                     | $2.00  |
| **Ingress** (10 shows/wk x 4 wks x 90MB) | 3.6 GB - free            | $0.00  |
| **Egress** (3 shows/wk x 4 wks x 90MB)   | 1.1 GB x $0.02           | $0.02  |
| **Storage** (grows ~3.6GB/mo, avg 20GB)  | 20 GB x $0.15            | $3.00  |
| **Snapshots** (10GB free, then $0.08/GB) | ~10 GB free              | $0.00  |
| **Total**                                |                          | **~$7/mo** |

### Storage Growth Projection

At 3.6GB/month growth from new downloads:

- 6 months: ~40GB storage = $6 storage -> **$10/mo total**
- 12 months: ~60GB storage = $9 storage -> **$13/mo total**

Options: periodically delete old shows to keep storage flat, or budget for gradual growth.

### Sources
- https://fly.io/docs/about/pricing/
- https://fly.io/pricing/
- https://fly.io/calculator

---

## Fly.io vs DigitalOcean Comparison

| Aspect             | Fly.io             | DigitalOcean           |
|--------------------|--------------------|------------------------|
| Base cost          | ~$4 (compute+IPv4) | $6 (droplet)           |
| Storage included   | 0 GB               | 25 GB                  |
| Storage cost       | $0.15/GB           | $0.02/GB (HDD)         |
| Bandwidth included | 0 GB               | 1 TB                   |
| Egress overage     | $0.02/GB           | $0.01/GB               |
| Month 1 total      | ~$7                | $6                     |
| Month 12 total     | ~$13               | ~$6.40                 |
| Setup complexity   | Simple (fly.toml)  | You manage the VM      |

**Bottom line**: DigitalOcean is cheaper long-term due to included storage and lower block storage rates.

---

## Other Platforms Considered

| Platform | Persistent Volumes | Pricing             | Notes                           |
|----------|--------------------|--------------------|----------------------------------|
| Railway  | Yes                | $5/mo + usage      | Simple, good for small projects  |
| Render   | Yes (Disks)        | $7/mo + $0.25/GB   | Easy setup, good docs            |

