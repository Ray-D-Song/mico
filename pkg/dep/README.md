# 容器依赖关系分析

容器之间可能存在复杂的依赖关系，这些依赖关系直接影响容器的启动顺序和迁移成功率。本模块负责分析和处理容器间的依赖关系。

## 依赖关系来源

### 1. 网络依赖
- **Docker Networks**: 连接到相同自定义网络的容器
- **容器链接**: 通过 `--link` 参数创建的链接（已废弃但仍广泛使用）
- **端口暴露**: 容器间通过端口进行服务调用

### 2. 数据卷依赖
- **共享卷**: 多个容器挂载同一个命名卷
- **卷容器模式**: 使用 `--volumes-from` 共享数据卷
- **绑定挂载**: 多个容器挂载同一主机目录

### 3. 环境变量依赖
- **服务发现**: 通过环境变量引用其他容器的主机名/IP
- **配置依赖**: 应用配置中包含对其他服务的引用

### 4. Docker Compose 依赖
- **显式依赖**: `depends_on` 字段声明的依赖关系
- **链接依赖**: `links` 字段创建的服务链接
- **健康检查依赖**: 等待服务健康状态的依赖

### 5. 启动顺序依赖
- **初始化依赖**: 某些容器需要等待其他容器完成初始化
- **健康检查**: 基于容器健康状态的启动顺序

### 6. 文件系统依赖
- **配置文件共享**: 配置容器生成文件供其他容器使用
- **日志依赖**: 多个容器共享日志目录

### 7. 进程间通信依赖
- **Unix Socket**: 通过 Unix socket 进行容器间通信
- **共享内存**: IPC 命名空间共享
- **信号量/管道**: 其他 IPC 机制

### 8. 外部系统依赖
- **数据库依赖**: 应用容器依赖数据库服务
- **消息队列依赖**: 消费者/生产者模式的依赖关系
- **缓存依赖**: 应用对缓存服务的依赖

### 9. 运行时依赖
- **资源依赖**: CPU/内存资源竞争导致的隐式依赖
- **许可证依赖**: 共享许可证服务的依赖

### 10. 业务逻辑依赖
- **微服务依赖**: API Gateway、认证服务等业务层面的依赖
- **数据流依赖**: ETL 流程中的数据处理依赖

## 如何识别依赖关系

### 检测优先级（按准确性和重要性排序）

1. **Docker Compose 文件分析** ⭐⭐⭐⭐⭐
   - 最准确的依赖关系来源
   - 解析 `depends_on`、`links`、`external_links` 字段
   - 支持健康检查条件

2. **网络连接分析** ⭐⭐⭐⭐
   - 检查容器共享的自定义网络
   - 分析 `--link` 参数创建的链接
   - 容易检测，影响面广

3. **数据卷共享分析** ⭐⭐⭐⭐
   - 识别共享命名卷的容器
   - 检查 `--volumes-from` 依赖
   - 影响数据一致性和完整性

4. **环境变量分析** ⭐⭐⭐
   - 检查环境变量中的服务引用
   - 识别 `*_HOST`、`*_PORT`、`*_URL` 等模式
   - 常见的配置方式

5. **端口暴露关系** ⭐⭐⭐
   - 分析容器间的端口依赖
   - 识别服务调用关系
   - 网络服务发现

6. **启动时间顺序** ⭐⭐
   - 基于容器创建时间的启动顺序
   - 最后的备选方案
   - 不够准确但总比随机启动好

### 检测算法

#### 1. 网络依赖检测
```go
func analyzeNetworkDependencies(containers []container.Summary) map[string][]string
```
- 分组分析连接到相同网络的容器
- 检查容器的 Links 配置
- 构建网络拓扑图

#### 2. 卷依赖检测  
```go
func analyzeVolumeDependencies(containers []container.Summary) map[string][]string
```
- 识别共享卷的容器组
- 分析卷的读写权限
- 确定数据流方向

#### 3. 环境变量依赖检测
```go
func analyzeEnvDependencies(containerConfig *container.Config) []string
```
- 解析环境变量中的服务引用
- 匹配服务名模式
- 提取依赖的服务列表

#### 4. Compose 文件解析
```go
func analyzeComposeFile(composePath string) (*DependencyGraph, error)
```
- 解析 docker-compose.yml 文件
- 提取显式依赖关系
- 支持多文件 compose 项目

#### 5. 综合依赖分析
```go
func DetectDependencies(ctx context.Context) (*DependencyGraph, error)
```
- 整合所有检测方法的结果
- 构建完整的依赖图
- 生成启动顺序

## 依赖图数据结构

### DependencyGraph
```go
type DependencyGraph struct {
    nodes map[string]*Node
    edges map[string][]string
}
```

### Node
```go  
type Node struct {
    Name         string
    Container    container.Summary
    Dependencies []string  // 依赖的容器
    Dependents   []string  // 依赖此容器的其他容器
}
```

## 启动顺序算法

使用**拓扑排序**算法确定容器启动顺序：

1. 找到没有依赖的容器作为第一批启动
2. 启动完成后，找到依赖已满足的容器作为下一批
3. 重复直到所有容器都安排了启动顺序
4. 检测循环依赖并报告错误

## 使用示例

```go
detector := NewDependencyDetector(scanner)
graph, err := detector.DetectDependencies(ctx)
if err != nil {
    return err
}

// 获取启动顺序
startupOrder := graph.GetStartupOrder()
for i, batch := range startupOrder {
    fmt.Printf("Batch %d: %v\n", i+1, batch)
}

// 检查循环依赖
if cycles := graph.DetectCycles(); len(cycles) > 0 {
    fmt.Printf("Warning: Circular dependencies detected: %v\n", cycles)
}
```

## 错误处理

- **循环依赖**: 检测并报告循环依赖，提供解决建议
- **缺失依赖**: 识别引用了不存在容器的依赖关系  
- **配置冲突**: 检测不一致的依赖配置

## 扩展性设计

- **插件化检测器**: 支持添加新的依赖检测方法
- **自定义规则**: 允许用户定义业务特定的依赖规则
- **可视化支持**: 生成依赖关系图的可视化表示