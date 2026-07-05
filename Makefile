BIN := kb
PKG := ./cmd/kb
SRC := $(shell pwd)
VERSION := $(shell git describe --always --dirty 2>/dev/null || echo dev)
DATE := $(shell date +%Y-%m-%d)

LDFLAGS := -X github.com/Ryong256/kanban/internal/buildinfo.Version=$(VERSION) \
           -X github.com/Ryong256/kanban/internal/buildinfo.Date=$(DATE) \
           -X github.com/Ryong256/kanban/internal/buildinfo.SourceDir=$(SRC)

.PHONY: install build test version

# Bootstrap install: stamps the binary so `kb update` works from anywhere.
install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

version:
	@echo "$(VERSION) $(DATE) $(SRC)"
