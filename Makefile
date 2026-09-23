BIN := kb
PKG := ./cmd/kb
SRC := $(shell pwd)
VERSION := $(shell git describe --always --dirty 2>/dev/null || echo dev)
DATE := $(shell date +%Y-%m-%d)
XDG_CONFIG_HOME_EFFECTIVE := $(if $(strip $(XDG_CONFIG_HOME)),$(XDG_CONFIG_HOME),$(HOME)/.config)
OPENCODE_PLUGIN_DIR := $(XDG_CONFIG_HOME_EFFECTIVE)/opencode/plugins
OMARCHY_PLUGIN_DIR := $(HOME)/.config/omarchy/plugins/io.github.ryong256.kanban

LDFLAGS := -X github.com/Ryong256/kanban/internal/buildinfo.Version=$(VERSION) \
           -X github.com/Ryong256/kanban/internal/buildinfo.Date=$(DATE) \
           -X github.com/Ryong256/kanban/internal/buildinfo.SourceDir=$(SRC)

.PHONY: install install-all install-opencode install-claude install-omarchy build test version

# Bootstrap install: stamps the binary so `kb update` works from anywhere.
install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

# Install the core plus every adapter.
install-all: install-opencode install-claude install-omarchy

# Adapters are thin invokers of the core, so an adapter newer than the binary it
# calls passes flags that binary does not have. Both targets rebuild kb first.
install-opencode: install
	install -d "$(OPENCODE_PLUGIN_DIR)"
	install -m 0644 "$(CURDIR)/integrations/opencode/kanban.ts" "$(OPENCODE_PLUGIN_DIR)/kanban.ts"

# kanban.sh speaks the adapter contract; the stop hook translates it into the
# Claude Code hook protocol. Both live next to each other because the hook
# resolves the adapter relative to its own directory.
install-claude: install
	install -d "$(HOME)/.claude/hooks"
	install -m 0755 "$(CURDIR)/integrations/claude/kanban.sh" "$(HOME)/.claude/hooks/kanban.sh"
	install -m 0755 "$(CURDIR)/integrations/claude/stop-hook.sh" "$(HOME)/.claude/hooks/kanban-reconcile-stop.sh"
	install -m 0755 "$(CURDIR)/integrations/claude/session-start.sh" "$(HOME)/.claude/hooks/kanban-session-start.sh"
	install -m 0755 "$(CURDIR)/integrations/claude/posttool-memsave.sh" "$(HOME)/.claude/hooks/kanban-posttool-memsave.sh"

# The bar plugin is copied, never symlinked into the working tree: checking out
# a commit without integrations/omarchy would dangle the link, and quickshell
# skips a dangling plugin silently. An older symlink install is replaced; a real
# directory is kept and only our files inside it are overwritten.
install-omarchy:
	if [ -L "$(OMARCHY_PLUGIN_DIR)" ]; then rm "$(OMARCHY_PLUGIN_DIR)"; fi
	install -d "$(OMARCHY_PLUGIN_DIR)"
	install -m 0644 "$(CURDIR)/integrations/omarchy/Main.qml" "$(OMARCHY_PLUGIN_DIR)/Main.qml"
	install -m 0644 "$(CURDIR)/integrations/omarchy/manifest.json" "$(OMARCHY_PLUGIN_DIR)/manifest.json"
	install -m 0644 "$(CURDIR)/integrations/omarchy/README.md" "$(OMARCHY_PLUGIN_DIR)/README.md"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

version:
	@echo "$(VERSION) $(DATE) $(SRC)"
