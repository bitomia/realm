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
	REALM_LIB := $(BIN_DIR)$(SEP)realm.lib
	REALM_SHARED_LIB := $(BIN_DIR)$(SEP)librealm.dll
	REALM_IMPORT_LIB := $(BIN_DIR)$(SEP)librealm.lib
else
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
	RM = rm -rf
	MKDIR = mkdir -p $(1)
	SEP = /
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm
	REALM_LIB := $(BIN_DIR)$(SEP)realm.a
	REALM_SHARED_LIB := $(BIN_DIR)$(SEP)librealm.so
endif

REALM_SHARED_HEADER := $(BIN_DIR)$(SEP)librealm.h
REALM_HEADER := $(BIN_DIR)$(SEP)realm.h


COMMIT_FLAG := -X 'github.com/bitomia/realm/internal/config.BuildGitCommit=$(GIT_COMMIT)'
MAKEFLAGS += --no-print-directory

# Installation directories
PREFIX ?= /usr/local
INSTALL_LIB_DIR := $(PREFIX)/lib
INSTALL_INCLUDE_DIR := $(PREFIX)/include
INSTALL_CMAKE_DIR := $(PREFIX)/lib/cmake/Realm

.PHONY: all
all:
	@echo "Building ($(GIT_COMMIT))..."
	$(GO) mod tidy
	$(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="$(COMMIT_FLAG)"

.PHONY: shared
lib: export CGO_ENABLED=1
lib: $(BIN_DIR)
	@echo "Building C shared library..."
	$(GO) build -o $(REALM_SHARED_LIB) -buildmode=c-shared lib/main.go
	@mv $(REALM_SHARED_HEADER) $(REALM_HEADER)
	@echo "Namespacing extern functions..."
	@sed -i '/extern "C" {/i namespace realm {' $(REALM_HEADER)
	@sed -i '/^#ifdef __cplusplus$$/{ N; /\n}$$/{s/}/}\n} \/\/ namespace realm/;} }' $(REALM_HEADER)
ifeq ($(OS),Windows_NT)
	@echo "Generating import library..."
	@cd $(BIN_DIR) && gendef librealm.dll
	@cd $(BIN_DIR) && dlltool -d librealm.def -l librealm.lib -D librealm.dll
	@del "$(BIN_DIR)$(SEP)librealm.def"
endif
	@echo "Generated: $(REALM_SHARED_LIB)"
	@echo "Generated: $(REALM_HEADER)"
ifeq ($(OS),Windows_NT)
	@echo "Generated: $(REALM_IMPORT_LIB)"
endif

.PHONY: install
install: lib
	@echo "Installing Realm library to $(PREFIX)..."
	@install -d $(INSTALL_LIB_DIR)
	@install -d $(INSTALL_INCLUDE_DIR)
	@install -d $(INSTALL_CMAKE_DIR)
	@install -m 755 $(REALM_LIB) $(INSTALL_LIB_DIR)/
	@install -m 755 $(REALM_SHARED_LIB) $(INSTALL_LIB_DIR)/
ifeq ($(OS),Windows_NT)
	@install -m 755 $(REALM_IMPORT_LIB) $(INSTALL_LIB_DIR)/
endif
	@install -m 644 $(REALM_HEADER) $(INSTALL_INCLUDE_DIR)/realm.h
	@install -m 644 cmake/RealmConfig.cmake $(INSTALL_CMAKE_DIR)/
	@install -m 644 cmake/RealmConfigVersion.cmake $(INSTALL_CMAKE_DIR)/
	@echo "Installation complete!"
	@echo "  Library: $(INSTALL_LIB_DIR)/$(notdir $(REALM_LIB))"
	@echo "  Library: $(INSTALL_LIB_DIR)/$(notdir $(REALM_SHARED_LIB))"
ifeq ($(OS),Windows_NT)
	@echo "  Import Library: $(INSTALL_LIB_DIR)/$(notdir $(REALM_IMPORT_LIB))"
endif
	@echo "  Header: $(INSTALL_INCLUDE_DIR)/realm.h"
	@echo "  CMake config: $(INSTALL_CMAKE_DIR)/"

$(BIN_DIR):
	$(call MKDIR,$(BIN_DIR))

.PHONY: clean
clean:
	-$(RM) "$(REALM_OUT)"
	-$(RM) "$(REALM_LIB)"
	-$(RM) "$(REALM_HEADER)"
	-$(RM) "$(REALM_SHARED_LIB)"
ifeq ($(OS),Windows_NT)
	-$(RM) "$(REALM_IMPORT_LIB)"
endif

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
