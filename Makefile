# ─────────────────────────────────────────────────────────────────────────────
# Makefile — common developer tasks for students-api
#
# Usage:
#   make <target>
#
# Examples:
#   make run       → start the dev server
#   make build     → compile a binary into ./out/
#   make test      → run all tests
#   make tidy      → clean up go.mod and go.sum
# ─────────────────────────────────────────────────────────────────────────────

# Name of the compiled binary
BINARY_NAME = students-api

# Where the binary will be placed
OUT_DIR = out

# Path to the config file used when running locally
CONFIG = config/local.yaml

# The main package to build / run
MAIN = ./cmd/students-api

# Go build flags:
#   CGO_ENABLED=1  required for the go-sqlite3 driver (it uses C code)
export CGO_ENABLED=1

.PHONY: all run build clean test tidy deps storage help

## all: default target — build the binary
all: build

## deps: download all Go module dependencies
deps:
	go mod download
	go mod verify

## tidy: remove unused dependencies and update go.sum
tidy:
	go mod tidy

## storage: create the storage directory (SQLite db file goes here)
storage:
	mkdir -p storage

## run: start the development server (requires config to exist)
run: storage
	go run $(MAIN) --config=$(CONFIG)

## build: compile a production binary into ./out/
build: storage
	mkdir -p $(OUT_DIR)
	go build -o $(OUT_DIR)/$(BINARY_NAME) $(MAIN)
	@echo "Binary built: $(OUT_DIR)/$(BINARY_NAME)"

## run-binary: run the compiled binary (must `make build` first)
run-binary: storage
	./$(OUT_DIR)/$(BINARY_NAME) --config=$(CONFIG)

## test: run all tests with verbose output and race detector
test:
	go test -v -race ./...

## vet: run Go's static analyser to catch common mistakes
vet:
	go vet ./...

## clean: remove compiled binaries and the database file
clean:
	rm -rf $(OUT_DIR)
	rm -f storage/storage.db
	@echo "Cleaned build artifacts and database"

## help: list all available make targets
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
