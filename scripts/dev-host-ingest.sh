#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Starting redis, web, signal (host-stack, no ingest container)..."
docker compose -f docker-compose.host-stack.yml up -d --build

echo "Waiting for Redis on 127.0.0.1:6379..."
for _ in $(seq 1 30); do
  if (echo >/dev/tcp/127.0.0.1/6379) 2>/dev/null; then
    break
  fi
  sleep 1
done

export RILLNET_CONFIG_PATH=configs/config.dev.host.yaml
export RILLNET_REDIS_ADDRESS=127.0.0.1:6379
export RILLNET_SERVER_ADDRESS=:8080
export RILLNET_JWT_SECRET=dev-docker-compose-secret-change-in-production
export RILLNET_REDIS_ENABLED=true

echo ""
echo "Open http://localhost — ingest on :8080"
echo ""

exec go run ./cmd/ingest
