# 核心模型定义 (Schema)

`schema` 包定义框架共享的数据模型，所有其他包依赖此包。

## 1. Message

表示一条对话消息。内嵌 `aimodel.Message`，增加 Agent 层元数据。

```go
type Message struct {
    aimodel.Message                    // 内嵌 aimodel 消息
    AgentID   string         `json:"agent_id,omitempty"`   // 产生该消息的 Agent
    Timestamp time.Time      `json:"timestamp"`            // 创建时间
    Metadata  map[string]any `json:"metadata,omitempty"`   // 扩展元数据
}
```

`aimodel.Message` 核心字段（直接复用）：

| 字段         | 类型              | 说明                                       |
| ------------ | ----------------- | ------------------------------------------ |
| Role         | aimodel.Role      | 消息角色：system / user / assistant / tool |
| Content      | aimodel.Content   | 消息内容（纯文本或多模态）                 |
| ToolCallID   | string            | 工具调用结果对应的 ToolCall ID             |
| ToolCalls    | []aimodel.ToolCall| 模型返回的工具调用请求列表                 |

便捷构造函数：

| 函数                    | 说明                                   |
| ----------------------- | -------------------------------------- |
| `NewUserMessage(text)`  | 创建用户消息                           |
| `NewAssistantMessage(msg, agentID)` | 包装 aimodel.Message 为 Agent 消息 |
| `ToAIModelMessages(msgs)` | 批量转换为 aimodel.Message          |
| `FromAIModelMessage(msg)` | 从 aimodel.Message 转换            |

> `aimodel.Content` 支持多模态：纯文本通过 `NewTextContent(text)` 创建，图文混合通过 `NewPartsContent(parts...)` 创建。

## 2. ContentPart

内容片段，支持多种类型。

```go
type ContentPart struct {
    Type     string `json:"type"`              // text / json / image / file
    Text     string `json:"text,omitempty"`
    Data     []byte `json:"data,omitempty"`
    MimeType string `json:"mime_type,omitempty"`
    URL      string `json:"url,omitempty"`
}
```

## 3. ToolResult

工具执行结果。

```go
type ToolResult struct {
    ToolCallID string        `json:"tool_call_id"`
    Content    []ContentPart `json:"content"`
    IsError    bool          `json:"is_error,omitempty"`
}
```

便捷函数：`TextResult(toolCallID, text)` 和 `ErrorResult(toolCallID, errMsg)`。

## 4. ToolCall

直接复用 `aimodel.ToolCall`。

| 字段      | 类型                   | 说明                         |
| --------- | ---------------------- | ---------------------------- |
| Index     | int                    | 调用索引（流式合并用）       |
| ID        | string                 | 唯一调用标识                 |
| Type      | string                 | 工具类型（固定 "function"）  |
| Function  | aimodel.FunctionCall   | 函数名和参数                 |

> `aimodel.FunctionCall` 包含 `Name`（函数名）和 `Arguments`（JSON 格式参数字符串）。流式场景下通过 `ToolCall.Merge(delta)` 合并增量。

## 5. ToolDef

工具定义，描述 Agent 可调用的工具。

```go
type ToolDef struct {
    Name         string `json:"name"`
    Description  string `json:"description"`
    Parameters   any    `json:"parameters,omitempty"`    // JSON Schema
    ForceUse     bool   `json:"force_use,omitempty"`     // 强制工具选择
    Source       string `json:"source,omitempty"`         // local / mcp / agent
    MCPServerURI string `json:"mcp_server_uri,omitempty"`
    AgentID      string `json:"agent_id,omitempty"`
}
```

工具来源常量：`ToolSourceLocal = "local"` / `ToolSourceMCP = "mcp"` / `ToolSourceAgent = "agent"`。

## 6. RunRequest / RunResponse

Agent 执行请求与响应。

**RunRequest**

```go
type RunRequest struct {
    Messages  []Message      `json:"messages"`
    SessionID string         `json:"session_id,omitempty"`
    Options   *RunOptions    `json:"options,omitempty"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}
