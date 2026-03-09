# Hook 与可观测性

`hook` 包提供事件驱动的可观测性机制，支持同步和异步两种 Hook 模式。

## 1. 核心接口

### Hook（同步）

在事件触发线程中同步执行，必须快速且非阻塞。

```go
type Hook interface {
    OnEvent(ctx context.Context, event schema.Event) error
    Filter() []string  // 关注的事件类型，空表示全部
}
```

### AsyncHook（异步）

事件通过 channel 异步分发，不阻塞主流程。

```go
type AsyncHook interface {
    EventChan() chan<- schema.Event
    Filter() []string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

### HookFunc 适配器

将普通函数适配为 Hook 实现。

```go
type HookFunc struct { /* internal */ }

func NewHookFunc(fn func(context.Context, schema.Event) error, types ...string) HookFunc
```

## 2. Manager

事件分发管理器，线程安全。

```go
type Manager struct { /* internal: mu, syncHooks, asyncHooks */ }

func NewManager() *Manager
```

| 方法            | 说明                                                       |
| --------------- | ---------------------------------------------------------- |
| `Register`      | 注册同步 Hook                                              |
| `RegisterAsync` | 注册异步 Hook                                              |
| `Dispatch`      | 分发事件：同步 Hook 顺序执行，异步 Hook 非阻塞 channel 发送 |
| `Start`         | 启动所有异步 Hook（失败时回滚已启动的）                    |
| `Stop`          | 停止所有异步 Hook（收集所有错误）                          |

行为特性：
- 同步 Hook 错误记录日志但不中断后续分发
- 异步 Hook channel 满时丢弃事件并警告
- `Dispatch` 在 context 取消时提前返回
- nil Manager 上调用 `Dispatch` 为 no-op

## 3. 事件类型

完整事件类型定义参见 [schema.md](schema.md) 第 7 节。

| 事件类型                    | 触发时机                     | 典型用途         |
| --------------------------- | ---------------------------- | ---------------- |
| `agent_start`               | Agent.Run 开始               | 轨迹起点         |
| `text_delta`                | LLM 流式文本增量             | 实时输出         |
| `tool_call_start`           | 工具调用开始                 | 调用追踪         |
| `tool_call_end`             | 工具调用结束                 | 延迟统计         |
| `tool_result`               | 工具执行结果                 | 结果记录         |
| `iteration_start`           | ReAct 迭代开始               | 迭代追踪         |
| `agent_end`                 | Agent.Run 结束               | 轨迹终点         |
| `error`                     | 执行错误                     | 错误监控         |
| `llm_call_start`            | LLM 调用开始                 | 性能分析         |
| `llm_call_end`              | LLM 调用结束                 | token 用量统计   |
| `llm_call_error`            | LLM 调用失败                 | 错误追踪         |
| `token_budget_exhausted`    | Token 预算耗尽               | 成本控制         |

## 4. 轨迹采集

通过 Hook 机制可完整采集 Agent 执行轨迹：

```
Trace
├── AgentStart (input="...")
├── LLMStart (model="gpt-4o", messages=[...])
├── LLMEnd (finish_reason="tool_calls", usage={...})
├── ToolStart (name="search", args="...")
├── ToolEnd (result="...")
├── LLMStart (messages=[... + tool_result])
├── LLMEnd (finish_reason="stop")
└── AgentEnd (output="...")
```

## 5. 轨迹采集与分析维度

| 采集维度       | 数据内容                                 | 用途                   |
| -------------- | ---------------------------------------- | ---------------------- |
| Token 用量     | 每次 LLM 调用的 prompt/completion tokens | 成本分析               |
| 延迟           | 各环节耗时                               | 性能瓶颈定位           |
| 工具调用链     | 工具名称、参数、结果序列                 | 行为分析               |
| 决策路径       | 路由选择、条件分支                       | Agent 行为理解         |
| 错误分布       | 错误类型、频率、上下文                   | 可靠性改进             |

## 6. Token 预算控制

框架内建 Token 预算机制，通过 LLM Middleware 和 Hook 协作实现。

| 配置项                 | 类型   | 说明                                         |
| ---------------------- | ------ | -------------------------------------------- |
| Agent.RunTokenBudget   | int    | 单次 Run 调用最大 token 数（0 表示不限制）   |
| Service.DailyTokenLimit| int    | 全局每日 token 上限                          |
| Service.TokenPerMinute | int    | 全局每分钟 token 速率限制                    |

当 Token 预算耗尽时：
1. LLM Middleware 检查剩余预算
2. 若不足以支撑下一次 LLM 调用，终止推理循环
3. 返回部分结果 + 预算耗尽提示
4. 触发 `TokenBudgetExhausted` 事件
