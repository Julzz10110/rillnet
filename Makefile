# RillNet — common developer targets (aligned with .github/workflows/ci.yml)
#
# Extended test targets (load, benchmark, legacy unit suites): make -C tests help

GO ?= go
BIN_DIR ?= bin
REDIS_ADDR ?= localhost:6379

INGEST_BIN = $(BIN_DIR)/ingest-server
SIGNAL_BIN = $(BIN_DIR)/signal-server

.PHONY: help build run clean \
	test test-unit test-integration test-smoke test-coverage test-all \
	lint smoke-stack compose-up compose-down compose-logs compose-host-up \
	test-help

.DEFAULT_GOAL := help

help:
	@echo "RillNet Makefile"
	@echo ""
	@echo "Build & run:"
	@echo "  build          Build ingest and signal binaries into $(BIN_DIR)/"
	@echo "  run            Build and start signal + ingest (requires Redis; see docs/LOCAL_SETUP.md)"
	@echo "  clean          Remove $(BIN_DIR)/ and test artifacts"
	@echo ""
	@echo "Tests (CI-aligned):"
	@echo "  test           Unit (pkg + internal) + integration (needs Redis)"
	@echo "  test-unit      pkg/... and internal/... only"
	@echo "  test-integration  integration + tests/unit/config (needs Redis)"
	@echo "  test-smoke     Stack API smoke tests (TestSmoke_*, needs Redis)"
	@echo "  test-coverage  Unit tests with -race and coverage report"
	@echo "  test-all       Delegate to tests/Makefile (may include outdated suites)"
	@echo ""
	@echo "Quality & stack:"
	@echo "  lint           Run golangci-lint (must be installed locally)"
	@echo "  smoke-stack    Docker Compose up + health checks (scripts/smoke-stack.*)"
	@echo "  compose-up     docker compose up -d --build"
	@echo "  compose-down   docker compose down"
	@echo "  compose-logs   docker compose logs -f"
	@echo "  compose-host-up  Redis+web+signal in Docker (ingest: scripts/dev-host-ingest.*)"
	@echo ""
	@echo "More: make test-help  (targets under tests/)"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(INGEST_BIN) ./cmd/ingest
	$(GO) build -o $(SIGNAL_BIN) ./cmd/signal

run: build
	@echo "Starting signal and ingest (Ctrl+C to stop; use separate terminals for logs)..."
	$(SIGNAL_BIN) &
	$(INGEST_BIN) &

clean:
	rm -rf $(BIN_DIR)
	$(GO) clean
	@$(MAKE) -C tests clean 2>/dev/null || true

# --- Tests (match CI where possible) ---

test-unit:
	$(GO) test ./pkg/... ./internal/... -count=1

test-integration:
	RILLNET_REDIS_ENABLED=true RILLNET_REDIS_ADDRESS=$(REDIS_ADDR) \
		$(GO) test ./tests/integration/... ./tests/unit/config/... -count=1 -timeout 10m

test-smoke:
	RILLNET_REDIS_ENABLED=true RILLNET_REDIS_ADDRESS=$(REDIS_ADDR) \
		$(GO) test ./tests/integration/... -run TestSmoke -count=1 -v

test: test-unit test-integration

test-coverage:
	$(GO) test ./pkg/... ./internal/... -race -coverprofile=coverage-unit.out -count=1
	$(GO) tool cover -func=coverage-unit.out | tail -1

test-all:
	$(MAKE) -C tests all

test-help:
	$(MAKE) -C tests help

# --- Lint & Docker ---

lint:
	golangci-lint run

smoke-stack:
	@if [ -f scripts/smoke-stack.sh ]; then bash scripts/smoke-stack.sh; else powershell -File scripts/smoke-stack.ps1; fi

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f

compose-host-up:
	docker compose -f docker-compose.host-stack.yml up -d --build
