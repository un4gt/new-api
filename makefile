FRONTEND_DIR = ./web
BACKEND_DIR = .

.PHONY: all build-frontend start-backend docker-build docker-push

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

DOCKER_IMAGE ?= tumuer/new-api-for-embeddings-and-reranker
DOCKERFILE ?= Dockerfile
DOCKER_CONTEXT ?= .
DOCKER_TAG ?= latest

docker-build:
	@echo "Building docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f $(DOCKERFILE) $(DOCKER_CONTEXT)

docker-push: docker-build
	@echo "Pushing docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
