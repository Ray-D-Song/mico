.PHONY: build test preview clean

# Build the binary
build:
	go build -v -o mico

# Run unit tests
test:
	go test -v ./...

# Preview/dry run - build and run with sample parameters
preview: build
	./mico pack

# Clean build artifacts
clean:
	rm -f mico *.zst

# Install dependencies
deps:
	go mod tidy
	go mod download