SHELL := /bin/bash

export REPO ?= platform9
export TAG ?= latest
export UI_IMG ?= ${REPO}/vjailbreak-ui:${TAG}
export V2V_IMG ?= ${REPO}/v2v-helper:${TAG}
export CONTROLLER_IMG ?= ${REPO}/vjailbreak-controller:${TAG}

.PHONY: ui
ui:
	docker build --platform linux/amd64 -t $(UI_IMG) ui/
	docker push $(UI_IMG)

.PHONY: v2v-helper
v2v-helper:
	docker build --platform linux/amd64 -t $(V2V_IMG) v2v-helper/
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

.PHONY: build-image
build-image: generate-manifests
	rm -rf artifacts/ && mkdir artifacts/
	docker build --platform linux/amd64 --output=artifacts/ -t vjailbreak-image:local image_builder/ 
