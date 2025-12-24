# ============================================================================
# Sloth Kubernetes CLI - Makefile
# ============================================================================
.PHONY: help test test-coverage test-race lint fmt vet build clean install uninstall install-tools ci

# ============================================================================
# Configuration
# ============================================================================
BINARY_NAME     := sloth-kubernetes
VERSION         := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT          := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE      := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
INSTALL_PATH    := /usr/local/bin
COVERAGE_FILE   := coverage.txt
COVERAGE_HTML   := coverage.html
GO_FILES        := $(shell find . -name '*.go' -not -path "./vendor/*")

# Build flags
LDFLAGS := -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.Commit=$(COMMIT)' \
	-X 'main.BuildDate=$(BUILD_DATE)'

# Colors for output
CYAN   := \033[36m
GREEN  := \033[32m
YELLOW := \033[33m
RED    := \033[31m
RESET  := \033[0m

# ============================================================================
# Default target
# ============================================================================
.DEFAULT_GOAL := help

# ============================================================================
# Help
# ============================================================================
help: ## Show this help message
	@echo ''
	@echo '$(CYAN)Sloth Kubernetes CLI$(RESET) - Multi-Cloud Kubernetes Provisioning'
	@echo ''
	@echo '$(GREEN)Usage:$(RESET) make [target]'
	@echo ''
	@echo '$(GREEN)Main targets:$(RESET)'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(CYAN)%-18s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo '$(GREEN)Quick start:$(RESET)'
	@echo '  make build install    Build and install the CLI'
	@echo ''

# ============================================================================
# Build targets
# ============================================================================
build: ## Build the binary
	@echo "$(GREEN)▶ Building $(BINARY_NAME)...$(RESET)"
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(COMMIT)"
	@go build -v -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "$(GREEN)✓ Binary built: ./$(BINARY_NAME)$(RESET)"

build-debug: ## Build with debug symbols (for debugging)
	@echo "$(GREEN)▶ Building $(BINARY_NAME) (debug)...$(RESET)"
	@go build -v -gcflags="all=-N -l" -o $(BINARY_NAME) .
	@echo "$(GREEN)✓ Debug binary built: ./$(BINARY_NAME)$(RESET)"

build-all: ## Build binaries for all platforms
	@echo "$(GREEN)▶ Building for all platforms...$(RESET)"
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "$(GREEN)✓ All binaries built in ./dist/$(RESET)"

# ============================================================================
# Install/Uninstall
# ============================================================================
install: build ## Build and install to /usr/local/bin
	@echo "$(GREEN)▶ Installing $(BINARY_NAME) to $(INSTALL_PATH)...$(RESET)"
	@sudo install -m 755 $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "$(GREEN)✓ Installed: $(INSTALL_PATH)/$(BINARY_NAME)$(RESET)"
	@echo ""
	@echo "$(CYAN)Run '$(BINARY_NAME) --help' to get started$(RESET)"

install-local: build ## Install to ~/bin (no sudo required)
	@echo "$(GREEN)▶ Installing $(BINARY_NAME) to ~/bin...$(RESET)"
	@mkdir -p ~/bin
	@install -m 755 $(BINARY_NAME) ~/bin/$(BINARY_NAME)
	@echo "$(GREEN)✓ Installed: ~/bin/$(BINARY_NAME)$(RESET)"
	@echo ""
	@echo "$(YELLOW)Note: Make sure ~/bin is in your PATH$(RESET)"

uninstall: ## Remove from /usr/local/bin
	@echo "$(YELLOW)▶ Uninstalling $(BINARY_NAME)...$(RESET)"
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "$(GREEN)✓ Uninstalled$(RESET)"

# ============================================================================
# Test targets
# ============================================================================
test: ## Run tests
	@echo "$(GREEN)▶ Running tests...$(RESET)"
	@go test -v -short ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)▶ Running tests with coverage...$(RESET)"
	@go test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@echo ""
	@echo "$(CYAN)Coverage summary:$(RESET)"
	@go tool cover -func=$(COVERAGE_FILE) | grep total

test-coverage-html: test-coverage ## Generate HTML coverage report
	@echo "$(GREEN)▶ Generating HTML coverage report...$(RESET)"
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "$(GREEN)✓ Report: $(COVERAGE_HTML)$(RESET)"
	@open $(COVERAGE_HTML) 2>/dev/null || xdg-open $(COVERAGE_HTML) 2>/dev/null || true

test-race: ## Run tests with race detector
	@echo "$(GREEN)▶ Running tests with race detector...$(RESET)"
	@go test -v -race ./...

# ============================================================================
# Code quality
# ============================================================================
lint: ## Run golangci-lint
	@echo "$(GREEN)▶ Running linter...$(RESET)"
	@golangci-lint run --timeout=5m

fmt: ## Format Go code
	@echo "$(GREEN)▶ Formatting code...$(RESET)"
	@gofmt -s -w $(GO_FILES)
	@goimports -w $(GO_FILES) 2>/dev/null || true
	@echo "$(GREEN)✓ Code formatted$(RESET)"

vet: ## Run go vet
	@echo "$(GREEN)▶ Running go vet...$(RESET)"
	@go vet ./...

# ============================================================================
# Dependencies
# ============================================================================
deps: ## Download and verify dependencies
	@echo "$(GREEN)▶ Downloading dependencies...$(RESET)"
	@go mod download
	@go mod tidy
	@go mod verify
	@echo "$(GREEN)✓ Dependencies ready$(RESET)"

install-tools: ## Install development tools
	@echo "$(GREEN)▶ Installing development tools...$(RESET)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)✓ Tools installed$(RESET)"

# ============================================================================
# Clean
# ============================================================================
clean: ## Clean build artifacts
	@echo "$(YELLOW)▶ Cleaning...$(RESET)"
	@rm -f $(BINARY_NAME)
	@rm -rf dist/
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@go clean -cache -testcache
	@echo "$(GREEN)✓ Clean complete$(RESET)"

clean-all: clean ## Clean everything including module cache
	@echo "$(YELLOW)▶ Cleaning module cache...$(RESET)"
	@go clean -modcache
	@echo "$(GREEN)✓ All caches cleaned$(RESET)"

# ============================================================================
# CI/CD
# ============================================================================
ci: vet lint test-coverage ## Run all CI checks
	@echo ""
	@echo "$(GREEN)============================================$(RESET)"
	@echo "$(GREEN)  All CI checks passed! ✅$(RESET)"
	@echo "$(GREEN)============================================$(RESET)"

# ============================================================================
# Info
# ============================================================================
version: ## Show version information
	@echo "$(CYAN)Sloth Kubernetes CLI$(RESET)"
	@echo "  Version:    $(VERSION)"
	@echo "  Commit:     $(COMMIT)"
	@echo "  Build Date: $(BUILD_DATE)"
	@echo "  Go Version: $(shell go version | cut -d' ' -f3)"
