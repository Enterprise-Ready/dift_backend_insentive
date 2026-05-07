.PHONY: all build run test test-race test-coverage lint clean docker-build docker-up docker-down migrate

# ── Variables ─────────────────────────────────────────────────
APP_NAME    := payment-gateway
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME  := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BINARY      := bin/$(APP_NAME)
DOCKER_IMG  := $(APP_NAME):$(VERSION)
MAIN_PKG    := ./cmd/server
GO_FLAGS    := -ldflags="-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# ── Build ─────────────────────────────────────────────────────
all: lint test build

build:
	@echo "📦 Building $(APP_NAME) $(VERSION)..."
	@mkdir -p bin
	CGO_ENABLED=0 go build $(GO_FLAGS) -trimpath -o $(BINARY) $(MAIN_PKG)
	@echo "✅ Built: $(BINARY)"

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GO_FLAGS) -trimpath -o $(BINARY)-linux $(MAIN_PKG)

run:
	@go run $(MAIN_PKG)

# ── Testing ───────────────────────────────────────────────────
test:
	@echo "🧪 Running tests..."
	go test ./... -count=1 -timeout=60s

test-race:
	@echo "🏁 Running tests with race detector..."
	go test -race ./... -count=1 -timeout=120s

test-coverage:
	@echo "📊 Running tests with coverage..."
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-integration:
	@echo "🔗 Running integration tests..."
	go test ./... -tags=integration -count=1 -timeout=120s

bench:
	go test -bench=. -benchmem -run=^$$ ./...

# ── Code Quality ──────────────────────────────────────────────
lint:
	@echo "🔍 Linting..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./... --timeout=5m

fmt:
	gofmt -s -w .
	goimports -w .

vet:
	go vet ./...

# ── Database ──────────────────────────────────────────────────
migrate-up:
	@echo "⬆️  Running migrations..."
	migrate -path ./migrations -database "postgres://pguser:secret123@localhost:5432/payment_gateway?sslmode=disable" up

migrate-down:
	@echo "⬇️  Rolling back migrations..."
	migrate -path ./migrations -database "postgres://pguser:secret123@localhost:5432/payment_gateway?sslmode=disable" down 1

migrate-version:
	migrate -path ./migrations -database "postgres://pguser:secret123@localhost:5432/payment_gateway?sslmode=disable" version

# ── Docker ────────────────────────────────────────────────────
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -f deployments/docker/Dockerfile -t $(DOCKER_IMG) .

docker-up:
	@echo "🚀 Starting services..."
	docker compose -f deployments/docker/docker-compose.yml up -d
	@echo "✅ Services started. API: http://localhost:8080"

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker/docker-compose.yml logs -f api

docker-ps:
	docker compose -f deployments/docker/docker-compose.yml ps

# ── Kubernetes ───────────────────────────────────────────────
k8s-deploy:
	kubectl apply -f deployments/k8s/

k8s-delete:
	kubectl delete -f deployments/k8s/

k8s-status:
	kubectl get all -n payment-gateway

k8s-logs:
	kubectl logs -f -l app=payment-gateway -n payment-gateway --all-containers

# ── Utilities ─────────────────────────────────────────────────
generate-key:
	@echo "Generating 32-byte encryption key:"
	@openssl rand -base64 32

generate-api-key:
	@echo "Generating API key:"
	@openssl rand -hex 32

load-test:
	@which k6 > /dev/null || (echo "Install k6: https://k6.io/docs/get-started/installation/" && exit 1)
	k6 run scripts/load_test.js

clean:
	rm -rf bin/ coverage.out coverage.html

# ── Help ──────────────────────────────────────────────────────
help:
	@echo "Enterprise Payment Gateway - Available targets:"
	@echo ""
	@echo "  build          - Build the binary"
	@echo "  run            - Run the server"
	@echo "  test           - Run unit tests"
	@echo "  test-race      - Run tests with race detector"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  bench          - Run benchmarks"
	@echo "  lint           - Run linter"
	@echo "  docker-up      - Start all services with Docker Compose"
	@echo "  docker-down    - Stop all services"
	@echo "  migrate-up     - Run database migrations"
	@echo "  generate-key   - Generate a random 32-byte encryption key"
	@echo "  load-test      - Run k6 load tests"
	@echo "  help           - Show this help"
