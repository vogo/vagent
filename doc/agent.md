# Agent 详细设计

## 1. 概述

Agent 是 vagent 框架的核心抽象，定义了智能体的统一行为接口。框架提供四种内建 Agent 类型，均通过接口组合和嵌入 `Base` 结构体实现。

---

## 2. 核心接口

### Agent 接口

所有 Agent 类型的基础行为接口（`agent/agent.go`）。

```go
type Agent interface {
    Run(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)
    ID() string
    Name() string
    Description() string
}
```

| 方法        | 参数                          | 返回值                  | 说明                 |
| ----------- | ----------------------------- | ----------------------- | -------------------- |
| Run         | ctx, req *RunRequest          | (*RunResponse, error)   | 执行 Agent           |
| ID          | —                             | string                  | Agent 唯一标识       |
| Name        | —                             | string                  | Agent 名称           |
| Description | —                             | string                  | Agent 描述           |

### StreamAgent 接口

扩展接口，增加流式执行能力。LLMAgent 原生支持流式，RouterAgent 和 WorkflowAgent 通过 `RunToStream` 适配。

```go
type StreamAgent interface {
    Agent
    RunStream(ctx context.Context, req *schema.RunRequest) (*schema.RunStream, error)
}
```

### StreamMiddleware

流式事件拦截器，用于在事件发送前进行转换或观测。

```go
type StreamMiddleware func(next func(schema.Event) error) func(schema.Event) error
```

### Base 与 Config

`Base` 结构体实现 `ID()/Name()/Description()` 三个方法，供具体 Agent 类型嵌入复用。

```go
type Config struct {
    ID          string
    Name        string
    Description string
}

type Base struct {
    AgentID          string
    AgentName        string
    AgentDescription string
}
```

### 便捷函数

| 函数           | 说明                                                         |
| -------------- | ------------------------------------------------------------ |
| `RunText`      | 发送单条文本消息给 Agent，返回 RunResponse                   |
| `RunStreamText`| 发送单条文本消息给 StreamAgent，返回 RunStream               |
| `RunToStream`  | 将非流式 Agent.Run 包装为 RunStream，发送 AgentStart/AgentEnd 事件 |

---

## 3. Agent 类型

```
             ┌─────────────────┐
             │  Agent (接口)   │
             └───────┬─────────┘
       ┌─────────┬───┴────┬──────────┐
       ▼         ▼        ▼          ▼
  ┌─────────┐ ┌──────┐ ┌──────┐ ┌──────┐
  │LLMAgent │ │Work- │ │Router│ │Custom│
  │         │ │flow  │ │Agent │ │Agent │
  └─────────┘ └──────┘ └──────┘ └──────┘
```

| Agent 类型    | 包路径                    | 实现接口                  | 说明                               |
| ------------- | ------------------------- | ------------------------- | ---------------------------------- |
| LLMAgent      | `agent/llmagent`          | Agent, StreamAgent        | ReAct 式工具调用，原生流式支持     |
| WorkflowAgent | `agent/workflowagent`     | Agent, StreamAgent        | 编排执行（顺序/DAG/循环）          |
| RouterAgent   | `agent/routeragent`       | Agent, StreamAgent        | 动态路由分发，基于 RouteFunc       |
| CustomAgent   | `agent`（主包）           | Agent                     | 委托给用户提供的 RunFunc           |

---

## 4. LLMAgent（`agent/llmagent`）

基于大模型的 ReAct 式 Agent，是框架最核心的 Agent 类型。

### 结构体字段

| 字段             | 类型                    | 说明                                     |
| ---------------- | ----------------------- | ---------------------------------------- |
| systemPrompt     | prompt.PromptTemplate   | 系统提示词模板                           |
| model            | string                  | 模型名称                                 |
| chatCompleter    | aimodel.ChatCompleter   | LLM 调用接口                             |
| toolRegistry     | tool.ToolRegistry       | 工具注册表                               |
| memoryManager    | *memory.Manager         | 内存管理器（会话历史）                   |
| maxIterations    | int                     | 最大 ReAct 循环次数（默认 10）           |
| runTokenBudget   | int                     | 单次 Run 的 Token 预算（0 = 无限制）     |
| maxTokens        | *int                    | LLM 单次响应最大 Token 数               |
| temperature      | *float64                | 采样温度                                 |
| streamBufferSize | int                     | 流式事件通道缓冲大小（默认 32）          |
| middlewares      | []StreamMiddleware      | 流式事件中间件链                         |
| hookManager      | *hook.Manager           | Hook 管理器，用于事件分发                |
| inputGuards      | []guard.Guard           | 输入安全检查链                           |
| outputGuards     | []guard.Guard           | 输出安全检查链                           |

