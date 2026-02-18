.PHONY: build test clean install lint

# Build variables
BINARY_NAME=gcq
DAEMON_NAME=gcqd
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
GO=go
OUTPUT_DIR=bin

# Default target
all: build

# Build both binaries
build:
	@mkdir -p $(OUTPUT_DIR)
	${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${BINARY_NAME} ./cmd/gcq
	${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${DAEMON_NAME} ./cmd/gcqd

# Build for all platforms
# Note: ARM64 builds require native compilation due to go-tree-sitter C bindings
# Only amd64 cross-platform builds are supported
build-all:
	@mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 ${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${BINARY_NAME}-linux-amd64 ./cmd/gcq
	GOOS=linux GOARCH=amd64 ${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${DAEMON_NAME}-linux-amd64 ./cmd/gcqd
	GOOS=darwin GOARCH=amd64 ${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${BINARY_NAME}-darwin-amd64 ./cmd/gcq
	GOOS=darwin GOARCH=amd64 ${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${DAEMON_NAME}-darwin-amd64 ./cmd/gcqd
	GOOS=windows GOARCH=amd64 ${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${BINARY_NAME}-windows-amd64.exe ./cmd/gcq
	GOOS=windows GOARCH=amd64 ${GO} build ${LDFLAGS} -o $(OUTPUT_DIR)/${DAEMON_NAME}-windows-amd64.exe ./cmd/gcqd

# Run tests
test:
	${GO} test -v -race -coverprofile=coverage.out ./...

# Run tests without coverage
test-no-cov:
	${GO} test -v -race ./...

# Clean build artifacts
clean:
	rm -rf $(OUTPUT_DIR)/
	rm -f coverage.out

# Run linter
lint:
	${GO} vet ./...
	golangci-lint run ./...

# Format code
fmt:
	${GO} fmt ./...

# Run the application
run:
	${GO} run ./cmd/gcq

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build both binaries (gcq, gcqd)"
	@echo "  build-all     - Build for all platforms"
	@echo "  test          - Run tests with coverage"
	@echo "  test-no-cov   - Run tests without coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  lint          - Run linters"
	@echo "  install       - Install Go dependencies"
	@echo "  install-bin   - Install binaries to GOPATH/bin"
	@echo "  fmt           - Format code"
	@echo "  run           - Run the application"
