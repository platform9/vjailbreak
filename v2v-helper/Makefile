SHELL :=  /usr/bin/env bash 

LDFLAGS += "-s -w -X github.com/platform9/vjailbreak/v2v-helper/pkg/version.Version=$(RELEASE_VERSION)"
BUILD_OPTIONS += -ldflags=$(LDFLAGS) -buildvcs=false
MOCK_TESTS += nbd openstack vm virtv2v vcenter

all: build test
.PHONY: all build test

build:
	@echo "Building v2v-helper..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(BUILD_OPTIONS) -a -o manager *.go

test:
	@echo "Running tests for v2v-helper"
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./... -v
#				mockgen -source=$$file -destination=$(basename $$file _mock.go) ; 

generate-tests:
	@for dir in $(MOCK_TESTS); do \
		echo "Generating mock in $$dir..."; \
		for file in $$dir/*.go; do \
			if [[ $$file != *"_test"* ]] && [[ $$file != *"_mock"* ]] && [ -f $$file ]; then \
				echo "Running mockgen on $$file..."; \
				mockgen -source=$$file -destination=$$dir/$$(basename "$$file" .go)_mock.go --package="$$dir"; \
			fi; \
		done; \
	done