# 编排机制 (Orchestration)

## 1. 概述

`orchestrate` 包提供基于 DAG（有向无环图）的任务编排引擎，支持并行执行、条件分支、动态节点生成、循环迭代、检查点恢复、补偿回滚、背压控制和优先级调度。

### 编排模式

| 模式   | 说明                                                   | 适用场景                       |
| ------ | ------------------------------------------------------ | ------------------------------ |
| 顺序   | 节点依次执行，输出链式传递（DAG 的线性特例）           | 文档处理管线、ETL              |
| DAG    | 按有向无环图拓扑排序执行，支持并行+依赖+条件分支       | 复杂任务分解、多步推理         |
| 循环   | 受控迭代执行，支持条件终止和最大次数限制               | 自修正、迭代优化               |

> 并行编排和条件编排是 DAG 模式的子集：并行 = 多个无依赖节点；条件 = 通过 ConditionalNode 动态选择激活的下游分支。

## 2. 核心接口

### 2.1 Runner

执行单元的核心接口，所有 DAG 节点通过 Runner 执行实际逻辑。

```go
type Runner interface {
    Run(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)
}
```

### 2.2 Aggregator

终端节点结果聚合接口。

```go
type Aggregator interface {
    Aggregate(ctx context.Context, results map[string]*schema.RunResponse) (*schema.RunResponse, error)
}
```

内建聚合策略：

| 策略                       | 说明                                             |
| -------------------------- | ------------------------------------------------ |
| `LastResultAggregator()`   | 按节点 ID 排序，取最后一个终端节点的结果         |
| `ConcatMessagesAggregator()` | 按节点 ID 排序，拼接所有终端节点的消息列表     |

### 2.3 CheckpointStore

检查点持久化接口，支持 DAG 恢复和重放。

```go
type CheckpointStore interface {
    Save(ctx context.Context, dagID, nodeID string, resp *schema.RunResponse) error
    Load(ctx context.Context, dagID, nodeID string) (*schema.RunResponse, error)
    LoadAll(ctx context.Context, dagID string) (map[string]*schema.RunResponse, error)
    Clear(ctx context.Context, dagID string) error
}
```

内建实现：`NewInMemoryCheckpointStore()` — 基于内存的线程安全实现（sync.RWMutex）。

### 2.4 Compensatable

可补偿接口，Runner 实现此接口声明补偿（回滚）能力。

```go
type Compensatable interface {
    Compensate(ctx context.Context, original *schema.RunResponse) error
}
```

### 2.5 IdempotentChecker

幂等性声明接口，影响补偿重试策略。

```go
type IdempotentChecker interface {
    Idempotent() bool
}
```

### 2.6 DAGEventHandler

DAG 执行生命周期事件处理器（线程安全）。

```go
type DAGEventHandler interface {
    OnNodeStart(nodeID string)
    OnNodeComplete(nodeID string, status NodeStatus, err error)
    OnCheckpointError(nodeID string, err error)
}
```

## 3. 节点类型

### 3.1 Node（基础节点）

DAG 执行图中的基本节点。

```go
type Node struct {
    ID           string
    Runner       Runner
    Deps         []string                                                    // 上游依赖节点 ID
    InputMapper  InputMapFunc                                                // 上游输出到当前输入的映射
    Optional     bool                                                        // true 时失败可降级（Skip 策略下）
    Condition    func(upstreamResults map[string]*schema.RunResponse) bool   // 节点执行条件
    Timeout      time.Duration                                               // 节点执行超时
    Retries      int                                                         // 最大重试次数
    ResourceTags []string                                                    // 资源标签（并发/速率控制）
    Priority     int                                                         // 调度优先级（值越大越优先）
}
```

**InputMapFunc** — 将上游节点输出映射为当前节点输入：

```go
type InputMapFunc func(upstreamResults map[string]*schema.RunResponse) (*schema.RunRequest, error)
```

### 3.2 ConditionalNode（条件节点）

根据上游输出动态选择激活的下游分支。

```go
type ConditionalNode struct {
    Node                  // 嵌入基础节点
    Branches   []Branch   // 条件分支列表，按顺序评估，首个匹配生效
    Default    string     // 默认分支目标节点 ID
    Exhaustive bool       // true 时强制要求 Default 非空
}

type Branch struct {
    Condition func(upstreamResults map[string]*schema.RunResponse) bool
    TargetID  string
}
```

