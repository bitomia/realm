GO ?= go
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

ROOT:=$(realpath .)

ifeq ($(OS),Windows_NT)
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>nul || echo Unknown)
	RM = del
	MKDIR = if not exist "$(1)" mkdir "$(1)"
	SEP = \\
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm.exe
	REALM_LIB := $(BIN_DIR)$(SEP)librealm.a
else
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
	RM = rm -rf
	MKDIR = mkdir -p $(1)
	SEP = /
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm
	REALM_LIB := $(BIN_DIR)$(SEP)librealm.a
endif


COMMIT_FLAG := -X 'github.com/bitomia/realm/internal/config.BuildGitCommit=$(GIT_COMMIT)'
MAKEFLAGS += --no-print-directory

.PHONY: all
all:
	@echo "Building ($(GIT_COMMIT))..."
	$(GO) mod tidy
	$(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="$(COMMIT_FLAG)"

.PHONY: lib
lib:
	$(GO) build -o $(REALM_LIB) -buildmode=c-archive lib/main.go

$(BIN_DIR):
	$(call MKDIR,$(BIN_DIR))

.PHONY: clean
clean:
	-$(RM) "$(REALM_OUT)"

.PHONY: verify-lint-cmd
verify-lint-cmd:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run cmd

.PHONY: verify-lint-daemon
verify-lint-daemon:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run daemon

.PHONY: verify-fmt
verify-fmt:
	$(GO) fmt ./...

.PHONY: test
test:
	$(GO) test -v ./internal/...
	$(GO) test -v ./daemon/db/...
