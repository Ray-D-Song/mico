<div align="center">
  <img src="assets/logo.svg" alt="mico" width="260" height="70">
</div>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24.5-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/github/v/release/ray-d-song/mico?label=version" alt="Version">
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat" alt="License">
</p>

> **Mi**grate **Co**ntainers —— Docker 容器跨服务器无痛迁移工具。

Mico 是一个 CLI 工具，在源服务器上将所有运行中的 Docker 容器（镜像、配置、数据卷、网络）打包为一个压缩文件，在目标服务器上一条命令即可完整恢复。

## 特性

- **完整快照** — 捕获镜像、容器配置、命名卷、绑定挂载和网络
- **增量模式** — 仅打包上次运行后有变更的容器
- **选择性打包** — 按容器名称过滤
- **依赖感知** — 通过拓扑排序遵循 Docker Compose `depends_on` 启动顺序
- **并发操作** — 可配置并发数，加速打包/解包
- **完整性校验** — 自动生成 SHA256 校验和，解包时验证
- **S3 备份/恢复** — 备份上传至 S3 兼容存储，一条命令即可恢复
- **多运行时** — 支持 Docker、OrbStack 和 Podman

> [!NOTE]
> Bind 挂载（`-v /host/path:/container/path`）在迁移后会转为命名卷，因为目标服务器上可能无法重现文件夹路径。

## 安装

### Homebrew（macOS / Linux）

```bash
brew tap ray-d-song/mico
brew install mico
```

### Shell（Linux / macOS）

```bash
curl -fsSL https://raw.githubusercontent.com/Ray-D-Song/mico/main/install.sh | sh
```

### PowerShell（Windows）

```powershell
irm https://raw.githubusercontent.com/Ray-D-Song/mico/main/install.ps1 | iex
```

> [!WARNING]
> Windows Docker (WSL2) 缺乏测试。

### Go install

```bash
go install github.com/Ray-D-Song/mico@latest
```

需要 Go 1.24+。

## 快速开始

```bash
# 在源服务器 — 打包所有容器
mico pack -o migration.zst

# 在目标服务器 — 恢复所有容器
mico unpack migration.zst

# 备份到 S3
mico backup

# 从 S3 恢复
mico unpack --s3
```

## 用法

### `mico pack`

将运行中的 Docker 容器打包为迁移压缩包。

```bash
mico pack [flags]
```

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--output` | `-o` | `mico-<timestamp>.zst` | 输出文件路径 |
| `--containers` | `-c` | *(全部)* | 逗号分隔的容器名称列表 |
| `--concurrent` | `-j` | `1` | 并发工作数 |
| `--incremental` | | `false` | 仅打包上次运行后有变更的容器 |

**示例：**

```bash
# 打包所有运行中的容器
mico pack

# 打包指定容器，4 个并发
mico pack -c web,db,redis -j 4

# 增量打包（仅打包变更的容器）
mico pack --incremental

# 自定义输出路径
mico pack -o production-backup.zst
```

### `mico unpack`

从迁移压缩包恢复容器。

```bash
mico unpack [flags] <archive>
```

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--s3` | | `false` | 从 S3 恢复，而非本地文件 |
| `--bucket` | `-b` | *(来自 ~/.mico/s3.ini)* | S3 桶名称 |
| `--key` | `-k` | *(最新)* | 指定要恢复的备份 key |
| `--list` | `-l` | `false` | 列出 S3 中所有备份 |
| `--no-verify` | | `false` | 跳过校验和验证 |
| `--force` | `-f` | `false` | 强制恢复（覆盖已有内容） |
| `--concurrent` | `-j` | `1` | 并发操作数 |

**示例：**

```bash
# 从本地文件恢复
mico unpack migration.zst

# 从 S3 恢复最新备份
mico unpack --s3

# 列出 S3 中所有备份
mico unpack --s3 --list

# 从 S3 恢复指定备份
mico unpack --s3 -k backup-2026/05/13/150405/s3_temp.zst

# 从 S3 恢复，跳过校验
mico unpack --s3 --no-verify

# 强制恢复，4 个并发
mico unpack -f -j 4 migration.zst
```

### `mico backup`

打包容器并上传至 S3 兼容存储，支持周期性备份。

```bash
mico backup [flags]
```

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--bucket` | `-b` | *(来自 ~/.mico/s3.ini)* | S3 桶名称 |
| `--containers` | `-c` | *(全部)* | 逗号分隔的容器名称 |
| `--interval` | `-i` | `0` | 备份间隔（小时，0 = 仅运行一次） |
| `--retention` | `-r` | `0` | 保留的备份数量（0 = 保留全部） |
| `--concurrent` | `-j` | `1` | 并发操作数 |

**示例：**

```bash
# 一次性备份
mico backup

# 仅备份指定容器
mico backup -c web,db

# 自定义桶，每 6 小时备份一次，保留最近 28 个
mico backup -b my-bucket -i 6 -r 28
```

### S3 配置

在 `~/.mico/s3.ini` 中放置配置文件。兼容所有 S3 兼容对象存储（AWS S3、RustFS、MinIO 等）。

以 RustFS 为例：

```ini
[default]
region = us-east-1
endpoint = http://192.168.1.100:9000
bucket = my-backups
use_path_style = true
aws_access_key_id = your-access-key
aws_secret_access_key = your-secret-key
```

支持的选项：

| 配置项 | 必填 | 说明 |
|--------|------|------|
| `endpoint` | 否 | S3 兼容 API 端点（不填则用 AWS 默认端点） |
| `bucket` | 否 | 默认桶名称（可被 `--bucket` 覆盖） |
| `use_path_style` | 否 | MinIO / RustFS / 自建 S3 需设为 `true` |
| `region` | 否 | AWS 区域 |
| `aws_access_key_id` | 否 | 访问密钥 |
| `aws_secret_access_key` | 否 | 私有密钥 |

所有 SDK 原生选项（`region`、`aws_access_key_id`、`aws_secret_access_key` 等）通过标准 AWS 凭证链加载。

> 如果未配置 `~/.mico/s3.ini`，程序会回退到默认 AWS 凭证链（环境变量、`~/.aws/credentials`、IAM 实例配置等）。

## 工作原理

### Pack 流程

1. 扫描所有运行中的容器
2. 从容器标签中分析 Docker Compose `depends_on` 依赖关系
3. 拓扑排序确定启动顺序
4. 保存镜像（`docker save`）、导出配置、备份数据卷和绑定挂载
5. 将所有内容打包为 `.zst`（Zstandard 压缩的 tar）归档
6. 生成 SHA256 校验和

### Unpack 流程

1. 验证 SHA256 校验和（可选）
2. 解压并提取归档文件
3. 检测端口冲突
4. 加载 Docker 镜像（`docker load`）
5. 恢复命名卷和绑定挂载
6. 重建 Docker 网络
7. 按依赖顺序重建并启动容器

### 归档结构

```
migration.zst             # 压缩归档文件
migration.zst.sha256      # SHA256 校验和附属文件（与归档一起生成）

归档内容：
├── manifest.json          # 包元数据和服务定义
└── services/
    └── <service-name>/
        ├── config.json    # 容器 inspect 配置
        ├── image.tar      # Docker 镜像导出文件
        ├── hostconfig.json
        ├── networks.json
        └── mounts.json
```

## 开发

```bash
# 安装依赖
make deps

# 编译
make build

# 运行测试
make test

# 预览（编译 + 打包）
make preview

# 清理构建产物
make clean
```

## 许可证

MIT
