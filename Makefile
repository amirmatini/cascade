.PHONY: build run clean install test fmt vet

BINARY_NAME=cascade
BUILD_DIR=build
GO=go

all: fmt vet build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) cmd/cascade/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml

install:
	@echo "Installing $(BINARY_NAME)..."
	@$(GO) install cmd/cascade/main.go

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf cache
	@echo "Clean complete"

test:
	@echo "Running tests..."
	@$(GO) test -v ./...

fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...

vet:
	@echo "Vetting code..."
	@$(GO) vet ./...

deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy

build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 cmd/cascade/main.go
	@GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 cmd/cascade/main.go
	@echo "All builds complete"
	@ls -lh $(BUILD_DIR)/

release:
	@echo "Creating release..."
	@mkdir -p $(BUILD_DIR)
	@$(MAKE) build-all
	@cd $(BUILD_DIR) && sha256sum cascade-* > SHA256SUMS
	@echo "Release ready in $(BUILD_DIR)/"

help:
	@echo "Available targets:"
	@echo "  build        - Build the binary for current platform"
	@echo "  build-all    - Build for Linux (amd64/arm64)"
	@echo "  release      - Build all platforms and generate checksums"
	@echo "  run          - Build and run the proxy"
	@echo "  install      - Install the binary"
	@echo "  clean        - Remove build artifacts and cache"
	@echo "  test         - Run tests"
	@echo "  fmt          - Format code"
	@echo "  vet          - Vet code"
	@echo "  deps         - Download and tidy dependencies"

