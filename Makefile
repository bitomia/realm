GO ?= go
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0
TAGS ?=

ROOT:=$(realpath .)

ifeq ($(OS),Windows_NT)
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>nul || echo Unknown)
	RM = del
	MKDIR = if not exist "$(1)" mkdir "$(1)"
	SEP = \\
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm.exe
else
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
	RM = rm -rf
	MKDIR = mkdir -p $(1)
	SEP = /
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm
endif


COMMIT_FLAG := -X 'github.com/bitomia/realm/common/config.BuildGitCommit=$(GIT_COMMIT)'

.PHONY: all
all:
	@echo "Building ($(GIT_COMMIT))..."
	CGO_ENABLED=0 $(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="$(COMMIT_FLAG)" $(if $(TAGS),-tags "$(TAGS)")

.PHONY: tidy
tidy:
	$(GO) mod tidy

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

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test -v ./drivers/...
	$(GO) test -v ./daemon/db/...
	$(GO) test -v ./daemon/containers/...
