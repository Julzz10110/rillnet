.PHONY: test test-smoke smoke-stack build

test:
	go test ./pkg/... ./internal/... -count=1
	go test ./tests/integration/... ./tests/unit/config/... -count=1

test-smoke:
	go test ./tests/integration/... -run TestSmoke -count=1 -v

smoke-stack:
	@if [ -f scripts/smoke-stack.sh ]; then bash scripts/smoke-stack.sh; else powershell -File scripts/smoke-stack.ps1; fi

build:
	go build -o bin/ingest-server ./cmd/ingest
	go build -o bin/signal-server ./cmd/signal
