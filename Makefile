GO ?= go
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

ROOT:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(ROOT)/bin
REALM_OUT := $(BIN_DIR)/realm

GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
COMMIT_FLAG := -X 'github.com/bitomia/realm/internal/config.BuildGitCommit=$(GIT_COMMIT)'
MAKEFLAGS += --no-print-directory

.PHONY: all
all:
	@echo "Building ($(GIT_COMMIT))..."
	$(GO) mod tidy
	$(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="$(COMMIT_FLAG)"

.PHONY: clean
clean:
	rm -f $(REALM_OUT)

.PHONY: verify-lint-cmd
verify-lint-cmd:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run cmd

.PHONY: verify-lint-daemon
verify-lint-daemon:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run daemon

.PHONY:
verify-fmt:
	$(GO) fmt ./...
