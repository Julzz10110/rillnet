.PHONY: build run test clean deploy

build:
	@echo "Building RillNet..."
	go build -o bin/ingest ./cmd/ingest
	go build -o bin/signal ./cmd/signal
	go build -o bin/monitor ./cmd/monitor

run: build
	@echo "Starting RillNet services..."
	./bin/signal &
	./bin/ingest &
	./bin/monitor &

test:
	@echo "Running tests..."
	go test ./tests/unit/... -v
	go test ./tests/integration/... -v

clean:
	@echo "Cleaning up..."
	rm -rf bin/
	go clean

deploy:
	@echo "Deploying to production..."
	docker-compose -f deployments/docker-compose.yml up -d

proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		pkg/protocol/*.proto

load-test:
	@echo "Running load tests..."
	go run tests/load/stress_test.go

# Testing
.PHONY: test test-all test-unit test-integration test-load coverage

test:
	@echo "Running tests..."
	@$(MAKE) -C tests unit

test-all:
	@echo "Running all tests..."
	@$(MAKE) -C tests all

test-unit:
	@$(MAKE) -C tests unit

test-integration:
	@$(MAKE) -C tests integration

test-load:
	@$(MAKE) -C tests load

test-coverage:
	@$(MAKE) -C tests coverage

test-benchmark:
	@$(MAKE) -C tests benchmark

test-clean:
	@$(MAKE) -C tests clean

test-help:
	@$(MAKE) -C tests help