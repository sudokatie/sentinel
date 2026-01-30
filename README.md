# Sentinel

Self-hosted uptime monitoring with beautiful dashboards.

Know when things break before your users do. Know when they come back up before the panic sets in.

## Why Sentinel?

Because SaaS monitoring costs $30/month to watch 5 URLs. Because you don't need Kubernetes to know if your blog is down. Because sometimes the simple tool is the right tool.

Sentinel is a single binary that checks your endpoints, tracks response times, sends alerts when things break, and shows you pretty graphs. That's it. No agents to install, no complex configuration, no vendor lock-in.

## Features

- HTTP endpoint monitoring with configurable intervals
- Response time tracking and uptime statistics  
- Email alerts on downtime and recovery (with cooldown so you don't get spammed)
- Terminal-aesthetic dashboard (because I have a type)
- SQLite storage (zero configuration, just works)
- Single binary deployment (download, run, done)
- REST API for automation (because clicking buttons is for amateurs)
- Hourly data aggregation (keeps 90 days of history without filling your disk)

## Quick Start

```bash
# Clone and build
git clone https://github.com/sudokatie/sentinel.git
cd sentinel
make build

# Run
./bin/sentinel
```

Visit http://localhost:3000. You now have uptime monitoring. It took 30 seconds.

## CLI

```bash
# Start the server
sentinel serve

# Add a check via CLI (because GUIs are optional)
sentinel check add https://api.example.com/health -n "My API" -i 30

# List all checks
sentinel check list

# Test a URL without saving (for the paranoid)
sentinel check test https://example.com

# Show version
sentinel version
```

## Configuration

Create `sentinel.yaml` in the current directory. Or don't - the defaults are sensible.

```yaml
server:
  host: "0.0.0.0"
  port: 3000

database:
  path: "./sentinel.db"

alerts:
  consecutive_failures: 2      # Alert after 2 failures (not just one hiccup)
  recovery_notification: true  # Tell me when it's back, too
  cooldown_minutes: 5          # Don't spam me
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
  results_days: 7              # Raw data kept for 7 days
  aggregates_days: 90          # Hourly summaries kept for 90 days

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

Because putting passwords in config files is embarrassing:

- `SENTINEL_PORT` - Server port
- `SENTINEL_DB_PATH` - Database file path
- `SENTINEL_SMTP_HOST` - SMTP server hostname
- `SENTINEL_SMTP_PORT` - SMTP server port
- `SENTINEL_SMTP_USER` - SMTP username
- `SENTINEL_SMTP_PASSWORD` - SMTP password (use this, not the config file)
- `SENTINEL_SMTP_FROM` - From address for alerts
- `SENTINEL_SMTP_TO` - Comma-separated recipient addresses
- `SENTINEL_EMAIL_ENABLED` - Enable email alerts (true/false)

## API

For when you want to automate everything:

```bash
# List all checks
curl http://localhost:3000/api/checks

# Create a check
curl -X POST http://localhost:3000/api/checks \
  -H "Content-Type: application/json" \
  -d '{"name":"My Service","url":"https://example.com"}'

# Get check with stats
curl http://localhost:3000/api/checks/1

# Trigger a check manually (impatience is a virtue)
curl -X POST http://localhost:3000/api/checks/1/trigger

# Get recent results
curl http://localhost:3000/api/checks/1/results?limit=50

# Get statistics
curl http://localhost:3000/api/checks/1/stats

# List incidents (the hall of shame)
curl http://localhost:3000/api/incidents?limit=20

# Health check (quis custodiet ipsos custodes?)
curl http://localhost:3000/api/health
```

## Docker

```bash
# Build
docker build -t sentinel .

# Run
docker run -d -p 3000:3000 -v sentinel-data:/data sentinel
```

For the container enthusiasts. I don't judge. (I judge a little.)

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

4,400 lines of Go. Not a single framework. Just standard library and a bit of SQLite. The way code should be.

## License

MIT
