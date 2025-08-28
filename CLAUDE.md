# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Mico is a Docker container migration tool written in Go. It allows seamless migration of Docker container services between servers using `mico pack` (to create migration packages) and `mico unpack` (to restore services on target servers).

## Common Commands

### Build
```bash
go build -v
```

### Test
```bash
go test ./...
```
Note: Tests currently fail due to empty implementation files in cmd/ directory.

### Run
```bash
go run main.go
```

## Architecture

This is a Go CLI application using the Cobra framework with the following structure:

### Core Components
- `main.go` - Entry point with basic "mico" output
- `cmd/` - Cobra command implementations (currently empty files)
  - `root.go` - CLI root command
  - `pack.go` - Container packing functionality  
  - `unpack.go` - Container unpacking functionality
- `pkg/` - Core business logic packages
  - `docker/` - Docker API operations (containers, images, networks)
  - `archive/` - Compression/decompression using zstd
  - `volume/` - Data volume backup and restore
  - `config/` - Configuration management and manifest handling

### Key Dependencies
- Docker Engine API for container operations
- Cobra CLI framework for command structure
- zstd compression (klauspost/compress) for efficient archiving
- Various Docker SDK packages for container management

## Implementation Status

This project is in early planning/setup phase:
- Basic project structure exists
- Go module configured with dependencies
- Implementation files are empty (cmd/*.go files have 0 lines)
- Only main.go contains minimal implementation

## Development Notes

The project follows a modular architecture separating CLI commands from core business logic. The planned implementation will handle complex Docker operations including container discovery, image export, volume backup, network configuration, and dependency management for container migration scenarios.