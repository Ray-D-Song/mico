<div align="center">
  <img src="assets/logo.svg" alt="mico" width="260" height="70">
</div>

![Go](https://img.shields.io/badge/Go-1.24.5-00ADD8?style=flat&logo=go)
![Version](https://img.shields.io/github/v/release/ray-d-song/mico?label=version)
![License](https://img.shields.io/badge/license-MIT-blue?style=flat)

> **Mi**grate **Co**ntainers — painless Docker container migration between servers.

[中文文档](README.zh.md)

Mico is a CLI tool that packs all running Docker containers (images, configs, volumes, networks) into a single compressed archive on the source server, and restores everything on the target server with a single command.

## Features

- **Full snapshot** — captures images, container configs, named volumes, bind mounts, and networks
- **Incremental mode** — only pack containers changed since last run
- **Selective packing** — filter by container name
- **Dependency-aware** — respects Docker Compose `depends_on` ordering via topological sort
- **Concurrent operations** — configurable worker count for faster packing/unpacking
- **Integrity verification** — SHA256 checksums auto-generated and verified on unpack
- **S3 backup/restore** — upload backups to S3-compatible storage, restore with a single command
- **Multi-runtime** — supports Docker, OrbStack and Podman

> [!NOTE]
> Bind mounts (`-v /host/path:/container/path`) are converted to named volumes after migration, since the target server may not replicate the same directory paths.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap ray-d-song/mico
brew install mico
```

### Shell (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Ray-D-Song/mico/main/install.sh | sh
```

### PowerShell (Windows)

```powershell
irm https://raw.githubusercontent.com/Ray-D-Song/mico/main/install.ps1 | iex
```

> [!WARNING]
> Windows Docker (WSL2) has not been tested.

### Go install

```bash
go install github.com/Ray-D-Song/mico@latest
```

Requires Go 1.24+.

## Quick Start

```bash
# On the source server — pack everything
mico pack -o migration.zst

# On the target server — restore everything
mico unpack migration.zst

# Backup to S3
mico backup

# Restore from S3
mico unpack --s3
```

## Usage

### `mico pack`

Pack running Docker containers into a migration archive.

```bash
mico pack [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `mico-<timestamp>.zst` | Output archive path |
| `--containers` | `-c` | *(all)* | Comma-separated container names |
| `--concurrent` | `-j` | `1` | Number of concurrent workers |
| `--incremental` | | `false` | Only pack containers changed since last run |

**Examples:**

```bash
# Pack all running containers
mico pack

# Pack specific containers with 4 workers
mico pack -c web,db,redis -j 4

# Incremental pack (only changes since last run)
mico pack --incremental

# Custom output path
mico pack -o production-backup.zst
```

### `mico unpack`

Restore containers from a migration archive.

```bash
mico unpack [flags] <archive>
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--s3` | | `false` | Restore from S3 instead of local file |
| `--bucket` | `-b` | *(from ~/.mico/s3.ini)* | S3 bucket name |
| `--key` | `-k` | *(latest)* | Specific backup key to restore |
| `--list` | `-l` | `false` | List available backups in S3 |
| `--no-verify` | | `false` | Skip checksum verification |
| `--force` | `-f` | `false` | Force restore (overwrite existing) |
| `--concurrent` | `-j` | `1` | Number of concurrent operations |

**Examples:**

```bash
# Unpack local archive
mico unpack migration.zst

# Restore latest backup from S3
mico unpack --s3

# List all backups in S3
mico unpack --s3 --list

# Restore specific backup from S3
mico unpack --s3 -k backup-2026/05/13/150405/s3_temp.zst

# Restore from S3 without checksum verification
mico unpack --s3 --no-verify

# Force restore with 4 concurrent workers
mico unpack -f -j 4 migration.zst
```

### `mico backup`

Pack containers and upload the archive to S3-compatible storage. Supports periodic backups.

```bash
mico backup [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--bucket` | `-b` | *(from ~/.mico/s3.ini)* | S3 bucket name |
| `--containers` | `-c` | *(all)* | Comma-separated container names |
| `--interval` | `-i` | `0` | Backup interval in hours (0 = run once) |
| `--retention` | `-r` | `0` | Number of backups to keep (0 = keep all) |
| `--concurrent` | `-j` | `1` | Number of concurrent operations |

**Examples:**

```bash
# One-time backup
mico backup

# Specific containers only
mico backup -c web,db

# Custom bucket, periodic every 6 hours, keep last 28
mico backup -b my-bucket -i 6 -r 28
```

### S3 Configuration

Place a config file at `~/.mico/s3.ini`. Compatible with any S3-compatible object storage (AWS S3, RustFS, MinIO, etc.).

Example for RustFS:

```ini
[default]
region = us-east-1
endpoint = http://192.168.1.100:9000
bucket = my-backups
use_path_style = true
aws_access_key_id = your-access-key
aws_secret_access_key = your-secret-key
```

Supported options:

| Key | Required | Description |
|-----|----------|-------------|
| `endpoint` | No | S3-compatible API endpoint (omit for AWS default) |
| `bucket` | No | Default bucket name (can be overridden by `--bucket`) |
| `use_path_style` | No | Set `true` for MinIO / RustFS / self-hosted S3 |
| `region` | No | AWS region |
| `aws_access_key_id` | No | Access key |
| `aws_secret_access_key` | No | Secret key |

All SDK-native options (`region`, `aws_access_key_id`, `aws_secret_access_key`, etc.) are loaded via the standard AWS credential chain.

> If `~/.mico/s3.ini` is not present, the program falls back to the default AWS credential chain (environment variables, `~/.aws/credentials`, IAM instance profiles, etc.).

## How It Works

### Pack flow

1. Scan all running containers (or filtered by `--containers`)
2. Analyze Docker Compose `depends_on` from container labels
3. Topological sort to determine start order
4. Save images (`docker save`), inspect configs, backup volumes & bind mounts
5. Bundle everything into a `.zst` (Zstandard-compressed tar) archive
6. Generate SHA256 checksum

### Unpack flow

1. Verify SHA256 checksum (optional)
2. Decompress and extract the archive
3. Check for port conflicts
4. Load Docker images (`docker load`)
5. Restore named volumes and bind mounts
6. Recreate Docker networks
7. Recreate and start containers in dependency order

### Archive structure

```
migration.zst             # Compressed archive
migration.zst.sha256      # SHA256 checksum sidecar (generated alongside archive)

Archive contents:
├── manifest.json          # Package metadata & service definitions
└── services/
    └── <service-name>/
        ├── config.json    # Container inspect JSON
        ├── image.tar      # Docker image save output
        ├── hostconfig.json
        ├── networks.json
        └── mounts.json
```

## Requirements

- Docker Engine 20.10+ on both source and target servers
- Sufficient disk space for the archive (~ sum of all image sizes)
- `docker save` / `docker load` availability

## Development

```bash
# Install dependencies
make deps

# Build
make build

# Run tests
make test

# Preview (build + pack)
make preview

# Clean build artifacts
make clean
```

## License

MIT