### Option 函数

通过 Functional Options 模式配置：

```go
llmagent.New(cfg,
    llmagent.WithChatCompleter(cc),
    llmagent.WithSystemPrompt(prompt.StringPrompt("...")),
    llmagent.WithToolRegistry(registry),
    llmagent.WithMemory(memMgr),
    llmagent.WithMaxIterations(20),
    llmagent.WithRunTokenBudget(4000),
    llmagent.WithMaxTokens(1024),
    llmagent.WithTemperature(0.7),
    llmagent.WithStreamBufferSize(64),
    llmagent.WithStreamMiddleware(mw1, mw2),
    llmagent.WithHookManager(hookMgr),
    llmagent.WithInputGuards(g1),
    llmagent.WithOutputGuards(g2),
)
```

### 执行流程（Run）

```
输入
 │
 ▼
┌──────────────────────────┐
│  InputGuard 链检查       │ ──→ block? ──→ 返回 BlockedError
└──────────┬───────────────┘
           │ pass / rewrite
           ▼
渲染 PromptTemplate（变量插值）
 + 加载会话历史（Session Memory）
 + 上下文压缩（Compressor）
 + 构造用户消息
 │
 ▼
┌──────────────────────────┐
│  ChatCompleter 调用      │◄──────────────┐
│  附带可用工具列表        │               │
└──────────┬───────────────┘               │
           │                               │
           ▼                               │
     FinishReason?                         │
     ┌────┴────┐                           │
     │         │                           │
  stop     tool_calls                      │
     │         │                           │
     │         ▼                           │
     │    执行工具 ──→ 将结果追加到消息 ───┘
     │    (ToolRegistry.Execute)
     ▼
┌──────────────────────────┐
│  OutputGuard 链检查      │ ──→ block? ──→ 返回 BlockedError
└──────────┬───────────────┘
           │ pass / rewrite
           ▼
存储消息到 Working Memory
 → 提升到 Session Memory
 │
 ▼
分发 AgentEnd 事件
 │
 ▼
返回 RunResponse
```

### 终止条件

| 条件               | StopReason              | 说明                                  |
| ------------------ | ----------------------- | ------------------------------------- |
| LLM 返回非工具调用 | `StopReasonComplete`    | 正常完成                              |
| 达到 maxIterations | `StopReasonMaxIterations` | 超过最大迭代次数                    |
| Token 预算耗尽     | `StopReasonBudgetExhausted` | runTokenBudget 用尽，分发 EventTokenBudgetExhausted |

### Token 预算控制

`budgetTracker`（`agent/llmagent/budget.go`）追踪单次 Run 的 Token 消耗：

- **Run 模式**：使用 LLM 返回的真实 Usage.TotalTokens
- **RunStream 模式**：基于流式字节数启发式估算（每 4 字节约 1 token）
- 在每次 LLM 调用前和工具执行前进行预算检查
- 预算为 0 表示无限制

### 流式执行（RunStream）

RunStream 通过 `schema.NewRunStream` 创建后台 goroutine 执行 ReAct 循环，额外发送以下事件：

| 事件                    | 说明                         |
| ----------------------- | ---------------------------- |
| EventAgentStart         | Agent 开始执行               |
| EventIterationStart     | 每次迭代开始                 |
| EventTextDelta          | LLM 文本增量输出             |
| EventToolCallStart/End  | 工具调用开始/结束            |
| EventToolResult         | 工具执行结果                 |
| EventTokenBudgetExhausted | Token 预算耗尽             |
| EventAgentEnd           | Agent 执行结束               |

---

## 5. RouterAgent（`agent/routeragent`）

根据输入动态选择子 Agent 处理请求。

### 核心类型

```go
// Route 将 Agent 与路由描述配对
type Route struct {
    Agent       agent.Agent
    Description string
}

// RouteResult 路由决策结果
type RouteResult struct {
    Agent agent.Agent
    Usage *aimodel.Usage  // 路由决策本身的 Token 消耗（可选）
}

// RouteFunc 路由选择函数
type RouteFunc func(ctx context.Context, req *schema.RunRequest, routes []Route) (*RouteResult, error)
```