关键函数：
- `ValidateConditionalNode(cn ConditionalNode)` — 校验结构合法性
- `EvaluateBranches(cn ConditionalNode, results)` — 评估条件，返回目标节点 ID
- `ExecuteConditional(ctx, cn, req, results)` — 执行节点并评估分支

### 3.3 DynamicSpawnNode（动态生成节点）

运行时根据输出动态创建子节点，实现 Map-Reduce 模式。

```go
type DynamicSpawnNode struct {
    Node                                                                              // 嵌入基础节点
    Spawner         func(ctx context.Context, output *schema.RunResponse) ([]Node, error)  // 子节点生成器
    SpawnAggregator Aggregator                                                        // 子节点结果聚合器
    MaxSpawnCount   int                                                               // 最大生成节点数（0 不限制）
    SpawnTimeout    time.Duration                                                     // 生成函数超时
    SpawnDepthLimit int                                                               // 最大嵌套深度（0 禁止嵌套）
}
```

安全约束：
- `MaxSpawnCount` 防止单次 Spawn 产生过多节点
- `SpawnDepthLimit` 防止递归 Spawn 爆炸（通过 context 追踪深度）

### 3.4 LoopNode（循环节点）

受控迭代执行，不违反 DAG 无环约束（内部迭代由节点自身控制）。

```go
type LoopNode struct {
    Body            Runner                                          // 循环体
    Condition       func(*schema.RunResponse) bool                  // 继续循环条件（true 继续）
    MaxIters        int                                             // 最大迭代次数
    ConvergenceFunc func(prev, curr *schema.RunResponse) bool       // 收敛检测（true 已收敛，停止）
}
```

终止条件（任一满足即停止）：
1. `Condition` 返回 false
2. `ConvergenceFunc` 返回 true
3. 迭代次数达到 `MaxIters`

`ExecuteLoop(ctx, ln, req)` 在迭代间累加 Usage 统计。

## 4. DAG 配置

```go
type DAGConfig struct {
    MaxConcurrency     int                     // 全局最大并行节点数
    ErrorStrategy      ErrorStrategy           // 错误处理策略
    EarlyExitFunc      func(nodeID string, resp *schema.RunResponse) bool  // 提前终止判断
    Aggregator         Aggregator              // 终端节点结果聚合器
    CheckpointStore    CheckpointStore         // 检查点存储
    ReplayMode         bool                    // 重放模式
    PriorityScheduling bool                    // 启用优先级调度
    CriticalPathAuto   bool                    // 自动计算关键路径优先级
    BackpressureCfg    *BackpressureConfig     // 背压配置
    ResourceLimits     map[string]int          // 按资源标签限制并发数
    ResourceRateLimits map[string]float64      // 按资源标签限制速率
    CompensateCfg      *CompensateConfig       // 补偿配置
    EventHandler       DAGEventHandler         // 生命周期事件处理器
}
```

### 函数选项

通过 `DAGOption` 函数选项配置：

| 选项                        | 说明                                 |
| --------------------------- | ------------------------------------ |
| `WithMaxConcurrency(n)`     | 设置全局最大并行节点数               |
| `WithErrorStrategy(s)`      | 设置错误处理策略                     |
| `WithEarlyExit(f)`          | 设置提前终止判断函数                 |
| `WithAggregator(a)`         | 设置终端节点结果聚合器               |
| `WithCheckpointStore(s)`    | 设置检查点存储                       |
| `WithReplayMode()`          | 启用重放模式                         |
| `WithPriorityScheduling()`  | 启用优先级调度                       |
| `WithBackpressure(cfg)`     | 启用背压控制                         |
| `WithResourceLimits(m)`     | 设置资源标签并发限制                 |
| `WithResourceRateLimits(m)` | 设置资源标签速率限制                 |
| `WithCompensation(cfg)`     | 设置补偿配置                         |
| `WithEventHandler(h)`       | 设置事件处理器                       |

## 5. DAG 执行结果

```go
type DAGResult struct {
    NodeResults map[string]*schema.RunResponse  // 各节点执行结果
    NodeStatus  map[string]NodeStatus           // 各节点状态
    FinalOutput *schema.RunResponse             // 聚合后最终输出
    Usage       *aimodel.Usage                  // 累计 Token 用量
    Timeline    []NodeTimeline                  // 执行时间线（甘特图数据）
}
```

