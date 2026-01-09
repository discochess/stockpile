.PHONY: all build test lint fmt clean install bench help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Binary names
BINARY_NAME=stockpile
BENCH_BINARY_NAME=stockpile-bench

# Build directories
BUILD_DIR=./build
CMD_DIR=./cmd

all: build

## build: Build all binaries
build:
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/stockpile
	$(GOBUILD) -o $(BUILD_DIR)/$(BENCH_BINARY_NAME) $(CMD_DIR)/stockpile-bench

## install: Install binaries to GOPATH/bin
install:
	$(GOCMD) install $(CMD_DIR)/stockpile
	$(GOCMD) install $(CMD_DIR)/stockpile-bench

## test: Run all tests
test:
	$(GOTEST) -v ./...

## test-short: Run tests without verbose output
test-short:
	$(GOTEST) ./...

## test-race: Run tests with race detector
test-race:
	$(GOTEST) -race ./...

## test-cover: Run tests with coverage
test-cover:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## bench: Run benchmarks (requires DATA_DIR)
bench:
	$(GOTEST) -bench=. -benchmem ./benchmark/micro/...

## lint: Run linter
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## fmt: Format code
fmt:
	$(GOFMT) -s -w .

## fmt-check: Check code formatting
fmt-check:
	@test -z "$$($(GOFMT) -l .)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

## tidy: Tidy go.mod
tidy:
	$(GOMOD) tidy

## clean: Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## deps: Download dependencies
deps:
	$(GOMOD) download

## verify: Verify dependencies
verify:
	$(GOMOD) verify

## update: Update dependencies
update:
	$(GOGET) -u ./...
	$(GOMOD) tidy

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
