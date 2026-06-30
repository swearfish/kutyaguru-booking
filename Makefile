# Makefile — cross-build the Windows binary from a macOS host.
#
# Wails v3 Windows builds are pure Go (CGO disabled by default): the WebView2
# runtime is loaded at runtime via a pure-Go loader, so the Windows .exe can be
# produced on macOS with only the Go toolchain, the wails3 CLI, and Node/npm.
# No mingw-w64, Zig, or Docker is required.
#
# Quick start:
#   make windows-setup        # one-time: install the wails3 CLI + frontend deps
#   make windows              # cross-build bin/kutyaguru-booking.exe (amd64)
#   make windows ARCH=arm64   # cross-build for Windows on ARM
#   make clean

# Keep this in sync with the wails/v3 version pinned in go.mod.
WAILS_VERSION ?= v3.0.0-alpha2.109
ARCH          ?= amd64
APP_NAME      := kutyaguru-booking
EXE           := bin/$(APP_NAME).exe

.DEFAULT_GOAL := help

.PHONY: help windows-setup windows windows-build windows-package clean

help: ## Show this help
	@echo "Cross-build the Windows binary on a macOS host."
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | \
		awk -F':.*## ' '{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Variables: ARCH=$(ARCH) (amd64|arm64)  WAILS_VERSION=$(WAILS_VERSION)"

windows-setup: ## Install the toolchain for Windows cross-build (wails3 CLI + frontend deps)
	@command -v go >/dev/null 2>&1 || { echo "ERROR: Go toolchain not found — install Go: https://go.dev/dl/"; exit 1; }
	@command -v npm >/dev/null 2>&1 || { echo "ERROR: npm not found — install Node.js: https://nodejs.org/"; exit 1; }
	@echo ">> Installing wails3 CLI ($(WAILS_VERSION)) ..."
	go install github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION)
	@command -v wails3 >/dev/null 2>&1 || { \
		echo "ERROR: wails3 is not on PATH after install."; \
		echo "       Add \"$$(go env GOPATH)/bin\" to your PATH and re-run."; \
		exit 1; }
	@echo ">> Installing frontend dependencies ..."
	cd frontend && npm install
	@echo ">> Setup complete. Build with: make windows"

windows: windows-build ## Cross-build the Windows GUI .exe (alias for windows-build)

windows-build: ## Cross-build the Windows GUI .exe -> bin/<app>.exe
	@command -v wails3 >/dev/null 2>&1 || { echo "ERROR: wails3 not found — run: make windows-setup"; exit 1; }
	CGO_ENABLED=0 wails3 task windows:build ARCH=$(ARCH) CGO_ENABLED=0
	@echo ">> Built $(EXE)"
	@file "$(EXE)" 2>/dev/null || true

windows-package: ## Build the NSIS installer (requires a Windows host with makensis)
	CGO_ENABLED=0 wails3 task windows:package ARCH=$(ARCH) CGO_ENABLED=0

clean: ## Remove build artifacts (bin/, stray .syso)
	rm -rf bin
	rm -f *.syso
