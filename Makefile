.PHONY: build install run chat serve test vet clean tidy help

# ── Binary names ──
BINARY   = pi-agent
BRIDGE   = pi-feishu-bridge
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -X main.version=$(VERSION)

# ── Paths ──
GO = go
BIN_DIR = bin

# ── Default ──
help:
	@echo "π-go Build System"
	@echo ""
	@echo "  make build       Build all binaries to bin/"
	@echo "  make install     Install pi-agent to $$GOPATH/bin"
	@echo "  make run         Quick run (single prompt)"
	@echo "  make chat        Start interactive TUI chat"
	@echo "  make serve       Start HTTP server"
	@echo "  make test        Run all tests"
	@echo "  make vet         Run go vet"
	@echo "  make clean       Remove bin/ directory"
	@echo "  make tidy        Run go mod tidy"
	@echo ""
	@echo "  make desktop     Build desktop app (Electron)"
	@echo "  make android     Build Android APK"
	@echo ""

# ── Build ──
build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/pi-agent
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BRIDGE) ./cmd/pi-feishu-bridge
	@echo "✅ Built $(BIN_DIR)/$(BINARY) and $(BIN_DIR)/$(BRIDGE)"

build-agent:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/pi-agent
	@echo "✅ Built $(BIN_DIR)/$(BINARY)"

# ── Install (puts binary in $GOPATH/bin) ──
install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/pi-agent
	@echo "✅ Installed pi-agent to $$($(GO) env GOPATH)/bin/"
	@echo "   Run: pi-agent --mode chat"

# ── Run ──
run:
	$(GO) run ./cmd/pi-agent --mode run --prompt "$(P)"

chat:
	$(GO) run ./cmd/pi-agent --mode chat

serve:
	$(GO) run ./cmd/pi-agent --mode serve --listen :8080

# ── Quality ──
test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf $(BIN_DIR)

tidy:
	$(GO) mod tidy

# ── Desktop / Android ──
desktop:
	cd desktop && npm install && npm run build

android:
	bash scripts/build-android.sh
