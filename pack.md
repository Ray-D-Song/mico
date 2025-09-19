# Pack 命令设计

## 执行流程

1. **环境检查**: 检查能否创建 Docker client
2. **容器扫描**: scan 检查正在运行的容器
3. **依赖分析**: dep 生成依赖关系，将有依赖关系的"块"抽象为 Service
4. **服务持久化**: Service 对象持久化到压缩包中，用于 unpack 时的恢复
5. **流式打包**: 创建 zstd 流和 tar 流进行流式处理
6. **校验和生成**: 打包完成后自动生成 SHA256 校验和文件

```go
type Service struct {
    id           int                    `json:"id"`
    Containers   []container.Summary     `json:"containers"`
    Images       []string                `json:"images"`        // 需要的镜像列表
    Volumes      []VolumeInfo           `json:"volumes"`       // 数据卷信息
    Networks     []NetworkInfo          `json:"networks"`      // 网络配置
    StartOrder   int                    `json:"start_order"`   // 启动顺序
}

type PackageManifest struct {
    Version     string    `json:"version"` // mico 的版本
    CreatedAt   time.Time `json:"created_at"`
    Source      string    `json:"source"`        // 源服务器信息
    Services    []Service `json:"services"`
    TotalSize   int64     `json:"total_size"`
}
```

## 包文件结构

```bash
# 主要文件
migration.zst           # 主迁移包（zstd压缩的tar文件）
migration.zst.sha256    # SHA256校验和文件

# 包内结构
migration.zst 内容:
├── manifest.json          # PackageManifest
├── services/
│   ├── web-service/
│   │   ├── containers.json
│   │   ├── images/
│   │   │   └── nginx.tar
│   │   └── volumes/
│   │       └── web-data.tar
│   └── db-service/
│       ├── containers.json
│       ├── images/
│       │   └── postgres.tar
│       └── volumes/
│           └── db-data.tar
```

## 校验和验证

### 实现方式
- **整包校验**: 对完整的 `migration.zst` 文件生成 SHA256 校验和
- **外部文件**: 校验和保存在 `migration.zst.sha256` 文件中
- **标准格式**: 兼容 `sha256sum` 工具的格式

### 文件内容示例
```bash
# migration.zst.sha256 文件内容
a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456  migration.zst
```

### 验证方式
```bash
# 手动验证
sha256sum -c migration.zst.sha256

# 程序验证
mico unpack migration.zst --verify
```