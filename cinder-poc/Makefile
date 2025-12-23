.PHONY: help run stop restart logs clean test-connect test-list test-create test-map test-unmap

SHELL := /bin/bash

help: ## Show this help message
	@echo "Cinder POC - Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

run: ## Start the demo server in background
	@echo "Starting Cinder POC server..."
	@bash -c 'source openstack.rc && go run cmd/demo/main.go > server.log 2>&1 &'
	@sleep 2
	@if lsof -i :8080 > /dev/null 2>&1; then \
		echo "✓ Server started on port 8080"; \
		tail -5 server.log; \
	else \
		echo "✗ Server failed to start. Check server.log"; \
		cat server.log; \
	fi

stop: ## Stop the demo server
	@echo "Stopping server..."
	@-lsof -ti :8080 | xargs kill -9 2>/dev/null
	@echo "✓ Server stopped"

restart: stop run ## Restart the server

logs: ## Show server logs
	@tail -f server.log

logs-all: ## Show all server logs
	@cat server.log

clean: ## Clean up logs and build artifacts
	@rm -f server.log nohup.out
	@echo "✓ Cleaned up"

# Test endpoints
test-connect: ## Test connection to OpenStack
	@echo "Testing connection..."
	@curl -s -X POST http://localhost:8080/connect && echo

test-list: ## List all volumes
	@echo "Listing volumes..."
	@curl -s http://localhost:8080/list-volumes | jq -r '.[] | "\(.name) (\(.id)) - \(.sizeBytes) bytes"'

test-whoami: ## Check provider type
	@curl -s http://localhost:8080/whoami && echo

# Volume operations
test-create: ## Create test volume (VOLUME_NAME=vj-test-vol SIZE_GB=1)
	@echo "Creating volume: $(or $(VOLUME_NAME),vj-test-vol)"
	@curl -s -X POST http://localhost:8080/create-volume \
		-H "Content-Type: application/json" \
		-d '{"name":"$(or $(VOLUME_NAME),vj-test-vol)","sizeBytes":$(or $(SIZE_BYTES),1073741824),"volumeType":""}' \
		| jq

test-get: ## Get volume info (VOLUME_NAME=vj-test-vol)
	@echo "Getting volume: $(or $(VOLUME_NAME),vj-test-vol)"
	@curl -s "http://localhost:8080/get-volume?name=$(or $(VOLUME_NAME),vj-test-vol)" | jq

test-map: ## Map volume to host (VOLUME_NAME=vj-test-vol HOST=esx-01 IQN=iqn.1998-01.com.vmware:...)
	@echo "Mapping volume $(or $(VOLUME_NAME),vj-test-vol) to $(or $(HOST),esx-01)"
	@curl -s -X POST http://localhost:8080/map-volume \
		-H "Content-Type: application/json" \
		-d '{"initiatorGroupName":"$(or $(HOST),esx-01)","volumeName":"$(or $(VOLUME_NAME),vj-test-vol)","iqns":["$(IQN)"]}' \
		| jq

test-unmap: ## Unmap volume from host (VOLUME_NAME=vj-test-vol HOST=esx-01 IQN=iqn.1998-01.com.vmware:...)
	@echo "Unmapping volume $(or $(VOLUME_NAME),vj-test-vol) from $(or $(HOST),esx-01)"
	@curl -s -X POST http://localhost:8080/unmap-volume \
		-H "Content-Type: application/json" \
		-d '{"initiatorGroupName":"$(or $(HOST),esx-01)","volumeName":"$(or $(VOLUME_NAME),vj-test-vol)","iqns":["$(IQN)"]}' \
		| jq

# Full workflow example
test-full-workflow: ## Run full test workflow (requires IQN env var)
	@echo "=== Full Workflow Test ==="
	@echo "1. Testing connection..."
	@$(MAKE) test-connect
	@echo ""
	@echo "2. Creating volume..."
	@$(MAKE) test-create VOLUME_NAME=vj-poc-test
	@sleep 2
	@echo ""
	@echo "3. Mapping volume (set IQN env var)..."
	@if [ -z "$(IQN)" ]; then \
		echo "⚠️  Set IQN environment variable first"; \
		echo "   Example: make test-full-workflow IQN=iqn.1998-01.com.vmware:esxi-hostname"; \
	else \
		$(MAKE) test-map VOLUME_NAME=vj-poc-test HOST=vj-poc-host IQN=$(IQN); \
	fi
	@echo ""
	@echo "=== Done - check results above ==="

# Development helpers
build: ## Build the binary
	@go build -o bin/cinder-poc cmd/demo/main.go
	@echo "✓ Built bin/cinder-poc"

deps: ## Download dependencies
	@go mod download
	@echo "✓ Dependencies downloaded"

tidy: ## Tidy go.mod
	@go mod tidy
	@echo "✓ go.mod tidied"

fmt: ## Format code
	@go fmt ./...
	@echo "✓ Code formatted"