```

**RunOptions** — 运行时参数覆盖。

```go
type RunOptions struct {
    Model          string   // 覆盖模型标识
    Temperature    *float64 // 覆盖采样温度
    MaxIterations  int      // 覆盖最大迭代次数
    MaxTokens      int      // 本次调用最大 token 数
    RunTokenBudget int      // 单次 Run token 预算
    Tools          []string // 可用工具名称白名单
    StopSequences  []string // 自定义停止序列
}
```

**RunResponse**

```go
type RunResponse struct {
    Messages   []Message      `json:"messages"`
    SessionID  string         `json:"session_id,omitempty"`
    Usage      *aimodel.Usage `json:"usage,omitempty"`
    Duration   int64          `json:"duration_ms,omitempty"`
    Metadata   map[string]any `json:"metadata,omitempty"`
    StopReason StopReason     `json:"stop_reason,omitempty"`
}
```

**StopReason** 常量：

| 常量                      | 说明                 |
| ------------------------- | -------------------- |
| `StopReasonComplete`      | 正常完成             |
| `StopReasonBudgetExhausted` | Token 预算耗尽     |
| `StopReasonMaxIterations` | 达到最大迭代次数     |

**Usage** — 直接复用 `aimodel.Usage`（PromptTokens / CompletionTokens / TotalTokens）。

## 7. Event

系统事件，用于 Hook 和流式输出。

```go
type Event struct {
    Type      string    // 事件类型
    AgentID   string    // 触发事件的 Agent
    SessionID string    // 会话标识
    Timestamp time.Time // 事件时间戳
    Data      EventData // 事件数据（sealed interface）
    ParentID  string    // 父事件 ID
}
```

### 事件类型常量

| 常量                         | 说明                     |
| ---------------------------- | ------------------------ |
| `EventAgentStart`            | Agent.Run 开始           |
| `EventTextDelta`             | 文本增量                 |
| `EventToolCallStart`         | 工具调用开始             |
| `EventToolCallEnd`           | 工具调用结束             |
| `EventToolResult`            | 工具执行结果             |
| `EventIterationStart`        | 迭代开始                 |
| `EventAgentEnd`              | Agent.Run 结束           |
| `EventError`                 | 错误                     |
| `EventLLMCallStart`          | LLM 调用开始             |
| `EventLLMCallEnd`            | LLM 调用结束             |
| `EventLLMCallError`          | LLM 调用失败             |
| `EventTokenBudgetExhausted`  | Token 预算耗尽           |

### 事件数据类型

所有数据类型实现 sealed `EventData` 接口：

| 数据类型                  | 关键字段                                           |
| ------------------------- | -------------------------------------------------- |
| `AgentStartData`          | 无                                                 |
| `TextDeltaData`           | Delta string                                       |
| `ToolCallStartData`       | ToolCallID, ToolName, Arguments                    |
| `ToolCallEndData`         | ToolCallID, ToolName, Duration                     |
| `ToolResultData`          | ToolCallID, ToolName, Result                       |
| `IterationStartData`      | Iteration int                                      |
| `AgentEndData`            | Duration, Message, StopReason                      |
| `TokenBudgetExhaustedData`| Budget, Used, Iterations, Estimated                |
| `ErrorData`               | Message string                                     |
| `LLMCallStartData`        | Model, Messages, Tools, Stream                     |
| `LLMCallEndData`          | Model, Duration, PromptTokens, CompletionTokens, TotalTokens, Stream |
| `LLMCallErrorData`        | Model, Duration, Error, Stream                     |

## 8. RunStream

流式事件流，基于 pull 模式。

```go
type RunStream struct { /* internal fields */ }

func NewRunStream(ctx context.Context, bufSize int, producer StreamProducer) *RunStream
func (s *RunStream) Recv() (Event, error)   // 获取下一个事件，EOF 表示成功结束
func (s *RunStream) Close() error           // 关闭流
func MergeStreams(ctx context.Context, bufSize int, streams ...*RunStream) *RunStream
```

`StreamProducer` 类型：`func(ctx context.Context, send func(Event) error) error`。
