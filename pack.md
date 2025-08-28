pack 命令流程：
1. 检查能否创建 Docker client
2. scan 检查正在运行的容器
3. dep 生成依赖关系，将有依赖关系的“块”抽象为 Service，同时这个对象也需要持久化到压缩包中，用于 unpack 时的恢复
4. 创建 zstd 流，以及 tar 流（tar 流用于 docker save）。

```go
type Service struct {
    Name         string                  `json:"name"`
    Containers   []container.Summary     `json:"containers"`
    Dependencies []string                `json:"dependencies"`  // 依赖的其他 Service
    Images       []string                `json:"images"`        // 需要图像列表
    Volumes      []VolumeInfo           `json:"volumes"`       // 数据卷息
    Networks     []NetworkInfo          `json:"networks"`      // 网络配置
    StartOrder   int                    `json:"start_order"`   // 启动顺序
}

type PackageManifest struct {
    Version     string    `json:"version"`
    CreatedAt   time.Time `json:"created_at"`
    Source      string    `json:"source"`        // 源服务器信息
    Services    []Service `json:"services"`
    TotalSize   int64     `json:"total_size"`
    Checksum    string    `json:"checksum"`
}
```

```bash
migration.zst 结构:
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
└── checksums.txt          # 完整性验证
```