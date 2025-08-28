.PHONY: build test preview clean

# Build the binary
build:
	go build -v -o mico

# Run unit tests
test:
	go test -v ./...

# Preview/dry run - build and run with sample parameters
preview: build
	./mico pack --help
	./mico pack -o sample.zst -c web,db,redis

# Clean build artifacts
clean:
	rm -f mico *.zst

# Install dependencies
deps:
	go mod tidy
	go mod download