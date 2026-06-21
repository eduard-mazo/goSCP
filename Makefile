# ─────────────────────────────────────────────────────────────────────────────
#  goSCP — embeddable HTTP file-exchange server (Go + Vue/shadcn/Tailwind)
#  Single offline binary: the Vue build is embedded into the Go executable.
# ─────────────────────────────────────────────────────────────────────────────

# Config -----------------------------------------------------------------------
BINARY      := goscp
PKG         := ./cmd/goscp
WEB_DIR     := web
DIST_DIR    := internal/assets/dist
BUILD_DIR   := bin
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     := -trimpath

# Frontend package manager.
PM          := pnpm

# Colors for nicer output
C_GREEN := \033[0;32m
C_CYAN  := \033[0;36m
C_RST   := \033[0m

.DEFAULT_GOAL := help

# Help -------------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@echo ""
	@echo "$(C_CYAN)goSCP — make targets$(C_RST)"
	@echo ""
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(C_GREEN)%-18s$(C_RST) %s\n", $$1, $$2}'
	@echo ""

# Dependencies -----------------------------------------------------------------
.PHONY: deps
deps: web-deps ## Install all dependencies (Go modules + npm)
	go mod download

.PHONY: web-deps
web-deps: ## Install frontend dependencies
	cd $(WEB_DIR) && $(PM) install

# Frontend ---------------------------------------------------------------------
.PHONY: web
web: web-deps ## Build the Vue frontend into the Go embed directory
	@echo "$(C_CYAN)» building frontend → $(DIST_DIR)$(C_RST)"
	cd $(WEB_DIR) && $(PM) run build

.PHONY: web-dev
web-dev: ## Run the Vite dev server (proxies /api → :8080)
	cd $(WEB_DIR) && $(PM) run dev

# Backend ----------------------------------------------------------------------
.PHONY: build
build: web ## Build the single embedded binary for the host OS
	@echo "$(C_CYAN)» building $(BINARY) $(VERSION)$(C_RST)"
	mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(BINARY)$(C_RST)"

.PHONY: build-go
build-go: ## Build Go only (assumes frontend already built)
	mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(PKG)

# Cross-compilation: embeds the same frontend, then builds per target ----------
.PHONY: build-linux
build-linux: web ## Cross-build embedded binary for Linux (amd64)
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(BINARY)-linux-amd64$(C_RST)"

.PHONY: build-windows
build-windows: web ## Cross-build embedded binary for Windows (amd64)
	mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(BINARY)-windows-amd64.exe$(C_RST)"

# Android / Termux runs a Linux userland but uses bionic, not glibc. A PIE build
# is dynamically linked against /lib/ld-linux-aarch64.so.1, which does not exist
# on Android (the loader is /system/bin/linker64) — so it fails with
# "no such file or directory". We therefore build a FULLY STATIC binary
# (CGO_ENABLED=0, no -buildmode=pie): it has no ELF interpreter and runs anywhere.
.PHONY: build-android
build-android: web ## Cross-build embedded binary for Android/Termux (arm64, static)
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-android-arm64 $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(BINARY)-android-arm64 (static, no interpreter)$(C_RST)"

# Native, on-device build INSIDE Termux. Unlike `build-android` (which
# cross-compiles from a desktop host), this runs Termux's own Go toolchain on
# the phone, so GOOS/GOARCH are left at the host defaults (android/arm64).
# CGO is disabled so the result stays self-contained and does not need Termux's
# clang/NDK at build time or any shared libs at run time. Requires Go in Termux:
#   pkg install golang
# The frontend is embedded, so it must be built first; on a phone that is slow,
# so if you already have internal/assets/dist populated use `build-termux-go`.
.PHONY: build-termux
build-termux: web ## Compile natively inside Termux (android/arm64, embeds frontend)
	@echo "$(C_CYAN)» native Termux build $(BINARY) $(VERSION)$(C_RST)"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(BINARY) (native Termux build)$(C_RST)"

.PHONY: build-termux-go
build-termux-go: ## Native Termux build, Go only (assumes frontend already built)
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(BINARY) (native Termux build)$(C_RST)"

.PHONY: run-termux
run-termux: build-termux ## Native Termux build + run (vars: ROOT=. PORT=8080)
	./$(BUILD_DIR)/$(BINARY) --root $(or $(ROOT),.) --port $(or $(PORT),8080)

.PHONY: release
release: build-linux build-windows build-android ## Build Linux + Windows + Android release binaries
	@echo "$(C_GREEN)✓ release binaries in $(BUILD_DIR)/$(C_RST)"
	@ls -lh $(BUILD_DIR)

# Run --------------------------------------------------------------------------
.PHONY: run
run: build ## Build everything and run the server (vars: ROOT=. PORT=8080)
	./$(BUILD_DIR)/$(BINARY) --root $(or $(ROOT),.) --port $(or $(PORT),8080)

.PHONY: dev
dev: ## Run backend (vars: ROOT=. PORT=8080) — pair with `make web-dev`
	go run $(PKG) --root $(or $(ROOT),.) --port $(or $(PORT),8080) --token devtoken

# Quality ----------------------------------------------------------------------
.PHONY: test
test: ## Run Go tests
	go test ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format Go code
	gofmt -s -w .

.PHONY: lint
lint: vet ## Lint Go (vet) and frontend
	cd $(WEB_DIR) && $(PM) run lint || true

.PHONY: tidy
tidy: ## Tidy Go modules
	go mod tidy

# Housekeeping -----------------------------------------------------------------
.PHONY: clean
clean: ## Remove build artifacts and embedded frontend
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	@printf 'Frontend build output is placed here by `make web`.\n' > $(DIST_DIR)/.gitkeep
	@echo "$(C_GREEN)✓ cleaned$(C_RST)"

.PHONY: distclean
distclean: clean ## clean + remove node_modules
	rm -rf $(WEB_DIR)/node_modules
