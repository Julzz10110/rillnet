#!/usr/bin/env bash
# Docker Compose stack smoke (S0 + health). S1–S7 run in CI via go test.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Building and starting stack..."
docker compose up -d --build

echo "==> Waiting for services..."
sleep 15

echo "==> S0: compose ps"
docker compose ps

fail=0
check() {
  local name="$1" url="$2"
  if curl -sf "$url" >/dev/null; then
    echo "OK  $name $url"
  else
    echo "FAIL $name $url"
    fail=1
  fi
}

check "ingest health" "http://localhost:8080/health"
check "ingest ready"  "http://localhost:8080/ready"
check "signal ready"  "http://localhost:8081/ready"
check "web"           "http://localhost/"

if [[ "$fail" -ne 0 ]]; then
  echo "Smoke checks failed. Logs: docker compose logs"
  exit 1
fi

echo ""
echo "Stack smoke passed (S0 + health)."
echo "Next: open http://localhost for S8 (2 browsers), or run:"
echo "  RILLNET_REDIS_ADDRESS=localhost:6379 go test ./tests/integration/... -run TestSmoke -count=1"
