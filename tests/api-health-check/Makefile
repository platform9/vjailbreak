.PHONY: build test docker-build docker-push deploy clean

# Variables
IMAGE_NAME ?= platform9/vjailbreak-api-health-check
IMAGE_TAG ?= latest
REGISTRY ?= quay.io

# Build the Go binary locally
build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-health-check .

# Run tests
test:
	go test -v ./...

# Build Docker image
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Tag for registry
docker-tag:
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Push to registry
docker-push: docker-tag
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Build and push
release: docker-build docker-push

# Deploy to Kubernetes
deploy:
	kubectl apply -f k8s/

# Delete from Kubernetes
undeploy:
	kubectl delete -f k8s/ || true

# Clean build artifacts
clean:
	rm -f api-health-check
	go clean

# Run locally (requires cluster access)
run:
	go run main.go

# Run with custom base URL
run-custom:
	BASE_URL=https://10.9.2.145 SKIP_SSL_VERIFY=true go run main.go
