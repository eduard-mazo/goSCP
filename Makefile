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

# ── Advantech ICR-323x industrial router (linux/arm/v7) ───────────────────────
# Modeled on goMqttModbus's icr323x target. The ICR runs a BusyBox userland with
# no package manager and a glibc far older than a desktop's, so — like the
# Android/Termux target — we cross-compile a FULLY STATIC pure-Go binary
# (CGO_ENABLED=0): no ELF interpreter, no libc dependency, runs as-is on the
# device. The frontend is embedded by the `web` prerequisite.
#
# User apps/configs/logs live under /root, which persists across reboots via
# OverlayFS (/opt is firmware-reserved; /tmp and /var are volatile). Override
# DEVICE with the router IP, e.g. `make deploy-icr DEVICE=192.168.1.1`.
PORT          ?= 8080
DEVICE        ?= 192.168.1.1
DEVICE_USER   ?= root
ICR_BIN_DIR   ?= /root/bin
ICR_DATA_DIR  ?= /root/data
ICR_LOG_DIR   ?= /root/log
ICR_BINARY    := $(BINARY)-icr-armv7

.PHONY: build-icr
build-icr: web ## Cross-build embedded binary for Advantech ICR-323x (arm/v7, static)
	@echo "$(C_CYAN)» ICR-323x build $(BINARY) $(VERSION)$(C_RST)"
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
	  go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(ICR_BINARY) $(PKG)
	@echo "$(C_GREEN)✓ $(BUILD_DIR)/$(ICR_BINARY) (static arm/v7, no interpreter)$(C_RST)"

.PHONY: verify-arm
verify-arm: ## Assert the ICR binary is a static ARM ELF
	@file $(BUILD_DIR)/$(ICR_BINARY) | grep -q "ARM" || { echo "ERROR: $(BUILD_DIR)/$(ICR_BINARY) is not an ARM binary — run 'make build-icr' first"; exit 1; }
	@file $(BUILD_DIR)/$(ICR_BINARY) | grep -q "statically linked" || echo "WARN: $(BUILD_DIR)/$(ICR_BINARY) is not statically linked — check CGO/toolchain"
	@file $(BUILD_DIR)/$(ICR_BINARY); ls -lh $(BUILD_DIR)/$(ICR_BINARY)

# deploy-icr only builds + prints copy-paste steps; nothing is pushed to the
# device automatically (no credentials in the build). Override DEVICE=<ip>.
define icr_deploy_steps
	@echo ""
	@echo "Built $(ICR_BINARY) for the ICR-323x. Copy it to the device by hand (/root persists via OverlayFS):"
	@echo ""
	@echo "  # 1. create dirs, then copy the binary"
	@echo "  ssh $(DEVICE_USER)@$(DEVICE) 'mkdir -p $(ICR_BIN_DIR) $(ICR_DATA_DIR) $(ICR_LOG_DIR)'"
	@echo "  scp $(BUILD_DIR)/$(ICR_BINARY) $(DEVICE_USER)@$(DEVICE):$(ICR_BIN_DIR)/$(BINARY)"
	@echo ""
	@echo "  # 2. make executable + test run (serves $(ICR_DATA_DIR) on port $(PORT))"
	@echo "  ssh $(DEVICE_USER)@$(DEVICE) 'chmod +x $(ICR_BIN_DIR)/$(BINARY)'"
	@echo "  ssh $(DEVICE_USER)@$(DEVICE) 'GOSCP_PASSWORD=changeme $(ICR_BIN_DIR)/$(BINARY) --root $(ICR_DATA_DIR) --port $(PORT)'"
	@echo ""
	@echo "  # 3. (optional) install a boot service:  make service-icr"
endef

.PHONY: deploy-icr
deploy-icr: build-icr verify-arm ## Build for ICR-323x + print manual deploy steps
	$(icr_deploy_steps)

# A BusyBox init.d service (start/stop/restart/status) so goSCP survives reboots.
# The ICR firmware's BusyBox has no start-stop-daemon applet, so we supervise the
# process with nohup + a PID file directly. Set a fixed token and/or password at
# the top of the script so they persist (otherwise a random token is generated
# and logged on every boot).
define ICR_INITD
#!/bin/sh
# $(BINARY) — goSCP HTTP file-exchange server (BusyBox init.d)
# Generated by `make service-icr`. Plain POSIX shell (no start-stop-daemon).
#
# Fixed credentials (leave GOSCP_TOKEN empty to keep auto-generating one):
GOSCP_TOKEN=
GOSCP_PASSWORD=
export GOSCP_TOKEN GOSCP_PASSWORD

DAEMON=$(ICR_BIN_DIR)/$(BINARY)
RUNDIR=/root/run
PIDFILE=$$RUNDIR/$(BINARY).pid
LOGFILE=$(ICR_LOG_DIR)/$(BINARY).log
ARGS="--root $(ICR_DATA_DIR) --port $(PORT)"

running() {
    [ -f "$$PIDFILE" ] && kill -0 "$$(cat "$$PIDFILE")" 2>/dev/null
}
start() {
    if running; then
        echo "$(BINARY) already running (PID $$(cat "$$PIDFILE"))"
        return 0
    fi
    mkdir -p "$$RUNDIR" $(ICR_LOG_DIR) $(ICR_DATA_DIR)
    echo "Starting $(BINARY)..."
    nohup "$$DAEMON" $$ARGS >> "$$LOGFILE" 2>&1 &
    echo $$! > "$$PIDFILE"
    echo "OK (PID $$(cat "$$PIDFILE"))"
}
stop() {
    echo "Stopping $(BINARY)..."
    if [ -f "$$PIDFILE" ]; then
        PID=$$(cat "$$PIDFILE")
        kill "$$PID" 2>/dev/null
        i=0
        while kill -0 "$$PID" 2>/dev/null && [ $$i -lt 10 ]; do sleep 1; i=$$((i+1)); done
        kill -9 "$$PID" 2>/dev/null
        rm -f "$$PIDFILE"
    fi
    echo "OK"
}
case "$$1" in
    start)   start ;;
    stop)    stop ;;
    restart) stop; sleep 2; start ;;
    status)
        if running; then
            echo "$(BINARY) running (PID $$(cat "$$PIDFILE"))"
        else
            echo "$(BINARY) stopped"
        fi ;;
    *) echo "Usage: $$0 {start|stop|restart|status}" ;;
esac
endef

$(BUILD_DIR)/$(BINARY).init: Makefile
	@mkdir -p $(BUILD_DIR)
	$(file >$@,$(ICR_INITD))
	@echo "wrote $@"

.PHONY: service-icr
service-icr: $(BUILD_DIR)/$(BINARY).init ## Generate a BusyBox init.d service for the ICR + install steps
	@echo ""
	@echo "Wrote $(BUILD_DIR)/$(BINARY).init (BusyBox init.d service). Install it by hand:"
	@echo ""
	@echo "  scp $(BUILD_DIR)/$(BINARY).init $(DEVICE_USER)@$(DEVICE):/etc/init.d/$(BINARY)"
	@echo "  ssh $(DEVICE_USER)@$(DEVICE) 'chmod +x /etc/init.d/$(BINARY)'"
	@echo "  ssh $(DEVICE_USER)@$(DEVICE) '/etc/init.d/$(BINARY) start'"

.PHONY: release
release: build-linux build-windows build-android build-icr ## Build Linux + Windows + Android + ICR-323x release binaries
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
