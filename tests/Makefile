# Variables
TEST_TIMEOUT = 10m
RACE_DETECTOR = true
COVERAGE_FILE = ../coverage.out
COVERAGE_HTML = ../coverage.html
BENCH_TIME = 5s
STRESS_TEST_DURATION = 30s
STRESS_TEST_PEERS = 100
PERF_TEST_OPS = 1000

# Default targets
.PHONY: all unit integration load benchmark stress coverage clean help

all: unit integration

# Unit tests
unit:
	@echo "🚀 Running unit tests..."
	@cd .. && go test ./tests/unit/... \
		-v \
		-timeout=$(TEST_TIMEOUT) \
		-coverprofile=$(COVERAGE_FILE)

unit-race:
	@echo "🏁 Running unit tests with race detector..."
	@cd .. && CGO_ENABLED=1 go test ./tests/unit/... \
		-v \
		-race \
		-timeout=$(TEST_TIMEOUT) \
		-coverprofile=$(COVERAGE_FILE)

unit-verbose:
	@echo "🔍 Running unit tests with verbose output..."
	@cd .. && go test ./tests/unit/... -v -timeout=$(TEST_TIMEOUT)

unit-short:
	@echo "⚡ Running quick unit tests..."
	@cd .. && go test ./tests/unit/... -short -timeout=2m

# Integration tests
integration:
	@echo "🔗 Running integration tests..."
	@cd .. && go test ./tests/integration/... \
		-v \
		-timeout=15m \
		-tags=integration

integration-webrtc:
	@echo "📹 Running WebRTC integration tests..."
	@cd .. && go test ./tests/integration/webrtc_integration_test.go \
		-v \
		-timeout=10m \
		-tags=integration

integration-mesh:
	@echo "🕸️ Running mesh network integration tests..."
	@cd .. && go test ./tests/integration/mesh_integration_test.go \
		-v \
		-timeout=10m \
		-tags=integration

# Load tests
load:
	@echo "📊 Running load tests..."
	@cd .. && go run tests/load/stress_test.go

load-custom:
	@echo "🎯 Running custom load test with $(STRESS_TEST_PEERS) peers for $(STRESS_TEST_DURATION)..."
	@cd .. && go run tests/load/stress_test.go \
		-peers $(STRESS_TEST_PEERS) \
		-duration $(STRESS_TEST_DURATION)

# Stress tests
stress:
	@echo "🔥 Running stress tests..."
	@cd .. && go test ./tests/load/... \
		-v \
		-timeout=30m \
		-tags=stress

stress-short:
	@echo "🔥 Running short stress tests..."
	@cd .. && go test ./tests/load/stress_test.go \
		-v \
		-timeout=5m \
		-run TestShortStress

# Benchmark tests
benchmark:
	@echo "⏱️ Running benchmark tests..."
	@cd .. && go test ./tests/load/... \
		-bench=. \
		-benchmem \
		-benchtime=$(BENCH_TIME) \
		-tags=benchmark

benchmark-cpu:
	@echo "💻 Running CPU benchmark tests..."
	@cd .. && go test ./tests/load/... \
		-bench=. \
		-benchmem \
		-cpuprofile=../cpu.pprof \
		-tags=benchmark

benchmark-memory:
	@echo "🧠 Running memory benchmark tests..."
	@cd .. && go test ./tests/load/... \
		-bench=. \
		-benchmem \
		-memprofile=../mem.pprof \
		-tags=benchmark

# Code coverage
coverage:
	@echo "📈 Generating test coverage report..."
	@cd .. && go test ./internal/... ./tests/unit/... \
		-coverprofile=$(COVERAGE_FILE) \
		-covermode=atomic
	@cd .. && go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✅ Coverage report generated: $(COVERAGE_HTML)"

coverage-show:
	@if [ -f "$(COVERAGE_HTML)" ]; then \
		echo "📊 Opening coverage report..."; \
		open $(COVERAGE_HTML) 2>/dev/null || xdg-open $(COVERAGE_HTML) 2>/dev/null || echo "Open $(COVERAGE_HTML) manually"; \
	else \
		echo "❌ Coverage report not found. Run 'make coverage' first."; \
	fi

coverage-ci:
	@echo "📊 Generating CI coverage report..."
	@cd .. && go test ./internal/... ./tests/unit/... \
		-coverprofile=$(COVERAGE_FILE) \
		-covermode=atomic
	@cd .. && go tool cover -func=$(COVERAGE_FILE)

# Service-specific tests
test-stream-service:
	@echo "🎬 Testing Stream Service..."
	@cd .. && go test ./tests/unit/services/stream_service_test.go \
		-v \
		-timeout=2m