**NodeStatus** 枚举：`NodePending` / `NodeRunning` / `NodeDone` / `NodeFailed` / `NodeSkipped`

**NodeTimeline** — 单节点执行时间记录：

| 字段      | 类型          | 说明               |
| --------- | ------------- | ------------------ |
| NodeID    | string        | 节点 ID            |
| StartTime | time.Time     | 开始时间           |
| EndTime   | time.Time     | 结束时间           |
| Duration  | time.Duration | 执行耗时           |
| Status    | NodeStatus    | 最终状态           |

## 6. DAG 执行引擎

### 6.1 入口函数

```go
// 核心执行函数
func ExecuteDAG(ctx context.Context, cfg DAGConfig, nodes []Node, req *schema.RunRequest) (*DAGResult, error)

// 函数选项包装
func RunDAG(ctx context.Context, nodes []Node, req *schema.RunRequest, opts ...DAGOption) (*DAGResult, error)

// 边列表构建 DAG
func BuildDAG(nodes []Node, edges [][2]string) ([]Node, error)

// 预执行验证
func ValidateDAG(nodes []Node) error
```

### 6.2 执行流程

```
ExecuteDAG(cfg, nodes, req)
       │
       ▼
┌──────────────────────┐
│  构建邻接表 + 入度表 │
│  环路检测 (DFS)      │ ──→ 有环 ──→ 返回错误
│  连通性检查 (BFS)    │
│  关键路径分析 (CPM)  │  （若启用 CriticalPathAuto）
│  初始化背压控制器    │  （若启用 BackpressureCfg）
│  初始化资源管理器    │  （若配置 ResourceLimits/RateLimits）
└──────┬───────────────┘
       │
       ▼
┌──────────────────────┐
│  加载检查点          │ ──→ 有历史结果 ──→ 标记为 Done，跳过执行
└──────┬───────────────┘
       │
       ▼
┌──────────────────────┐
│  入度=0 的节点       │
│  加入就绪队列        │  （FIFO 或优先级队列）
└──────┬───────────────┘
       │
       ▼
┌──────────────────────────────────────────────┐
│  事件驱动调度循环:                            │
│    就绪节点 → 资源/背压检查 → goroutine 执行 │
│    ├── Condition 检查（跳过不满足条件的节点） │
│    ├── InputMapper 构造输入                   │
│    ├── Runner.Run(ctx, req)（含重试）         │
│    ├── 保存检查点（若启用）                   │
│    └── 通过 channel 通知调度器                │
│                                               │
│  调度器收到完成通知后：                        │
│    ├── 检查 EarlyExit 条件                    │
│    ├── 错误处理（Abort/Skip/Compensate）      │
│    ├── 更新下游节点入度                       │
│    └── 入度=0 的节点立即加入就绪队列          │
└──────┬───────────────────────────────────────┘
       │ 所有节点完成 / EarlyExit 触发
       ▼
┌──────────────────────┐
│  收集终端节点结果     │
│  Aggregator 聚合     │
│  生成 DAGResult      │
└──────────────────────┘
```

> **事件驱动即时调度**：每个节点完成后立即触发下游入度检查，不需要等待同批次其他节点完成。节点 A 完成后，若节点 C 的所有依赖已满足，C 会立即启动。

### 6.3 拓扑排序与环路检测

- **topologicalSort()** — Kahn 算法，返回节点拓扑序
- **detectCycle()** — DFS 三色标记法（白/灰/黑），检测环路
- **checkConnected()** — BFS 检验所有节点连通性

## 7. 错误处理策略

```go
type ErrorStrategy int

const (
    Abort      ErrorStrategy = iota  // 立即终止整个 DAG（默认）
    Skip                             // 跳过失败节点（需 Optional=true）
    Compensate                       // 触发补偿（Saga 模式）
)
```

节点级重试通过 `Node.Retries` 配置，在 Runner 执行失败后自动重试指定次数。

## 8. 补偿机制 (Saga 模式)

### CompensateConfig

```go
type CompensateConfig struct {
    Strategy   CompensateStrategy  // 补偿策略
    Timeout    time.Duration       // 单个节点补偿超时
    MaxRetries int                 // 补偿失败重试次数（需幂等）
}

type CompensateStrategy int

const (
    BackwardCompensate CompensateStrategy = iota  // 按拓扑逆序回滚（默认）
    ForwardRecovery                               // 向前恢复，重试失败节点
)
```