### 创建

```go
routeragent.New(cfg, routes, routeragent.WithFunc(routeFunc))
```

### 内建 RouteFunc

| 函数         | 说明                                                    |
| ------------ | ------------------------------------------------------- |
| `FirstFunc`  | 始终选择第一个 Route                                    |
| `IndexFunc(i)` | 选择指定索引的 Route                                  |
| `KeywordFunc(fallback)` | 按关键词匹配 Route.Description（大小写不敏感），支持 fallback |
| `RandomFunc` | 随机选择一个 Route                                      |
| `LLMFunc(cc, model, fallback)` | 使用 LLM 语义理解选择最佳 Route，支持 fallback |

### 执行流程

1. 调用 `RouteFunc` 从 routes 中选择目标 Agent
2. 将原始 `RunRequest` 委托给目标 Agent 执行
3. 合并路由决策的 Usage 和目标 Agent 的 Usage
4. RunStream 通过 `RunToStream` 适配

---

## 6. WorkflowAgent（`agent/workflowagent`）

使用编排引擎执行多 Agent 协作，支持三种模式。

### 三种模式

| 模式     | 构造函数        | 说明                                     |
| -------- | --------------- | ---------------------------------------- |
| Sequence | `New(cfg, steps...)` | 按顺序依次执行子 Agent，前一个输出作为后一个输入 |
| DAG      | `NewDAG(cfg, dagCfg, nodes)` / `NewDAGWithEdges(cfg, dagCfg, nodes, edges)` | 按有向无环图定义的依赖关系执行，支持并行节点 |
| Loop     | `NewLoop(cfg, body, condition, maxIters)` | 循环执行 body Agent，直到 condition 返回 false 或达到 maxIters |

### Sequence 模式

```go
wf := workflowagent.New(cfg, agentA, agentB, agentC)
// 执行顺序：A → B → C
// 每个 step 的输出 Messages 作为下一个 step 的输入 Messages
// Usage 累加
```

### DAG 模式

```go
wf, err := workflowagent.NewDAG(cfg, dagCfg, nodes)
// 或使用边定义
wf, err := workflowagent.NewDAGWithEdges(cfg, dagCfg, nodes, edges)
```

- 构造时即校验 DAG 结构（重复 ID、缺失依赖、环、断连子图）
- 执行委托给 `orchestrate.ExecuteDAG`
- 详细 DAG 引擎设计参见 [orchestrate.md](orchestrate.md)

### Loop 模式

```go
wf := workflowagent.NewLoop(cfg, bodyAgent, func(resp *schema.RunResponse) bool {
    return needContinue(resp) // 返回 true 继续循环
}, 10)
```

- 执行委托给 `orchestrate.ExecuteLoop`
- 每次迭代将上次输出作为输入
- condition 返回 false 或达到 maxIters 时停止

### RunStream

通过 `RunToStream` 适配，发送 AgentStart/AgentEnd 生命周期事件。

---

## 7. CustomAgent（`agent` 主包）

最轻量的 Agent 类型，将 Run 委托给用户提供的函数。

```go
type RunFunc func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)

a := agent.NewCustomAgent(agent.Config{ID: "my-agent"}, func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
    // 自定义逻辑
    return &schema.RunResponse{...}, nil
})
```

- RunFunc 为 nil 时调用 Run 返回错误
- 不实现 StreamAgent 接口

---

## 8. Agent 类型选择指南

### WorkflowAgent 与 RouterAgent 职责边界

| 维度     | WorkflowAgent                    | RouterAgent                      |
| -------- | -------------------------------- | -------------------------------- |
| 决策时机 | 构建时定义 / 基于上游输出的静态条件 | 运行时动态判断                   |
| 决策依据 | 上游输出的结构化字段              | 用户输入的语义理解（LLM 或规则） |
| 输出     | 多节点聚合结果                    | 单一最优 Agent 的结果            |
| 成本     | 低（规则判断）                    | 高（LLMFunc 需要 LLM 调用做路由）|
| 适用场景 | 已知流程、确定性分支              | 开放式输入、需要语义理解的路由   |

> **选择原则**：当分支条件可通过结构化字段判断时，使用 WorkflowAgent；当分支需要理解自然语言语义时，使用 RouterAgent。
