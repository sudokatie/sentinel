# Sentinel

Self-hosted uptime monitoring with beautiful dashboards.

Know when things break before your users do.

## Features

- HTTP endpoint monitoring with configurable intervals
- Response time tracking and uptime statistics
- Email alerts on downtime and recovery
- Terminal-aesthetic dashboard
- SQLite storage (zero configuration)
- Single binary deployment
- REST API for automation

## Quick Start

```bash
# Clone and build
git clone https://github.com/katieblackabee/sentinel.git
cd sentinel
make build

# Run
./bin/sentinel
```

Visit http://localhost:3000 to access the dashboard.

## Configuration

Create `sentinel.yaml` in the current directory:

```yaml
server:
  host: "0.0.0.0"
  port: 3000

database:
  path: "./sentinel.db"

alerts:
  consecutive_failures: 2
  recovery_notification: true
  cooldown_minutes: 5
  email:
    enabled: true
    smtp_host: smtp.gmail.com
    smtp_port: 587
    smtp_user: your@email.com
    smtp_password: your-app-password
    smtp_tls: true
    from_address: sentinel@yoursite.com
    to_addresses:
      - alerts@yoursite.com

retention:
  results_days: 7
  aggregates_days: 90

checks:
  - name: My API
    url: https://api.example.com/health
    interval: 30s
    timeout: 10s
    expected_status: 200
    tags:
      - api
      - production
```

### Environment Variables

All configuration values can be overridden via environment variables with `SENTINEL_` prefix:

- `SENTINEL_PORT` - Server port
- `SENTINEL_DB_PATH` - Database file path
- `SENTINEL_SMTP_HOST` - SMTP server hostname
- `SENTINEL_SMTP_PORT` - SMTP server port
- `SENTINEL_SMTP_USER` - SMTP username
- `SENTINEL_SMTP_PASSWORD` - SMTP password (recommended over config file)
- `SENTINEL_SMTP_FROM` - From address for alerts
- `SENTINEL_SMTP_TO` - Comma-separated list of recipient addresses
- `SENTINEL_EMAIL_ENABLED` - Enable email alerts (true/false)

## API

### Checks

```bash
# List all checks
curl http://localhost:3000/api/checks

# Create a check
curl -X POST http://localhost:3000/api/checks \
  -H "Content-Type: application/json" \
  -d '{"name":"My Service","url":"https://example.com"}'

# Get a check
curl http://localhost:3000/api/checks/1

# Update a check
curl -X PUT http://localhost:3000/api/checks/1 \
  -H "Content-Type: application/json" \
  -d '{"interval_seconds":60}'

# Delete a check
curl -X DELETE http://localhost:3000/api/checks/1

# Trigger a check manually
curl -X POST http://localhost:3000/api/checks/1/trigger

# Get check results
curl http://localhost:3000/api/checks/1/results?limit=50

# Get check statistics
curl http://localhost:3000/api/checks/1/stats
```

### Incidents

```bash
# List incidents
curl http://localhost:3000/api/incidents?limit=20
```

### Health

```bash
curl http://localhost:3000/api/health
```

## Docker

```bash
# Build
docker build -t sentinel .

# Run
docker run -d -p 3000:3000 -v sentinel-data:/data sentinel
```

## Development

```bash
# Run tests
make test

# Build
make build

# Run locally
make run
```

## Architecture

```
sentinel/
├── cmd/sentinel/       # CLI entry point
├── internal/
│   ├── config/         # Configuration loading
│   ├── storage/        # SQLite storage layer
│   ├── checker/        # HTTP checks and scheduling
│   ├── alerter/        # Alert management and email
│   └── web/            # HTTP server and UI
└── static/             # CSS and JavaScript
```

## License

MIT
