# Mico

> **Mi**grate **Co**ntainers —— Docker 容器跨服务器无痛迁移工具。

Mico 是一个 CLI 工具，在源服务器上将所有运行中的 Docker 容器（镜像、配置、数据卷、网络）打包为一个压缩文件，在目标服务器上一条命令即可完整恢复。

## 特性

- **完整快照** — 捕获镜像、容器配置、命名卷、绑定挂载和网络
- **增量模式** — 仅打包上次运行后有变更的容器
- **选择性打包** — 按容器名称过滤
- **依赖感知** — 通过拓扑排序遵循 Docker Compose `depends_on` 启动顺序
- **并发操作** — 可配置并发数，加速打包/解包
- **完整性校验** — 自动生成 SHA256 校验和，解包时验证

## 安装

### Shell（Linux / macOS）

```bash
curl -fsSL https://raw.githubusercontent.com/Ray-D-Song/mico/main/install.sh | sh
```

### PowerShell（Windows）

```powershell
irm https://raw.githubusercontent.com/Ray-D-Song/mico/main/install.ps1 | iex
```

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
| `--verify` | `-v` | `false` | 解包前验证 SHA256 校验和 |
| `--force` | `-f` | `false` | 强制恢复（覆盖已有内容） |
| `--concurrent` | `-j` | `1` | 并发操作数 |

**示例：**

```bash
# 带完整性校验的解包
mico unpack --verify migration.zst

# 强制恢复，4 个并发
mico unpack -f -j 4 migration.zst
```

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
migration.zst
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
