BIN := kb
PKG := ./cmd/kb
SRC := $(shell pwd)
VERSION := $(shell git describe --always --dirty 2>/dev/null || echo dev)
DATE := $(shell date +%Y-%m-%d)
XDG_CONFIG_HOME_EFFECTIVE := $(if $(strip $(XDG_CONFIG_HOME)),$(XDG_CONFIG_HOME),$(HOME)/.config)
OPENCODE_PLUGIN_DIR := $(XDG_CONFIG_HOME_EFFECTIVE)/opencode/plugins

LDFLAGS := -X github.com/Ryong256/kanban/internal/buildinfo.Version=$(VERSION) \
           -X github.com/Ryong256/kanban/internal/buildinfo.Date=$(DATE) \
           -X github.com/Ryong256/kanban/internal/buildinfo.SourceDir=$(SRC)

.PHONY: install install-opencode build test version

# Bootstrap install: stamps the binary so `kb update` works from anywhere.
install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

# Install only the repository-owned adapter; OpenCode discovers it on restart.
install-opencode:
	install -d "$(OPENCODE_PLUGIN_DIR)"
	install -m 0644 "$(CURDIR)/integrations/opencode/kanban.ts" "$(OPENCODE_PLUGIN_DIR)/kanban.ts"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

version:
	@echo "$(VERSION) $(DATE) $(SRC)"
