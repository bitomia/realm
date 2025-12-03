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

	# Daemon library outputs
	DAEMON_SHARED_HEADER := $(BIN_DIR)$(SEP)realm-daemon.h
	DAEMON_SHARED_LIB := $(BIN_DIR)$(SEP)realm-daemon.dll
	DAEMON_IMPORT_DEF := $(BIN_DIR)$(SEP)realm-daemon.def
	DAEMON_IMPORT_LIB := $(BIN_DIR)$(SEP)realm-daemon.lib

	# Client library outputs
	CLIENT_SHARED_HEADER := $(BIN_DIR)$(SEP)realm-client.h
	CLIENT_SHARED_LIB := $(BIN_DIR)$(SEP)realm-client.dll
	CLIENT_IMPORT_DEF := $(BIN_DIR)$(SEP)realm-client.def
	CLIENT_IMPORT_LIB := $(BIN_DIR)$(SEP)realm-client.lib
else
	GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
	RM = rm -rf
	MKDIR = mkdir -p $(1)
	SEP = /
	BIN_DIR := $(ROOT)$(SEP)bin
	REALM_OUT := $(BIN_DIR)$(SEP)realm

	# Daemon library outputs
	DAEMON_SHARED_HEADER := $(BIN_DIR)$(SEP)librealm-daemon.h
	DAEMON_SHARED_LIB := $(BIN_DIR)$(SEP)librealm-daemon.so

	# Client library outputs
	CLIENT_SHARED_HEADER := $(BIN_DIR)$(SEP)librealm-client.h
	CLIENT_SHARED_LIB := $(BIN_DIR)$(SEP)librealm-client.so
endif


COMMIT_FLAG := -X 'github.com/bitomia/realm/common/config.BuildGitCommit=$(GIT_COMMIT)'
MAKEFLAGS += --no-print-directory

# Installation directories
PREFIX ?= /usr/local
INSTALL_LIB_DIR := $(PREFIX)/lib
INSTALL_INCLUDE_DIR := $(PREFIX)/include
INSTALL_CMAKE_DIR := $(PREFIX)/lib/cmake/Realm

.PHONY: all
all:
	@echo "Building ($(GIT_COMMIT))..."
	$(GO) build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="$(COMMIT_FLAG)"

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: lib
lib: lib-client lib-daemon

.PHONY: lib-daemon
lib-daemon: export CGO_ENABLED=1
lib-daemon: $(BIN_DIR)
	@echo "Building C shared library for daemon..."
	$(GO) build -o $(DAEMON_SHARED_LIB) -buildmode=c-shared -buildvcs=false -ldflags="$(COMMIT_FLAG)" ./clib/daemon
ifneq ($(OS),Windows_NT)
	@mv $(DAEMON_SHARED_HEADER) $(BIN_DIR)$(SEP)realm-daemon.h
endif
	@echo "Namespacing extern functions..."
	@sed -i '/extern "C" {/i namespace realm::daemon {' $(BIN_DIR)$(SEP)realm-daemon.h
	@sed -i '/^#ifdef __cplusplus$$/{ N; /\n}$$/{s/}/}\n} \/\/ namespace realm::daemon/;} }' $(BIN_DIR)$(SEP)realm-daemon.h
	@echo "Adding _CRT_USE_C_COMPLEX_H define..."
	@sed -i '/#include <complex.h>/i #define _CRT_USE_C_COMPLEX_H' $(BIN_DIR)$(SEP)realm-daemon.h
ifeq ($(OS),Windows_NT)
	@echo "Generating import library..."
	@cd $(BIN_DIR) && gendef $(DAEMON_SHARED_LIB)
	@cd $(BIN_DIR) && dlltool -d $(DAEMON_IMPORT_DEF) -l $(DAEMON_IMPORT_LIB) -D $(DAEMON_SHARED_LIB)
	@$(RM) "$(DAEMON_IMPORT_DEF)"
endif
	@echo "Generated: $(DAEMON_SHARED_LIB)"
	@echo "Generated: $(BIN_DIR)$(SEP)realm-daemon.h"
ifeq ($(OS),Windows_NT)
	@echo "Generated: $(DAEMON_IMPORT_LIB)"
endif

.PHONY: lib-client
lib-client: export CGO_ENABLED=1
lib-client: $(BIN_DIR)
	@echo "Building C shared library for client..."
	$(GO) build -o $(CLIENT_SHARED_LIB) -buildmode=c-shared -buildvcs=false -ldflags="$(COMMIT_FLAG)" ./clib/client
