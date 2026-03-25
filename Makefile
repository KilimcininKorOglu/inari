.PHONY: build build-release test lint fmt vet clean

# Build debug binary
build:
	go build -o bin/inari ./cmd/inari/
	go build -o bin/inari-benchmark ./cmd/benchmark/

# Build release binary with version injection and optimizations
build-release:
	go build -ldflags="-s -w -X main.version=$(shell git describe --tags --always)" \
		-o bin/inari ./cmd/inari/
	go build -ldflags="-s -w -X main.version=$(shell git describe --tags --always)" \
		-o bin/inari-benchmark ./cmd/benchmark/

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run a single test by name (usage: make test-one TEST=TestSketch)
test-one:
	go test -v -run $(TEST) ./...

# Lint with go vet
vet:
	go vet ./...

# Format check
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Auto-format
fmt:
	gofmt -w .

# Clean build artifacts
clean:
	rm -rf bin/
	go clean ./...
