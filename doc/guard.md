# Guardrails 安全检查 (Guard)

`guard` 包提供输入/输出安全检查链，支持 prompt injection 检测、内容过滤、PII 脱敏、主题限制等。

## 1. 核心接口

### Guard

统一的安全检查接口，所有 Guard 实现此接口。

```go
type Guard interface {
    Check(msg *Message) (*Result, error)
    Name() string
}
```

### Direction

消息方向枚举。

```go
type Direction int

const (
    DirectionInput  Direction = 0  // 用户输入
    DirectionOutput Direction = 1  // Agent 输出
)
```

### Message

Guard 检查的消息结构。

```go
type Message struct {
    Direction Direction
    Content   string
    AgentID   string
    SessionID string
    ToolCalls []schema.ToolResult
    Metadata  map[string]any
}
```

便捷构造：`NewInputMessage(content)` / `NewOutputMessage(content)`。

### Action 与 Result

检查结果。

```go
type Action string

const (
    ActionPass    Action = "pass"
    ActionBlock   Action = "block"
    ActionRewrite Action = "rewrite"
)

type Result struct {
    Action     Action
    Content    string    // rewrite 时的替换内容
    Reason     string    // 拒绝或改写原因
    Violations []string  // 违反的规则列表
    GuardName  string    // 产生结果的 Guard 名称
}
```

便捷构造：`Pass()` / `Block(guardName, reason, violations...)` / `Rewrite(guardName, content, reason, violations...)`。

### BlockedError

当 Guard 拦截时返回的错误类型。

```go
type BlockedError struct {
    Result *Result
}
```

## 2. Guard 链

通过 `RunGuards` 顺序执行多个 Guard，任一 Guard 返回 block 则立即终止。rewrite 结果会更新消息内容后继续后续检查。

```go
func RunGuards(ctx context.Context, msg *Message, guards ...Guard) (*Result, error)
```

执行流程：

```
用户输入
   │
   ▼
┌──────────────────┐
│  InputGuard 链   │ ──→ block? ──→ 返回拒绝响应
│  (顺序执行)      │ ──→ rewrite? ──→ 使用改写内容继续
└──────┬───────────┘
       │ pass
       ▼
   Agent.Run
       │
       ▼
┌──────────────────┐
│  OutputGuard 链  │ ──→ block? ──→ 返回安全提示
│  (顺序执行)      │ ──→ rewrite? ──→ 使用改写内容返回
└──────┬───────────┘
       │ pass
       ▼
   返回用户
```

## 3. 内建 Guard 实现

### PromptInjectionGuard

检测 prompt injection 攻击（越狱、角色劫持等）。

```go
type PromptInjectionConfig struct {
    Patterns []PatternRule  // 正则检测规则
}

type PatternRule struct {
    Name    string
    Pattern *regexp.Regexp
}
```

提供 `DefaultInjectionPatterns()` 获取内建检测规则。

### ContentFilterGuard

过滤有害内容（关键词匹配）。

```go
type ContentFilterConfig struct {
    BlockedKeywords []string
    CaseSensitive   bool
}
```

### PIIGuard

检测并脱敏个人身份信息（PII），使用 rewrite 模式替换敏感内容。

```go
type PIIConfig struct {
    Patterns    []PatternRule
    Replacement string  // 默认 "[REDACTED]"
}
```

提供 `DefaultPIIPatterns()` 获取内建 PII 检测规则。

### TopicGuard

限制对话主题范围。

```go
type TopicConfig struct {
    AllowedTopics []string
    BlockedTopics []string
    CaseSensitive bool
}
```

### LengthGuard

限制输入/输出长度（rune 计数）。

```go
type LengthConfig struct {
    MaxLength int
}
```

### CustomGuard

用户自定义检查逻辑。

```go
type CheckFunc func(msg *Message) (*Result, error)

func NewCustomGuard(name string, fn CheckFunc) *CustomGuard
```

## 4. Guard 类型总结

| Guard 类型            | 模式    | 说明                                       |
| --------------------- | ------- | ------------------------------------------ |
| PromptInjectionGuard  | block   | 正则检测 prompt injection 攻击             |
| ContentFilterGuard    | block   | 关键词过滤有害内容                         |
| PIIGuard              | rewrite | 检测并脱敏 PII                             |
| TopicGuard            | block   | 限制对话主题范围                           |
| LengthGuard           | block   | 限制内容长度                               |
| CustomGuard           | 自定义  | 用户自定义检查逻辑                         |
