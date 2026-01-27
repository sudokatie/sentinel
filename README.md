# Sentinel

Self-hosted uptime monitoring with beautiful dashboards.

Know when things break before your users do.

## Features

- HTTP endpoint monitoring
- Response time tracking
- Email alerts on downtime
- Beautiful terminal-aesthetic dashboard
- SQLite storage (zero configuration)
- Single binary deployment

## Quick Start

```bash
# Download
git clone https://github.com/katieblackabee/sentinel.git
cd sentinel

# Build
make build

# Run
./bin/sentinel
```

Visit http://localhost:3000 to access the dashboard.

## Configuration

Create `sentinel.yaml` in the current directory:

```yaml
server:
  port: 3000

alerts:
  email:
    enabled: true
    smtp_host: smtp.gmail.com
    smtp_port: 587
    smtp_user: your@email.com
    smtp_password: your-app-password
    from_address: sentinel@yoursite.com
    to_addresses:
      - alerts@yoursite.com

checks:
  - name: My API
    url: https://api.example.com/health
    interval: 30s
```

All configuration can also be set via environment variables with `SENTINEL_` prefix.

## License

MIT
