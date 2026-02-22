.PHONY: lint build test

# Run golangci-lint
lint:
	golangci-lint run ./...

# Build all packages
build:
	go build -v ./...

# Run tests with race detection and coverage
test:
	go test -v -race -coverprofile=coverage.out ./...