ifneq ($(OS),Windows_NT)
	@mv $(CLIENT_SHARED_HEADER) $(BIN_DIR)$(SEP)realm-client.h
endif
	@echo "Namespacing extern functions..."
	@sed -i '/extern "C" {/i namespace realm::client {' $(BIN_DIR)$(SEP)realm-client.h
	@sed -i '/^#ifdef __cplusplus$$/{ N; /\n}$$/{s/}/}\n} \/\/ namespace realm::client/;} }' $(BIN_DIR)$(SEP)realm-client.h
	@echo "Adding _CRT_USE_C_COMPLEX_H define..."
	@sed -i '/#include <complex.h>/i #define _CRT_USE_C_COMPLEX_H' $(BIN_DIR)$(SEP)realm-client.h
ifeq ($(OS),Windows_NT)
	@echo "Generating import library..."
	@cd $(BIN_DIR) && gendef $(CLIENT_SHARED_LIB)
	@cd $(BIN_DIR) && dlltool -d $(CLIENT_IMPORT_DEF) -l $(CLIENT_IMPORT_LIB) -D $(CLIENT_SHARED_LIB)
	@$(RM) "$(CLIENT_IMPORT_DEF)"
endif
	@echo "Generated: $(CLIENT_SHARED_LIB)"
	@echo "Generated: $(BIN_DIR)$(SEP)realm-client.h"
ifeq ($(OS),Windows_NT)
	@echo "Generated: $(CLIENT_IMPORT_LIB)"
endif

.PHONY: install
install: lib
	@echo "Installing Realm libraries to $(PREFIX)..."
	@install -d $(INSTALL_LIB_DIR)
	@install -d $(INSTALL_INCLUDE_DIR)
	@install -d $(INSTALL_CMAKE_DIR)
	@install -m 755 $(DAEMON_SHARED_LIB) $(INSTALL_LIB_DIR)/
	@install -m 755 $(CLIENT_SHARED_LIB) $(INSTALL_LIB_DIR)/
ifeq ($(OS),Windows_NT)
	@install -m 755 $(DAEMON_IMPORT_LIB) $(INSTALL_LIB_DIR)/
	@install -m 755 $(CLIENT_IMPORT_LIB) $(INSTALL_LIB_DIR)/
endif
	@install -m 644 $(BIN_DIR)$(SEP)realm-daemon.h $(INSTALL_INCLUDE_DIR)/realm-daemon.h
	@install -m 644 $(BIN_DIR)$(SEP)realm-client.h $(INSTALL_INCLUDE_DIR)/realm-client.h
	@install -m 644 clib/cmake/RealmConfig.cmake $(INSTALL_CMAKE_DIR)/
	@install -m 644 clib/cmake/RealmConfigVersion.cmake $(INSTALL_CMAKE_DIR)/
	@echo "Installation complete!"
	@echo "  Daemon library: $(INSTALL_LIB_DIR)/$(notdir $(DAEMON_SHARED_LIB))"
	@echo "  Client library: $(INSTALL_LIB_DIR)/$(notdir $(CLIENT_SHARED_LIB))"
ifeq ($(OS),Windows_NT)
	@echo "  Daemon import library: $(INSTALL_LIB_DIR)/$(notdir $(DAEMON_IMPORT_LIB))"
	@echo "  Client import library: $(INSTALL_LIB_DIR)/$(notdir $(CLIENT_IMPORT_LIB))"
endif
	@echo "  Daemon header: $(INSTALL_INCLUDE_DIR)/realm-daemon.h"
	@echo "  Client header: $(INSTALL_INCLUDE_DIR)/realm-client.h"
	@echo "  CMake config: $(INSTALL_CMAKE_DIR)/"

$(BIN_DIR):
	$(call MKDIR,$(BIN_DIR))

.PHONY: clean
clean:
	-$(RM) "$(REALM_OUT)"
	-$(RM) "$(BIN_DIR)$(SEP)realm-daemon.h"
	-$(RM) "$(BIN_DIR)$(SEP)realm-client.h"
	-$(RM) "$(DAEMON_SHARED_LIB)"
	-$(RM) "$(CLIENT_SHARED_LIB)"
ifeq ($(OS),Windows_NT)
	-$(RM) "$(DAEMON_IMPORT_LIB)"
	-$(RM) "$(CLIENT_IMPORT_LIB)"
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

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test -v ./drivers/...
	$(GO) test -v ./daemon/db/...

.PHONY: doc
doc: lib-daemon
	@echo "Generating doxygen documentation..."
	@cd lib && doxygen Doxyfile
	@echo "Documentation generated in docs/html/"