test-mesh-service:
	@echo "🕸️ Testing Mesh Service..."
	@cd .. && go test ./tests/unit/services/mesh_service_test.go \
		-v \
		-timeout=2m

test-webrtc:
	@echo "📹 Testing WebRTC components..."
	@cd .. && go test ./tests/unit/webrtc/... \
		-v \
		-timeout=5m

test-repositories:
	@echo "💾 Testing repositories..."
	@cd .. && go test ./tests/unit/repositories/... \
		-v \
		-timeout=2m

# Tests with different environments
test-race:
	@echo "🏁 Running tests with race detector..."
	@cd .. && go test ./tests/... \
		-race \
		-timeout=$(TEST_TIMEOUT)

test-verbose:
	@echo "🔍 Running tests with verbose output..."
	@cd .. && go test ./tests/... \
		-v \
		-timeout=$(TEST_TIMEOUT)

# Cleanup
clean:
	@echo "🧹 Cleaning test artifacts..."
	@rm -f ../coverage.out ../coverage.html ../cpu.pprof ../mem.pprof ../*.test
	@find .. -name "*.test" -delete

clean-profiles:
	@echo "🧹 Cleaning profile files..."
	@rm -f ../*.pprof

# Help
help:
	@echo "🎯 RillNet Test Management"
	@echo ""
	@echo "📋 Available targets:"
	@echo "  all              - Run all tests (unit + integration)"
	@echo "  unit             - Run unit tests"
	@echo "  integration      - Run integration tests"
	@echo "  load             - Run load tests"
	@echo "  stress           - Run stress tests"
	@echo "  benchmark        - Run benchmark tests"
	@echo "  coverage         - Generate coverage report"
	@echo ""
	@echo "🔧 Specific test targets:"
	@echo "  test-stream-service - Test stream service"
	@echo "  test-mesh-service   - Test mesh service"
	@echo "  test-webrtc         - Test WebRTC components"
	@echo "  test-repositories   - Test repositories"
	@echo ""
	@echo "⚙️  Configuration:"
	@echo "  STRESS_TEST_PEERS=200 make load-custom"
	@echo "  STRESS_TEST_DURATION=1m make load-custom"
	@echo "  PERF_TEST_OPS=5000 make benchmark"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  clean            - Remove test artifacts"
	@echo "  clean-profiles   - Remove profile files"

# Aliases
test: unit
check: unit
ci: unit integration coverage-ci
perf: benchmark
validate: unit integration

# Version information
version:
	@echo "RillNet Test Suite v1.0.0"
	@go version

# Install test dependencies
deps:
	@echo "📦 Installing test dependencies..."
	@cd .. && go get -u github.com/stretchr/testify/assert
	@cd .. && go get -u github.com/stretchr/testify/mock
	@cd .. && go get -u github.com/stretchr/testify/require
	@echo "✅ Dependencies installed"

# Check dependencies installation
check-deps:
	@echo "🔍 Checking test dependencies..."
	@cd .. && go list -m github.com/stretchr/testify > /dev/null 2>&1 && echo "✅ testify installed" || echo "❌ testify missing"
	@cd .. && go list -m github.com/pion/webrtc/v3 > /dev/null 2>&1 && echo "✅ webrtc installed" || echo "❌ webrtc missing"
	@cd .. && go list -m github.com/gin-gonic/gin > /dev/null 2>&1 && echo "✅ gin installed" || echo "❌ gin missing"

# Generate test data
test-data:
	@echo "📁 Generating test data..."
	@mkdir -p ../test-data/streams
	@mkdir -p ../test-data/peers
	@echo "✅ Test data directories created"

# Docker tests
test-docker:
	@echo "🐳 Running tests in Docker..."
	@docker build -t rillnet-tests -f ../deployments/docker/Dockerfile.test ..
	@docker run --rm rillnet-tests

# Parallel test execution
test-parallel:
	@echo "⚡ Running tests in parallel..."
	@cd .. && go test ./tests/unit/... -parallel=4 -timeout=$(TEST_TIMEOUT)

# Tests with different log levels
test-debug:
	@echo "🐛 Running tests with debug logging..."
	@cd .. && RILLNET_LOG_LEVEL=debug go test ./tests/... -v -timeout=$(TEST_TIMEOUT)

test-silent:
	@echo "🤫 Running tests silently..."
	@cd .. && go test ./tests/... -timeout=$(TEST_TIMEOUT) 2>/dev/null || echo "Tests completed with status: $$?"

# Export test environment variables
export-test-env:
	@echo "🔧 Exporting test environment variables..."
	@export RILLNET_TEST_MODE=true
	@export RILLNET_LOG_LEVEL=info
	@export RILLNET_WEBRTC_MOCK=true
	@echo "✅ Test environment variables exported"