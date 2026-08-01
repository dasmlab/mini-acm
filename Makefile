# ==============================================================================
# mock-me — LAB / TEST / DEV ONLY
# Virtual rack for ACM hub SNO + compact managed clusters. See README.md.
# ==============================================================================

BINARY_NAME    := mock-me
BIN_DIR        := bin
IMAGE_TOOL     ?= podman
IMAGE_NAME     ?= quay.io/dasmlab/mock-me
IMAGE_TAG      ?= latest
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS        := -X main.version=$(VERSION)

.PHONY: help
help: ## Show this help
	@echo "mock-me — LAB/TEST/DEV only (see README.md)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: web
web: ## Build Vue UI into cmd/.../static for go:embed
	cd web && npm install && npm run build
	rm -rf cmd/$(BINARY_NAME)/static/*
	cp -a web/dist/. cmd/$(BINARY_NAME)/static/
	@touch cmd/$(BINARY_NAME)/static/.gitkeep

.PHONY: build
build: ## Build the CLI into ./bin
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

.PHONY: build-all
build-all: web build ## Build UI then Go binary

.PHONY: serve
serve: build ## Run UI+API on :8080
	./$(BIN_DIR)/$(BINARY_NAME) serve --listen :8080 --data-dir ./data

.PHONY: test
test: ## Run go vet + go test
	go vet ./...
	go test ./...

.PHONY: fmt
fmt: ## gofmt all Go source
	gofmt -l -w $$(find . -name '*.go' -not -path './vendor/*')

.PHONY: image
image: ## Build the container image (podman/docker)
	$(IMAGE_TOOL) build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Containerfile .

.PHONY: image-push
image-push: ## Push the container image
	$(IMAGE_TOOL) push $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: hub-dry
hub-dry: build ## Dry-run hub create from example config
	./$(BIN_DIR)/$(BINARY_NAME) --dry-run --manual hub create --config config/hub.example.yaml --skip-wait --skip-acm

.PHONY: cluster-dry
cluster-dry: build ## Dry-run cluster create from example config
	./$(BIN_DIR)/$(BINARY_NAME) --dry-run --manual cluster create --config config/cluster.example.yaml

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
