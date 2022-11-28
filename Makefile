OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
ALL_PLATFORM = linux/amd64

# Image URL to use all building/pushing image targets
REGISTRY ?= sh4d1
IMG ?= scaleway-k8s-node-coffee
FULL_IMG ?= $(REGISTRY)/$(IMG)

IMAGE_TAG ?= $(shell git rev-parse HEAD)

DOCKER_CLI_EXPERIMENTAL ?= enabled

all: fmt vet
	go build -o bin/scaleway-k8s-node-coffee ./cmd/

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

run: fmt vet
	go run ./cmd/main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Deploy the controller
deploy:
	kubectl apply -f ./deploy -n scaleway-k8s-node-coffee

# Build the docker image
docker-build: test
	docker build --platform=linux/$(ARCH) -f Dockerfile . -t ${FULL_IMG}

# Push the docker image
docker-push:
	docker push ${FULL_IMG}

docker-buildx-all:
	@echo "Making release for tag $(IMAGE_TAG)"
	docker buildx build --platform=$(ALL_PLATFORM) -f Dockerfile --push -t $(FULL_IMG):$(IMAGE_TAG) .

release: docker-buildx-all
