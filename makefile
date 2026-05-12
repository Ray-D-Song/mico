.PHONY: build test preview clean e2e

# Build the binary
build:
	go build -o mico .

# Run unit tests
test:
	go test -v ./...

# Run end-to-end tests
e2e:
	./tests/pack-and-unpack.sh

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