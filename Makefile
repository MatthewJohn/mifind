.PHONY: help build run run-dev test web web-build web-dev config-clean

help:
	@echo "mifind - Personal unified search"
	@echo ""
	@echo "Targets:"
	@echo "  make build         - Build all binaries"
	@echo "  make run           - Run mifind API (requires web-build first)"
	@echo "  make run-dev       - Run mifind API with dev web server (hot-reload)"
	@echo "  make test          - Run tests"
	@echo "  make web-build     - Build React web UI"
	@echo "  make web-dev       - Start React dev server (in foreground)"
	@echo "  make config-clean  - Remove local configs"
	@echo ""
	@echo "Individual binaries:"
	@echo "  make build-mifind"
	@echo "  make build-filesystem-api"
	@echo "  make build-mifind-mcp"
	@echo ""
	@echo "Running individual services:"
	@echo "  make run-mifind"
	@echo "  make run-filesystem-api"

build: build-mifind build-filesystem-api build-mifind-mcp

build-mifind:
	go build -o bin/mifind ./cmd/mifind

build-filesystem-api:
	go build -o bin/filesystem-api ./cmd/filesystem-api

build-mifind-mcp:
	go build -o bin/mifind-mcp ./cmd/mifind-mcp

run: run-mifind

run-mifind: web-build
	bin/mifind

run-dev: web-dev
	bin/mifind

run-filesystem-api:
	bin/filesystem-api

run-mifind-mcp:
	bin/mifind-mcp

test:
	go test ./... -short

web-build:
	cd web && npm install && npm run build && ./copy-to-api.sh

web-dev:
	cd web && npm run dev

config-clean:
	rm -f config/mifind.yaml config/filesystem-api.yaml
