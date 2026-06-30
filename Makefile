GO ?= go
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

ROOT:=$(realpath .)

ifeq ($(OS),Windows_NT)
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>nul || echo Unknown)
	GIT_TAG := $(shell git describe --tags --abbrev=0 2>nul || echo dev)
	RM = del
	MKDIR = if not exist "$(1)" mkdir "$(1)"
	SEP = \\
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm.exe
else
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
	GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
	RM = rm -rf
	MKDIR = mkdir -p $(1)
	SEP = /
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm
endif

VERSION := $(GIT_TAG)-$(GIT_COMMIT)
NETPLANE_STATIC_DIR := $(ROOT)/bin/netplane
EXT_LDFLAGS := -extldflags '-L$(NETPLANE_STATIC_DIR) -lm'

.PHONY: all
all:
	@echo "Building $(VERSION)..."
	$(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="-X 'github.com/bitomia/realm/common/config.BuildGitCommit=$(GIT_COMMIT)' -X 'github.com/bitomia/realm/common/config.Version=$(GIT_TAG)'"

.PHONY: debug
debug:
	@echo "Building debug $(VERSION)..."
	$(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -gcflags="all=-N -l" -ldflags="-X 'github.com/bitomia/realm/common/config.BuildGitCommit=$(GIT_COMMIT)' -X 'github.com/bitomia/realm/common/config.Version=$(GIT_TAG)'"

.PHONY: ee
ee: netplane
	@echo "Building $(VERSION)-ee (static musl via zig)..."
	CC="zig cc -target x86_64-linux-musl" CGO_ENABLED=1 $(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="-X 'github.com/bitomia/realm/common/config.BuildGitCommit=$(GIT_COMMIT)-ee' -X 'github.com/bitomia/realm/common/config.Version=$(GIT_TAG)' -extldflags '-L$(NETPLANE_STATIC_DIR) -lm -lunwind'" -tags=EE

.PHONY: netplane
netplane:
	@mkdir -p $(NETPLANE_STATIC_DIR)
	@cp -f $(NETPLANE_LIB_DIR)/libnetplane_client.a $(NETPLANE_STATIC_DIR)/libnetplane_client.a

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
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run cmd/...

.PHONY: verify-lint-agent
verify-lint-agent:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run agent/...

lint: verify-lint-cmd verify-lint-agent

.PHONY: verify-fmt
verify-fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test -v ./drivers/...
	$(GO) test -v ./agent/db/...
	$(GO) test -v ./agent/containers/...
	$(GO) test -v ./agent/artifacts/...
