# Backup Health Monitor Makefile

# Binary name
BINARY_NAME=backup-health-monitor

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Main package path
MAIN_PATH=./cmd/backup-health-monitor

# Build directory
BUILD_DIR=./target

# Deployment configuration
CONFIG_DIR=/etc/backup-health
SERVICE_DIR=/etc/systemd/system
INSTALL_DIR=/usr/local/bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: all build clean test deps tidy run help deploy uninstall release docs fmt lint

# Default target
all: clean deps build

# Build the binary for the host architecture
build:
	@echo "Building $(BINARY_NAME) for host..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 $(GOBUILD) -a -trimpath -ldflags "-X main.version=$(VERSION) -extldflags '-static' -w -s" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete. Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

# Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GOGET) -v ./...

# Tidy dependencies
tidy:
	@echo "Tidying module dependencies..."
	@$(GOMOD) tidy

# Install binary to system
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@sudo chmod +x $(INSTALL_DIR)/$(BINARY_NAME)

# Full deployment (install binary, config, and systemd service)
deploy: build
	@echo "Deploying $(BINARY_NAME)..."
	@sudo mkdir -p $(CONFIG_DIR)
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@sudo chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@sudo cp examples/config-production.yaml $(CONFIG_DIR)/config.yaml
	@sudo cp examples/backup-health-monitor.service $(SERVICE_DIR)/
	@sudo systemctl daemon-reload
	@echo "Deployment complete. Configure $(CONFIG_DIR)/config.yaml and run:"
	@echo "  sudo systemctl enable backup-health-monitor.service"
	@echo "  sudo systemctl start backup-health-monitor.service"

# Uninstall from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo systemctl stop backup-health-monitor.service 2>/dev/null || true
	@sudo systemctl disable backup-health-monitor.service 2>/dev/null || true
	@sudo rm -f $(SERVICE_DIR)/backup-health-monitor.service
	@sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@sudo systemctl daemon-reload
	@echo "Uninstall complete. Configuration files in $(CONFIG_DIR) were left intact."

# Create cross-compiled release packages for all target architectures
release: clean
	@echo "Creating release builds and packages..."
	@mkdir -p $(BUILD_DIR)/release
	@VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	for arch in "amd64" "arm64" "arm"; do \
		export GOOS=linux; \
		export GOARCH=$$arch; \
		export GOARM=""; \
		platform_suffix="linux-$$arch"; \
		if [ "$$arch" = "arm" ]; then \
			export GOARM="7"; \
			platform_suffix="linux-armv7"; \
		elif [ "$$arch" = "arm64" ]; then \
			platform_suffix="linux-aarch64"; \
		fi; \
		echo "--> Building for $$platform_suffix..."; \
		raw_binary="$(BUILD_DIR)/release/$(BINARY_NAME)-$${VERSION}-$$platform_suffix"; \
		package_dir_name="$(BINARY_NAME)-$${VERSION}-$$platform_suffix.d"; \
		package_dir="$(BUILD_DIR)/release/$${package_dir_name}"; \
		# remove stale outputs (defensive) \
		rm -f $$raw_binary; \
		rm -rf $$package_dir; \
		CGO_ENABLED=0 $(GOBUILD) -a -trimpath -ldflags "-X main.version=$${VERSION} -extldflags '-static' -w -s" -o $$raw_binary $(MAIN_PATH); \
		\
	done
	@echo "Release binaries created in $(BUILD_DIR)/release/"

# Coverage report
coverage:
	@echo "Generating coverage report..."
	@$(GOTEST) -coverprofile=$(BUILD_DIR)/coverage.out ./...
	@$(GOCMD) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated: $(BUILD_DIR)/coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@go mod tidy

# Lint code
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  all          - Clean, download deps, and build"
	@echo "  build        - Build the binary for current platform"
	@echo "  clean        - Clean build artifacts"
	@echo ""
	@echo "Development targets:"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy module dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code (requires golangci-lint)"
	@echo ""
	@echo "Testing targets:"
	@echo "  test         - Run all tests"
	@echo "  coverage     - Generate coverage report"
	@echo ""
	@echo "Deployment targets:"
	@echo "  install      - Install binary to /usr/local/bin"
	@echo "  deploy       - Full deployment (binary + config + service)"
	@echo "  uninstall    - Remove from system"
	@echo "  release      - Create cross-compiled release packages"
	@echo ""
	@echo "Documentation targets:"
	@echo "  docs         - List available documentation"
	@echo "  help         - Show this help"
