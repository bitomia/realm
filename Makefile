MAKEFLAGS += --no-print-directory

.PHONY: all

ROOT:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(ROOT)/bin
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "Unknown Version")
COMMIT_FLAG := -X 'github.com/bitomia/realm/internal/config.BuildGitCommit=$(GIT_COMMIT)'

GO_BUILD := go build -mod=readonly -buildvcs=false

REALM_OUT := $(BIN_DIR)/realm

all:
	@echo "Building ($(GIT_COMMIT))..."
	@go mod tidy
	@go build -C ./cmd -o $(REALM_OUT) -mod=readonly -buildvcs=false -ldflags="$(COMMIT_FLAG)"

clean:
	@echo "Cleaning..."
	@rm -f $(REALM_OUT)
