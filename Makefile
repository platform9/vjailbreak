export UI_IMG ?= tanaypf9/vjailbreak-ui:v0.2
export V2V_IMG ?= tanaypf9/v2v-helper:v0.2
export CONTROLLER_IMG ?= tanaypf9/vjailbreak-controller:v0.2

.PHONY: ui
ui:
	docker build --platform linux/amd64 -t $(UI_IMG) ui/
	docker push $(UI_IMG)

.PHONY: v2v-helper
v2v-helper:
	docker build --platform linux/amd64 -t $(V2V_IMG) v2v-helper/
	docker push $(V2V_IMG)

.PHONY: vjail-controller
vjail-controller: v2v-helper
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

