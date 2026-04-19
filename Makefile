BINARY_NAME=pushsite
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOPATH  ?= $(shell go env GOPATH)

LDFLAGS = -s -w \
	-X github.com/anuragvishwa/pushsite/cmd.Version=$(VERSION) \
	-X github.com/anuragvishwa/pushsite/cmd.GitCommit=$(COMMIT) \
	-X 'github.com/anuragvishwa/pushsite/cmd.BuildTime=$(DATE)'

.PHONY: all build install uninstall clean test vet lint run help cross

## help: Show this help message
help:
	@echo "Pushsite — Deploy frontend apps to EC2"
	@echo ""
	@echo "Usage:"
	@echo "  make build      Build the binary"
	@echo "  make install    Build and install to /usr/local/bin"
	@echo "  make uninstall  Remove from /usr/local/bin"
	@echo "  make test       Run all tests"
	@echo "  make vet        Run go vet"
	@echo "  make clean      Remove build artifacts"
	@echo "  make cross      Cross-compile for linux/mac/windows"
	@echo "  make run        Build and run with --help"
	@echo ""

## all: Build and test
all: test build

## build: Build the binary for current OS/arch
build:
	@echo "→ Building $(BINARY_NAME) $(VERSION)..."
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "✓ Built ./$(BINARY_NAME)"

## install: Install to /usr/local/bin (requires sudo)
install: build
	@echo "→ Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Installed! Run 'pushsite --help' to get started."

## uninstall: Remove from /usr/local/bin
uninstall:
	@echo "→ Removing $(BINARY_NAME) from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Uninstalled."

## test: Run all tests
test:
	@echo "→ Running tests..."
	CGO_ENABLED=0 go test ./... -count=1
	@echo "✓ All tests passed."

## vet: Run go vet
vet:
	@echo "→ Running go vet..."
	go vet ./...
	@echo "✓ Vet passed."

## clean: Remove build artifacts
clean:
	@echo "→ Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -rf dist/
	@echo "✓ Clean."

## run: Build and run --help
run: build
	./$(BINARY_NAME) --help

## cross: Cross-compile for all platforms
cross:
	@echo "→ Cross-compiling $(VERSION)..."
	@mkdir -p dist
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-arm64 .
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "✓ Built binaries in dist/"
	@ls -lh dist/
