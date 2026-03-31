.PHONY: build run dev clean test fmt lint help docker-build docker-run docker-logs podman-build podman-run podman-logs

BINARY_NAME := heimsense
BUILD_DIR   := ./bin
MAIN_PKG    := ./cmd/server/
IMAGE_NAME  := heimsense

# Load .env if it exists
ifneq (,$(wildcard ./.env))
	include .env
	export
endif

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'

## build: Compile the binary
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	@echo "🔨 Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)
	@echo "✅ Built → $(BUILD_DIR)/$(BINARY_NAME)"

## run: Build and run the server
run: build
	@echo "🚀 Starting server..."
	$(BUILD_DIR)/$(BINARY_NAME)

## dev: Run with live reload (go run)
dev:
	@echo "🔄 Running in dev mode..."
	go run $(MAIN_PKG)

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf $(BUILD_DIR) server
	@echo "✅ Clean"

## test: Run all tests
test:
	go test -v -race ./...

## fmt: Format all Go files
fmt:
	gofmt -s -w .

## lint: Run go vet
lint:
	go vet ./...

## setup: Configure Claude Code to use this adapter
setup:
	@bash scripts/setup-claude.sh

## revert: Revert Claude Code to previous settings
revert:
	@if [ -f "$$HOME/.claude/settings.json.bak" ]; then \
		cp "$$HOME/.claude/settings.json.bak" "$$HOME/.claude/settings.json"; \
		echo "✅ Reverted to previous settings"; \
	else \
		echo "❌ No backup found"; \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t $(IMAGE_NAME):latest .

## docker-run: Run with Docker Compose
docker-run:
	@echo "🐳 Starting with Docker Compose..."
	docker compose up -d
	@echo "✅ Container started on http://localhost:8080"

## docker-stop: Stop Docker Compose
docker-stop:
	docker compose down

## docker-logs: Show Docker container logs
docker-logs:
	docker compose logs -f

## podman-build: Build Podman image
podman-build:
	@echo "📦 Building Podman image..."
	podman build -t $(IMAGE_NAME):latest .

## podman-run: Run with Podman Compose
podman-run:
	@echo "📦 Starting with Podman Compose..."
	podman-compose up -d
	@echo "✅ Container started on http://localhost:8080"

## podman-stop: Stop Podman Compose
podman-stop:
	podman-compose down

## podman-logs: Show Podman container logs
podman-logs:
	podman-compose logs -f
