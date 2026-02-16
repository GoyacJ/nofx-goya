# NOFX Makefile for testing and development

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

NOFX_BIN := nofx
WEB_DIR := web
NPM_WEB := npm --prefix $(WEB_DIR)
NOFX_PORT ?= 8080

.PHONY: help \
	test test-backend test-frontend test-coverage \
	build build-backend-only build-frontend ensure-web-dist \
	run run-fast run-binary run-frontend check \
	fmt lint clean \
	docker-build docker-up docker-down docker-logs \
	deps deps-update deps-frontend

# Default target
help:
	@echo "NOFX Testing & Development Commands"
	@echo ""
	@echo "Testing:"
	@echo "  make test                 - Run all tests (backend + frontend)"
	@echo "  make test-backend         - Run backend tests only"
	@echo "  make test-frontend        - Run frontend tests only"
	@echo "  make test-coverage        - Generate backend coverage report"
	@echo ""
	@echo "Build:"
	@echo "  make build                - Build standalone binary (embed frontend)"
	@echo "  make build-frontend       - Build frontend"
	@echo "  make ensure-web-dist      - Ensure web/dist exists for go:embed"
	@echo "  make build-backend-only   - Build backend binary only (without prebuild script)"
	@echo ""
	@echo "Run:"
	@echo "  make run                  - Prebuild frontend and run single-process NOFX"
	@echo "  make run-fast             - Run NOFX directly (requires existing web/dist)"
	@echo "  make run-binary           - Run existing ./nofx binary"
	@echo "  make check                - Check /health and /api/health"
	@echo ""
	@echo "Clean:"
	@echo "  make clean                - Clean build artifacts and test cache"

# =============================================================================
# Testing
# =============================================================================

# Run all tests
test: ensure-web-dist
	@echo "🧪 Running backend tests..."
	go test -v ./...
	@echo ""
	@echo "🧪 Running frontend tests..."
	$(NPM_WEB) run test
	@echo "✅ All tests completed"

# Backend tests only
test-backend: ensure-web-dist
	@echo "🧪 Running backend tests..."
	go test -v ./...

# Frontend tests only
test-frontend:
	@echo "🧪 Running frontend tests..."
	$(NPM_WEB) ci
	$(NPM_WEB) run test

# Coverage report
test-coverage: ensure-web-dist
	@echo "📊 Generating coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Backend coverage: coverage.html"

# =============================================================================
# Build
# =============================================================================

# Ensure web/dist exists before go:embed compilation
ensure-web-dist:
	@if [ ! -f "$(WEB_DIR)/dist/index.html" ]; then \
		echo "📦 web/dist not found, building frontend..."; \
		$(NPM_WEB) ci; \
		$(NPM_WEB) run build; \
	else \
		echo "✅ web/dist exists"; \
	fi

# Build standalone binary (frontend build + go build with embed)
build:
	@echo "🔨 Building standalone NOFX..."
	./scripts/build-standalone.sh
	@echo "✅ Standalone build completed: ./$(NOFX_BIN)"

# Build backend binary only (for internal use)
build-backend-only: ensure-web-dist
	@echo "🔨 Building backend only..."
	go build -o $(NOFX_BIN) .
	@echo "✅ Backend built: ./$(NOFX_BIN)"

# Build frontend
build-frontend:
	@echo "🔨 Building frontend..."
	$(NPM_WEB) ci
	$(NPM_WEB) run build
	@echo "✅ Frontend built: ./$(WEB_DIR)/dist"

# =============================================================================
# Development
# =============================================================================

# Run standalone NOFX (frontend prebuild + single process)
run:
	@echo "🚀 Starting standalone NOFX..."
	./scripts/run-standalone.sh

# Run standalone NOFX quickly (assumes web/dist already exists)
run-fast: ensure-web-dist
	@echo "🚀 Starting standalone NOFX (fast mode)..."
	go run .

# Run existing binary
run-binary:
	@if [ ! -x "./$(NOFX_BIN)" ]; then \
		echo "❌ ./$(NOFX_BIN) not found. Run 'make build' first."; \
		exit 1; \
	fi
	@echo "🚀 Running ./$(NOFX_BIN)..."
	./$(NOFX_BIN)

# Run frontend in development mode
run-frontend:
	@echo "🚀 Starting frontend dev server..."
	$(NPM_WEB) run dev

# Check health endpoints on current port
check:
	@echo "🔍 Checking NOFX health endpoints on port $(NOFX_PORT)..."
	@curl -fsS "http://127.0.0.1:$(NOFX_PORT)/health" && echo ""
	@curl -fsS "http://127.0.0.1:$(NOFX_PORT)/api/health" && echo ""
	@echo "✅ Health checks passed"

# Format Go code
fmt:
	@echo "🎨 Formatting Go code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Lint Go code (requires golangci-lint)
lint:
	@echo "🔍 Linting Go code..."
	golangci-lint run
	@echo "✅ Linting completed"

# =============================================================================
# Clean
# =============================================================================

clean:
	@echo "🧹 Cleaning..."
	rm -f $(NOFX_BIN)
	rm -f coverage.out coverage.html
	rm -rf $(WEB_DIR)/dist
	go clean -testcache
	@echo "✅ Cleaned"

# =============================================================================
# Docker
# =============================================================================

# Build Docker images
docker-build:
	@echo "🐳 Building Docker images..."
	docker compose build
	@echo "✅ Docker images built"

# Run Docker containers
docker-up:
	@echo "🐳 Starting Docker containers..."
	docker compose up -d
	@echo "✅ Docker containers started"

# Stop Docker containers
docker-down:
	@echo "🐳 Stopping Docker containers..."
	docker compose down
	@echo "✅ Docker containers stopped"

# View Docker logs
docker-logs:
	docker compose logs -f

# =============================================================================
# Dependencies
# =============================================================================

# Download Go dependencies
deps:
	@echo "📦 Downloading Go dependencies..."
	go mod download
	@echo "✅ Dependencies downloaded"

# Update Go dependencies
deps-update:
	@echo "📦 Updating Go dependencies..."
	go get -u ./...
	go mod tidy
	@echo "✅ Dependencies updated"

# Install frontend dependencies
deps-frontend:
	@echo "📦 Installing frontend dependencies..."
	$(NPM_WEB) ci
	@echo "✅ Frontend dependencies installed"
