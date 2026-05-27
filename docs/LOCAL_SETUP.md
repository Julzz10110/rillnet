# Local setup

## Prerequisites

- Docker and Docker Compose
- Go 1.26+ (optional, for running binaries outside Docker)
- Two browsers (or one normal + one incognito) for P2P smoke tests

## Quick start (Docker Compose)

```bash
git clone <repository-url>
cd rillnet

docker compose up -d --build
docker compose ps
```

Expected services: `redis`, `rillnet-web`, `rillnet-ingest`, `rillnet-signal`, `prometheus`, `grafana`.

### Health checks

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/ready
curl -s http://localhost:8081/ready
```

`/ready` should return `"dependencies":"ok"` when Redis is connected.

### UI

Open http://localhost

1. Register or login (note: auth is stub — any password works until real user storage ships).
2. Start publishing (allow camera).
3. In a second browser, login, refresh streams, join.

### WebRTC on Docker Desktop (Windows)

Browser ↔ ingest needs **UDP**. Docker Desktop often leaves ICE stuck in `checking` with `tracks=0` even when the publisher preview works locally.

**Recommended for Windows:** run **ingest on the host**, UI + signal + Redis in Docker:

```powershell
# Stop full stack if it was running (avoids port conflicts)
docker compose down

.\scripts\dev-host-ingest.ps1
```

Then open http://localhost, Ctrl+F5, publish in tab 1, join in tab 2.

Checklist:

| Check | Expected |
|-------|----------|
| Publisher DevTools console | `[RillNet Publisher] ice=connected` |
| Ingest terminal | `publisher started streaming track` |
| Viewer log | `registered=yes`, `ice=connected`, `tracks>=1` |

Full-container mode (`docker compose up`) can work on Linux; on Windows prefer host ingest or set `RILLNET_WEBRTC_NAT_1TO1_IP` to your LAN IP and ensure UDP `50000-50200` is forwarded.

## Configuration profiles

| File | Use |
|------|-----|
| `configs/config.yaml` | Default; Redis disabled |
| `configs/config.dev.yaml` | Redis on; used by Compose via volume mount |
| `configs/config.dev.host.yaml` | Host `go run ./cmd/ingest` with Compose Redis on `localhost:6379` |
| `configs/config.staging.yaml` | Staging template |
| `configs/config.prod.yaml` | Production template |

Copy `.env.example` to `.env` for local `go run` with Redis:

```bash
cp .env.example .env
# Start Redis: docker run -d -p 6379:6379 redis:7-alpine
export RILLNET_CONFIG_PATH=configs/config.dev.yaml
export RILLNET_REDIS_ADDRESS=localhost:6379
go run ./cmd/ingest
go run ./cmd/signal
```

## Nginx

- Development: `web/nginx.conf` (includes dev CORS helpers on proxy).
- Production template: `web/nginx.prod.conf` (CORS only on backend; TLS block commented).

## Troubleshooting

See [TROUBLESHOOTING.md](../TROUBLESHOOTING.md).

## CI vs CD

- **CI** (`.github/workflows/ci.yml`) runs on push/PR — tests, lint, build.
- **CD** (`.github/workflows/cd.yml`) is **disabled** for now (manual trigger only). Image push requires registry secrets; use `docker compose` locally.

## Smoke checklist

**Automated (S1–S7):** with Redis running:

```bash
export RILLNET_REDIS_ADDRESS=localhost:6379   # or use CI
make test-smoke
# or: go test ./tests/integration/... -run TestSmoke -count=1 -v
```

**Stack (S0 + health):**

```bash
./scripts/smoke-stack.sh
# Windows: .\scripts\smoke-stack.ps1
```

**UI (S8):** two browsers — register, publish, join (see steps above).

Record manual S0/S8 in [SMOKE_REPORT.md](SMOKE_REPORT.md). Known issues: [KNOWN_ISSUES.md](KNOWN_ISSUES.md).
