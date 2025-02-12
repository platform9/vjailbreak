SHELL :=  /usr/bin/env bash

LDFLAGS += "-s -w -X github.com/platform9/vjailbreak/v2v-helper/pkg/version.Version=$(RELEASE_VERSION)"
BUILD_OPTIONS += -ldflags=$(LDFLAGS) -buildvcs=false

all: build test
.PHONY: all build test

build:
	@echo "Building v2v-helper..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(BUILD_OPTIONS) -a -o manager *.go

test:
	@echo "Running tests for v2v-helper"
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./... -v