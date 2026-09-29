BINARY   ?= app
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE    ?= go-echo-template
PLATFORMS?= linux/amd64,linux/arm64

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## Run the API locally
	go run ./cmd/api

.PHONY: build
build: ## Build the binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/api

.PHONY: test
test: ## Run tests with race detection
	go test -race -count=1 ./...

.PHONY: lint
lint: ## Vet and format-check
	go vet ./...
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "run: gofmt -w ." && exit 1)

.PHONY: tidy
tidy: ## Tidy modules
	go mod tidy

.PHONY: docker
docker: ## Build the image for the local platform
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

.PHONY: docker-multi
docker-multi: ## Build and push a multi-platform image (needs a buildx builder)
	docker buildx build --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) --push .

.PHONY: up
up: ## Start api + postgres
	docker compose up --build -d

.PHONY: down
down: ## Stop everything
	docker compose down

.PHONY: logs
logs: ## Tail the api logs
	docker compose logs -f api
