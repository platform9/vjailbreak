SHELL := /usr/bin/env bash
BIN := vassessment
BIN_DIR := $(shell pwd)/bin

RELEASE_VER=$(BUILD_VERSION)
GIT_SHA    := $(shell git rev-parse --short HEAD)
ifeq ($(BUILD_VERSION),)
	RELEASE_VER=dev
endif
VERSION = $(RELEASE_VER)-$(GIT_SHA)
LDFLAGS += "-s -w -X github.com/platform9/vjailbreak/vassessment/version.Version=$(VERSION)"
BUILD_OPTIONS += -ldflags=$(LDFLAGS) -buildvcs=false

.PHONY: build
build:
	@echo "Building $(BIN)..."
	go build $(BUILD_OPTIONS) -o $(BIN_DIR)/$(BIN) ./cmd/vassessment

.PHONY: test
test:
	@echo "Running tests for vassessment"
	go test ./... -v

.PHONY: run
run:
	go run ./cmd/vassessment

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
