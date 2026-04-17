# Sentinel

Self-hosted uptime monitoring with beautiful dashboards.

Know when things break before your users do. Know when they come back up before the panic sets in.

## Why Sentinel?

Because SaaS monitoring costs $30/month to watch 5 URLs. Because you don't need Kubernetes to know if your blog is down. Because sometimes the simple tool is the right tool.

Sentinel is a single binary that checks your endpoints, tracks response times, sends alerts when things break, and shows you pretty graphs. That's it. No agents to install, no complex configuration, no vendor lock-in.

## Features

- HTTP endpoint monitoring with configurable intervals
- Response time tracking and uptime statistics  
- SSL certificate monitoring (expiry alerts, issuer info)
- Multi-channel alerts: Email, Slack, Discord (with cooldown so you don't get spammed)
- Public status pages (share uptime with your users)
- Terminal-aesthetic dashboard (because I have a type)
- SQLite storage (zero configuration, just works)
- Single binary deployment (download, run, done)
- REST API for automation (because clicking buttons is for amateurs)
- Hourly data aggregation (keeps 90 days of history without filling your disk)
- Synthetic monitoring (run Playwright scripts as health checks)
- Multi-probe locations (distributed agents, regional outage detection)
- Anomaly detection (latency spike alerts, trend analysis)
- Incident management with status tracking and timeline notes
- Maintenance windows (suppress alerts during planned downtime)

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
  ssl_expiry_days: 30          # Alert when SSL cert expires within 30 days
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
  slack:
    enabled: true
    webhook_url: https://hooks.slack.com/services/T00/B00/xxx
  discord:
    enabled: true
    webhook_url: https://discord.com/api/webhooks/123/abc

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
- `SENTINEL_SLACK_ENABLED` - Enable Slack alerts (true/false)
- `SENTINEL_SLACK_WEBHOOK` - Slack incoming webhook URL
- `SENTINEL_DISCORD_ENABLED` - Enable Discord alerts (true/false)
- `SENTINEL_DISCORD_WEBHOOK` - Discord webhook URL

## Public Status Pages

Share your service status with users without giving them admin access.

Status pages are keyed by tag. Any check tagged with `production` is visible at `/status/production`.

```yaml
checks:
  - name: API Server
    url: https://api.example.com/health
    tags:
      - production  # Visible at /status/production
      - api

  - name: Web App
    url: https://app.example.com
    tags:
      - production  # Also visible at /status/production
      - frontend
```

Then share `https://yoursite.com/status/production` with your users.

The status page shows:
- Overall status (operational or degraded)
- Aggregate uptime percentage
- Individual service status with response times
- 24-hour sparkline for each service

No login required. No branding (yet).

## Synthetic Monitoring

Run Playwright scripts to test actual user flows. HTTP checks tell you if the server responds. Synthetic checks tell you if the login button works.

```go
import "github.com/katieblackabee/sentinel/internal/checker"

// Create a synthetic checker
synth := checker.NewSyntheticChecker("/var/screenshots")

// Run a Playwright script
response := synth.Execute(&checker.SyntheticRequest{
    ScriptPath: "/scripts/login-flow.spec.ts",
    Timeout:    30 * time.Second,
    Name:       "Login Flow",
})

// Check results
if response.Success {
    fmt.Printf("Total time: %dms\n", response.TotalDurationMs)
    fmt.Println(response.StepSummary())
} else {
    fmt.Printf("Failed: %s\n", response.Error)
    fmt.Printf("Screenshot: %s\n", response.ScreenshotPath)
}
```

Features:
- Step-by-step timing breakdown
- Screenshots on failure (so you can see what went wrong)
- JSON output parsing from Playwright reporter
- Timeout handling (because hung scripts are worse than failed ones)

Requirements:
- Node.js and npx available in PATH
- Playwright installed (`npm install -D @playwright/test`)

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

# List active incidents only
curl http://localhost:3000/api/incidents/active

# Get incident details with timeline
curl http://localhost:3000/api/incidents/1

# Update incident status (investigating, identified, monitoring, resolved)
curl -X PUT http://localhost:3000/api/incidents/1/status \
  -H "Content-Type: application/json" \
  -d '{"status":"identified"}'

# Update incident title
curl -X PUT http://localhost:3000/api/incidents/1/title \
  -H "Content-Type: application/json" \
  -d '{"title":"Database connection issues"}'

# Add a note to an incident (build your timeline)
curl -X POST http://localhost:3000/api/incidents/1/notes \
  -H "Content-Type: application/json" \
  -d '{"content":"Root cause identified: connection pool exhausted","author":"Alice"}'

# Health check (quis custodiet ipsos custodes?)
curl http://localhost:3000/api/health
```

## Incident Management

Incidents are auto-created when a check fails. But raw downtime isn't the whole story.

**Status Tracking**: Each incident has a status that follows the standard incident response lifecycle:
- `investigating` - Initial state. Something's wrong, you're looking into it.
- `identified` - You know what's broken.
- `monitoring` - Fix is deployed, watching for recurrence.
- `resolved` - All clear. Time for the postmortem.

**Incident Notes**: Build a timeline of what happened and when. Add notes as you investigate:
- "Noticed elevated error rates"
- "Root cause: database failover triggered by network partition"
- "Deployed hotfix to connection retry logic"

Notes are timestamped and optionally attributed. Your future self will thank you during the postmortem.

**Titles**: Give incidents meaningful names. "API Outage" beats "Incident #47".

## Multi-Probe Locations

Check from multiple geographic locations. Catch regional outages that single-location monitoring misses.

### Probe Agent

Deploy the probe agent binary on servers in different regions:

```bash
# Build the probe agent
go build -o probe ./cmd/probe/

# Run a probe agent
./probe -server http://sentinel.example.com:3000 \
        -key probe-secret-key \
        -name "US-East-1" \
        -region us-east \
        -city "Virginia" \
        -country "US" \
        -lat 37.5 -lon -77.4
```

The probe agent:
- Registers with the Sentinel server on startup
- Sends heartbeats every 30 seconds
- Pulls assigned checks and executes them locally
- Reports results back to the coordinator
- Auto-deregisters on graceful shutdown

### Coordinator

The server-side coordinator:
- Assigns checks to probes based on `min_probes` configuration
- Aggregates results from multiple probes
- Detects regional outages (some probes fail, others succeed)
- Detects global outages (all probes fail)
- Compares latency across regions

### Outage Detection

- **Regional outage**: Check fails from >=50% of probes in a region but succeeds elsewhere
- **Global outage**: Check fails from all probes
- Alerts include affected regions in the notification payload

### API Endpoints

```bash
# Register a probe
curl -X POST http://localhost:3000/api/probes/register \
  -H "Content-Type: application/json" \
  -d '{"name":"US-East-1","region":"us-east","api_key":"probe-key"}'

# List active probes
curl http://localhost:3000/api/probes

# Submit a check result from a probe
curl -X POST http://localhost:3000/api/probes/1/results \
  -H "Content-Type: application/json" \
  -d '{"check_id":1,"status":"up","response_time_ms":45}'

# Get probe results for a check
curl http://localhost:3000/api/checks/1/probe-results
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
├── cmd/probe/          # Standalone probe agent binary
├── internal/
│   ├── config/         # Configuration loading
│   ├── storage/        # SQLite storage layer
│   ├── checker/        # HTTP checks, scheduling, synthetic
│   ├── alerter/        # Alert management and email
│   ├── anomaly/        # Latency anomaly detection
│   ├── probe/          # Multi-probe registry, coordinator, geo utilities
│   └── web/            # HTTP server and UI
└── static/             # CSS and JavaScript
```

4,400 lines of Go. Not a single framework. Just standard library and a bit of SQLite. The way code should be.

## License

MIT
