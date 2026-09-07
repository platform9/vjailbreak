SHELL := /bin/bash
export PATH := /usr/local/go/bin:$(PATH)

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

VERSION = $(RELEASE_VER)-$(GIT_SHA)

export REGISTRY ?= quay.io
export REPO ?= platform9
export TAG ?= $(VERSION)
export UI_IMG ?= ${REGISTRY}/${REPO}/vjailbreak-ui:${TAG}
export V2V_IMG ?= ${REGISTRY}/${REPO}/v2v-helper:${TAG}
export CONTROLLER_IMG ?= ${REGISTRY}/${REPO}/vjailbreak-controller:${TAG}
export VPWNED_IMG ?= ${REGISTRY}/${REPO}/vjailbreak-vpwned:${TAG}
export AI_IMG ?= ${REGISTRY}/${REPO}/vjailbreak-ai:${TAG}
export RELEASE_VERSION ?= $(VERSION)
export KUBECONFIG ?= ~/.kube/config
export CONTAINER_TOOL ?= docker


.PHONY: setup-hooks
setup-hooks: ## Configure git to use repo-tracked hooks
	@if [ "$$(git config core.hooksPath)" != ".githooks" ]; then \
		git config core.hooksPath .githooks; \
		echo "[setup-hooks] core.hooksPath set to .githooks"; \
	fi

.PHONY: ui
ui: setup-hooks
	docker build --platform linux/amd64 -t $(UI_IMG) ui/

.PHONY: v2v-helper
v2v-helper: setup-hooks
	make -C v2v-helper build
	docker build --platform linux/amd64 --build-arg RELEASE_VERSION=$(VERSION) -t $(V2V_IMG) -f v2v-helper/Dockerfile .

.PHONY: test-v2v-helper
test-v2v-helper: setup-hooks
	cd v2v-helper && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./... -v

.PHONY: vjail-controller
vjail-controller: setup-hooks v2v-helper
	make -C k8s/migration/ docker-build

.PHONY: vjail-controller-only
vjail-controller-only: setup-hooks
	make -C k8s/migration/ docker-build

.PHONY: generate-manifests
generate-manifests: setup-hooks vjail-controller ui
	rm -rf image_builder/deploy && mkdir -p image_builder/deploy && chmod 755 image_builder/deploy
	envsubst < ui/deploy/ui.yaml > image_builder/deploy/01ui.yaml
	envsubst < image_builder/configs/version-config.yaml > image_builder/deploy/version-config.yaml
	cp image_builder/cronjob/version-checker.yaml image_builder/deploy/version-checker.yaml
	cp image_builder/configs/vjailbreak-settings.yaml image_builder/deploy/vjailbreak-settings.yaml
	make -C k8s/migration/ build-installer && cp k8s/migration/dist/install.yaml image_builder/deploy/00controller.yaml
	cp deploy/volumeimageprofile-defaults.yaml image_builder/configs/volumeimageprofile-defaults.yaml
	envsubst < vjailbreak-ai/deploy/vjailbreak-ai.yaml > image_builder/deploy/08vjailbreak-ai.yaml
	
.PHONY: generate-debug-prompt
generate-debug-prompt:
	bash scripts/build-debug-prompt.sh

.PHONY: check-skill-freshness
SKILL_WATCHED_FILES := \
	k8s/migration/api/v1alpha1/migration_types.go \
	k8s/migration/api/v1alpha1/migrationplan_types.go \
	v2v-helper/virtv2v/virtv2vops.go \
	pkg/vpwned/server/ai_handler.go

check-skill-freshness:
	@SKILL_DATE=$$(awk '/^last-updated:/{print $$2; exit}' .claude/skills/vjb-debug/SKILL.md); \
	if [ -z "$$SKILL_DATE" ]; then \
	  echo "ERROR: last-updated field not found in SKILL.md" >&2; exit 2; \
	fi; \
	FAIL=0; \
	for f in $(SKILL_WATCHED_FILES); do \
	  if [ ! -f "$$f" ]; then \
	    echo "ERROR: watched file not found: $$f" >&2; FAIL=1; continue; \
	  fi; \
	  FILE_DATE=$$(git log -1 --format=%ci -- "$$f" 2>/dev/null | awk '{print $$1}'); \
	  if [ -z "$$FILE_DATE" ]; then continue; fi; \
	  if [ "$$FILE_DATE" \> "$$SKILL_DATE" ]; then \
	    echo "SKILL STALE: $$f modified ($$FILE_DATE) after skill last-updated date ($$SKILL_DATE). Bump version and last-updated in SKILL.md." >&2; \
	    FAIL=1; \
	  fi; \
	done; \
	exit $$FAIL

.PHONY: build-vpwned
build-vpwned: setup-hooks generate-debug-prompt
	make -C pkg/vpwned docker-build

.PHONY: vjailbreak-ai
vjailbreak-ai: setup-hooks
	docker build --platform linux/amd64 -t $(AI_IMG) vjailbreak-ai/

build-installer: setup-hooks
	make -C k8s/migration/ build-installer 

.PHONY: docker-build-image
docker-build-image: generate-manifests
	rm -rf artifacts/ && mkdir artifacts/
	cp -r k8s/kube-prometheus image_builder/deploy/
	docker build --platform linux/amd64 --output=artifacts/ -t vjailbreak-image:local image_builder/ 

.PHONY: lint
lint: setup-hooks
	make -C k8s/migration/ lint

.PHONY: build-image
build-image: generate-manifests
	rm -rf artifacts/ && mkdir artifacts/
	docker build --platform linux/amd64 --output=artifacts/ -t vjailbreak-image:local image_builder/ 

run-local: setup-hooks
	cd k8s/migration/cmd/ && go run main.go --kubeconfig ${KUBECONFIG} --local true
