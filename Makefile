SHELL := /bin/bash

RELEASE_VER=$(BUILD_VERSION)
GIT_SHA    := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
GIT_PARENT := $(shell git show-branch -a 2>/dev/null | grep '\*' | grep -v "$(GIT_BRANCH)" | head -n1 | sed 's/.*\[\(.*\)\].*/\1/' | sed 's/[\^~].*//')
GIT_TAG := $(shell git describe --exact-match --tags $(git log -n1 --pretty='%h'))
ifeq ($(BUILD_VERSION),)
	RELEASE_VER=$(GIT_PARENT)
ifneq ($(GIT_TAG),)
	RELEASE_VER=$(GIT_TAG)
ifeq ($(GIT_PARENT),)
	RELEASE_VER=99.99.99
endif
endif
endif

VERSION = $(RELEASE_VER)-$(GIT_BRANCH)-$(GIT_SHA)

export REPO ?= platform9
export TAG ?= $(VERSION)

export UI_IMG ?= ${REPO}/vjailbreak-ui:${TAG}
export V2V_IMG ?= ${REPO}/v2v-helper:${TAG}
export CONTROLLER_IMG ?= ${REPO}/vjailbreak-controller:${TAG}
export RELEASE_VERSION ?= $(VERSION)

.PHONY: ui
ui:
	docker build --platform linux/amd64 -t $(UI_IMG) ui/
	docker push $(UI_IMG)

.PHONY: v2v-helper
v2v-helper:
	docker build --platform linux/amd64 --build-arg RELEASE_VERSION=$(VERSION) -t $(V2V_IMG) v2v-helper/
	docker push $(V2V_IMG)

.PHONY: test-v2v-helper
test-v2v-helper:
	cd v2v-helper && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./... -v

.PHONY: vjail-controller
vjail-controller: v2v-helper
	make -C k8s/migration/ docker-build docker-push

.PHONY: vjail-controller-only
vjail-controller-only:
	make -C k8s/migration/ docker-build docker-push

.PHONY: generate-manifests
generate-manifests: vjail-controller ui
	rm -rf image_builder/deploy && mkdir image_builder/deploy
	envsubst < ui/deploy/ui.yaml > image_builder/deploy/01ui.yaml
	make -C k8s/migration/ build-installer && cp k8s/migration/dist/install.yaml image_builder/deploy/00controller.yaml

.PHONY: docker-build-image
docker-build-image: generate-manifests
	rm -rf artifacts/ && mkdir artifacts/
	cp -r k8s/kube-prometheus image_builder/deploy/
	docker build --platform linux/amd64 --output=artifacts/ -t vjailbreak-image:local image_builder/ 

.PHONY: lint-check
lint-check:
	golangci-lint run --path-prefix=k8s/migration

.PHONY: build-image
build-image: generate-manifests
	rm -rf artifacts/ && mkdir artifacts/
	docker build --platform linux/amd64 --output=artifacts/ -t vjailbreak-image:local image_builder/ 