### 补偿执行规则

- 仅对已成功完成且实现了 `Compensatable` 接口的 Runner 执行补偿
- `BackwardCompensate`：按拓扑逆序回滚已完成节点
- `ForwardRecovery`：先尝试重试失败节点，失败则回退到反向补偿
- 补偿失败时：若 `IdempotentChecker.Idempotent()` 为 true 则重试（指数退避）
- 补偿错误通过 `DAGEventHandler.OnCheckpointError` 报告

## 9. 检查点与恢复

### 恢复流程

```
DAG 执行: A ✓ → B ✓ → C ✗ (失败)
                         │
                    保存检查点: {A: result, B: result}
                         │
恢复执行: Resume
    A → 读取检查点 → 跳过（已完成）
    B → 读取检查点 → 跳过（已完成）
    C → 无检查点 → 重新执行
```

**ReplayMode** — 所有在检查点中的节点跳过实际执行，直接使用历史结果，用于调试和数据流分析。

## 10. 并发控制

| 机制                 | 说明                                                       |
| -------------------- | ---------------------------------------------------------- |
| channel + goroutine  | 事件驱动调度，节点完成后通过 channel 通知调度器             |
| context.WithCancel   | 支持 Abort 策略和 EarlyExit 下快速取消所有进行中节点       |
| sync.Mutex           | 保护入度表和结果表的并发读写                               |
| MaxConcurrency       | 全局最大并行节点数（信号量控制）                           |
| ResourceLimits       | 按资源标签限制并发数（如 `{"gpu": 2, "external-api": 5}`） |
| ResourceRateLimits   | 按资源标签的速率限制（令牌桶），与并发数限制互补           |
| PriorityQueue        | 就绪队列支持优先级排序（heap），关键路径节点优先调度       |
| BackpressureControl  | 根据下游延迟 AIMD 算法动态调整并发度                       |

### 资源管理器 (resourceManager)

- 按资源标签细粒度管理并发和速率
- 获取多标签时按排序顺序锁定，防止 ABBA 死锁
- 速率限制使用令牌桶算法

### 背压控制 (backpressureController)

AIMD（加性增/乘性减）算法动态调整并发度：

```go
type BackpressureConfig struct {
    InitialConcurrency int           // 初始并发度
    MinConcurrency     int           // 最小并发度
    MaxConcurrency     int           // 最大并发度
    LatencyThreshold   time.Duration // 延迟阈值
    AdjustInterval     time.Duration // 调整间隔
}
```

- 平均延迟 ≤ 阈值：并发度 +1（加性增）
- 平均延迟 > 阈值：并发度 /2（乘性减）

### 优先级调度

- 基于 `container/heap` 的优先级队列
- 高 Priority 值优先调度，相同优先级按入队顺序（FIFO）
- `ComputeCriticalPath(nodes)` — 关键路径法（CPM）自动计算节点优先级

## 11. 时间线追踪 (timelineTracker)

线程安全记录节点执行时间，用于生成甘特图数据。

- `recordStart(nodeID)` — 记录开始时间
- `recordEnd(nodeID, status)` — 记录结束时间和状态
- `result()` — 返回按开始时间排序的 `[]NodeTimeline`

## 12. 文件结构

```
orchestrate/
├── orchestrate.go      # Runner、Node、DAGConfig、DAGResult、ErrorStrategy 等核心定义
├── dag.go              # DAG 执行引擎（dagExecutor、ExecuteDAG、RunDAG、BuildDAG）
├── conditional.go      # ConditionalNode、Branch、分支评估
├── spawn.go            # DynamicSpawnNode、动态子节点生成
├── loop.go             # LoopNode、循环执行
├── aggregator.go       # Aggregator 接口、LastResult/ConcatMessages 聚合器
├── checkpoint.go       # CheckpointStore 接口、InMemoryCheckpointStore
├── compensate.go       # Compensatable、CompensateConfig、补偿执行逻辑
├── backpressure.go     # backpressureController、AIMD 算法
├── priority.go         # priorityQueue、ComputeCriticalPath
├── resource.go         # resourceManager、tokenBucket 速率限制
├── timeline.go         # timelineTracker、NodeTimeline
├── topo.go             # 拓扑排序、环路检测、连通性检查
└── *_test.go           # 各组件单元测试和集成测试
```
