# 内存系统 (Memory)

## 1. 三级内存架构

```
┌─────────────────────────────────────────────────┐
│                  Memory System                  │
│                                                 │
│  ┌─────────────┐                                │
│  │   Working    │  当前推理循环的临时上下文      │
│  │   Memory     │  生命周期：单次 Run 调用       │
│  └──────┬──────┘                                │
│         │ 提升 (Promoter)                        │
│  ┌──────▼──────┐                                │
│  │   Session    │  会话级上下文                  │
│  │   Memory     │  生命周期：一个 Session        │
│  └──────┬──────┘                                │
│         │ 归档 (Archiver)                        │
│  ┌──────▼──────┐                                │
│  │  Persistent  │  持久化存储                    │
│  │   Memory     │  生命周期：跨 Session 持久     │
│  └─────────────┘                                │
└─────────────────────────────────────────────────┘
```

## 2. 各级内存对比

| 维度       | Working Memory           | Session Memory           | Persistent Memory         |
| ---------- | ------------------------ | ------------------------ | ------------------------- |
| 生命周期   | 单次 Run 调用            | 一个 Session             | 跨 Session 持久           |
| 存储位置   | 进程内存                 | 进程内存                 | 进程内存（可扩展为持久化）|
| 并发安全   | 不安全（单 goroutine）   | 安全（sync.Mutex）       | 安全（sync.Mutex）        |
| 典型内容   | 当前消息、工具结果       | 对话历史、中间状态       | 用户画像、知识库、长期记忆|

## 3. 核心接口

### 3.1 Store 接口

底层 KV 存储抽象，实现者仅需支持单 goroutine 使用，并发控制由上层 Memory 处理。

```go
type Store interface {
    Get(ctx context.Context, key string) (any, bool, error)
    Set(ctx context.Context, key string, value any, ttl int64) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]StoreEntry, error)
    Clear(ctx context.Context) error
}
```

可选的批量操作接口：

```go
type BatchStore interface {
    BatchGet(ctx context.Context, keys []string) (map[string]any, error)
    BatchSet(ctx context.Context, entries map[string]any, ttl int64) error
}
```

### 3.2 Memory 接口

核心内存操作接口，三级内存均实现此接口。

```go
type Memory interface {
    Get(ctx context.Context, key string) (any, error)
    Set(ctx context.Context, key string, value any, ttl int64) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]Entry, error)
    Clear(ctx context.Context) error
    BatchGet(ctx context.Context, keys []string) (map[string]any, error)
    BatchSet(ctx context.Context, entries map[string]any, ttl int64) error
}
```

### 3.3 Entry

内存条目。

| 字段      | 类型      | 说明                                 |
| --------- | --------- | ------------------------------------ |
| Key       | string    | 条目键                               |
| Value     | any       | 条目值                               |
| Scope     | Scope     | 内存层级：working / session / store  |
| AgentID   | string    | 所属 Agent                           |
| SessionID | string    | 所属会话                             |
| CreatedAt | time.Time | 创建时间                             |
| TTL       | int64     | 过期时间（秒），0 表示不过期         |

## 4. 内存实现

### 4.1 内部基础结构

- **memoryBase**：所有层级共享的基础实现，持有 Store 引用、Scope、AgentID、SessionID。
- **syncMemory**：包装 memoryBase，通过 `sync.Mutex` 提供并发安全，供 SessionMemory 和 PersistentMemory 使用。

### 4.2 WorkingMemory

- 通过 `memoryBase` 直接实现，无同步机制
- 每次 Run 创建独立实例，Run 结束后丢弃
- 单 goroutine 访问，天然安全
- 所有方法检查 `ctx.Err()` 支持取消

```go
NewWorkingMemory(agentID, sessionID string) *WorkingMemory
NewWorkingMemoryWithStore(store Store, agentID, sessionID string) *WorkingMemory
```

### 4.3 SessionMemory

- 通过 `syncMemory` 包装，使用 `sync.Mutex` 保证并发安全
- 跨多次 Run 持久，在 Session 生命周期内共享

```go
NewSessionMemory(agentID, sessionID string) *SessionMemory
NewSessionMemoryWithStore(store Store, agentID, sessionID string) *SessionMemory
```

### 4.4 PersistentMemory

- 通过 `syncMemory` 包装，使用 `sync.Mutex` 保证并发安全
- 跨 Session 持久，全局共享

```go
NewPersistentMemory() *PersistentMemory
NewPersistentMemoryWithStore(store Store) *PersistentMemory
```

### 4.5 MapStore

内建的内存 KV 存储实现，同时实现 `Store` 和 `BatchStore` 接口。

- 基于 `map[string]*storeRecord`
- 支持 TTL 自动过期（Get/List 时惰性清理）
- 不安全并发（由上层 Memory 负责同步）

```go
NewMapStore() *MapStore
```

## 5. 内存流转

### 5.1 流转流程

1. **Run 开始** — 创建 WorkingMemory，加载 Session 上下文
2. **推理循环中** — 工具调用结果、中间推理写入 WorkingMemory
3. **Run 结束** — WorkingMemory 中的关键信息提升到 SessionMemory（由 Promoter 决策）
4. **Session 结束** — SessionMemory 中的重要信息归档到 PersistentMemory（由 Archiver 决策）

### 5.2 Promoter（提升策略）

决定 WorkingMemory 中哪些条目提升到 SessionMemory。

```go
type Promoter interface {
    Promote(ctx context.Context, entries []Entry) ([]Entry, error)
}
```

内建策略：

| 策略           | 说明                       |
| -------------- | -------------------------- |
| `PromoteAll()` | 提升所有条目（默认）       |
| `PromoteNone()`| 不提升任何条目             |

`PromoteFunc` 函数适配器可将普通函数转为 Promoter 接口。

### 5.3 Archiver（归档策略）

决定 SessionMemory 中哪些条目归档到 PersistentMemory。

```go
type Archiver interface {
    Archive(ctx context.Context, entries []Entry) ([]Entry, error)
}
```

内建策略：

| 策略            | 说明                       |
| --------------- | -------------------------- |
| `ArchiveAll()`  | 归档所有条目               |
| `ArchiveNone()` | 不归档任何条目（默认）     |

`ArchiveFunc` 函数适配器可将普通函数转为 Archiver 接口。

## 6. 上下文压缩 (ContextCompressor)

WorkingMemory 受 LLM 上下文窗口限制，当消息列表过长时需要压缩。

### 6.1 接口定义

```go
type ContextCompressor interface {
    Compress(ctx context.Context, messages []schema.Message, maxTokens int) ([]schema.Message, error)
}
```

`CompressFunc` 函数适配器可将普通函数转为 ContextCompressor 接口。

### 6.2 TokenBudgetCompressor

按 token 预算从最近消息向前保留，精确控制上下文长度。

- 反向迭代：从最新消息向最旧消息累加 token 数
- 始终保留至少一条消息（最新的）
- maxTokens ≤ 0 时返回所有消息不变

```go
NewTokenBudgetCompressor() *TokenBudgetCompressor
```

### 6.3 SlidingWindowCompressor

滑动窗口压缩，保留最近 N 条消息。

- 先按窗口大小截取最近 N 条消息
- 再委托内部 TokenBudgetCompressor 按 token 预算进一步裁剪
- 可配置自定义 `TokenEstimator`

```go
NewSlidingWindowCompressor(windowSize int) *SlidingWindowCompressor
```

### 6.4 ImportanceRankingCompressor

按消息重要性评分排序，优先保留高分消息。

- 使用 `MessageScorer` 函数对每条消息打分
- 按分数降序排列，在 token 预算内保留高分消息
- 始终保留最高分消息
- 输出保持原始时间顺序

```go
NewImportanceRankingCompressor(scorer MessageScorer) *ImportanceRankingCompressor
NewImportanceRankingCompressorWithDefaults() *ImportanceRankingCompressor
```

**DefaultMessageScorer 评分规则**：

| 消息类型                   | 基础分 | 说明                     |
| -------------------------- | ------ | ------------------------ |
| System 消息                | 1000   | 最高优先级               |
| Tool 消息                  | 100    | 工具调用结果             |
| Assistant（含 ToolCalls）  | 100    | 工具调用请求             |
| User 消息                  | 50     | 用户输入                 |
| 普通 Assistant 消息        | 10     | 一般回复                 |

所有消息额外获得新近度加分：`base * 0.05 * index / total`。

### 6.5 SummarizeAndTruncCompressor

将较早消息摘要，保留最近消息。

- 将消息分为"较早"和"最近"两部分
- 通过自定义 `Summarizer` 函数摘要较早消息
- 保留最后 `keepLastN` 条消息原文
- 摘要消息前置，元数据标记 `compressed=true`、`strategy=summarize_and_trunc`
- 摘要角色可配置（默认 `aimodel.RoleUser`）

```go
NewSummarizeAndTruncCompressor(summarizer Summarizer, keepLastN int, opts ...SummarizeOption) *SummarizeAndTruncCompressor
```

### 6.6 ChainCompressor

组合多个压缩器，按顺序依次执行。

- 前一个压缩器的输出作为下一个的输入
- 适合组合使用，如：SlidingWindow → TokenBudget → ImportanceRanking

```go
NewChainCompressor(compressors ...ContextCompressor) *ChainCompressor
```

## 7. Token 估算

```go
type TokenEstimator func(msg schema.Message) int
```

**DefaultTokenEstimator**：启发式估算，`len(text) / 4`，非空内容最小返回 1。仅考虑文本内容。

所有压缩器通过 `WithTokenEstimator()` 方法支持自定义估算器。

## 8. Manager（内存管理器）

Manager 编排三级内存系统，协调提升和归档流程。

```go
NewManager(opts ...ManagerOption) *Manager
```

### 配置选项

| 选项                         | 说明                     | 默认值         |
| ---------------------------- | ------------------------ | -------------- |
| `WithSession(Memory)`        | 设置 Session 层内存      | nil            |
| `WithStore(Memory)`          | 设置 Store 层内存        | nil            |
| `WithPromoter(Promoter)`     | 设置提升策略             | `PromoteAll()` |
| `WithArchiver(Archiver)`     | 设置归档策略             | `ArchiveNone()`|
| `WithCompressor(Compressor)` | 设置上下文压缩策略       | nil            |

### 关键方法

| 方法                     | 说明                                                       |
| ------------------------ | ---------------------------------------------------------- |
| `Session() Memory`       | 获取 Session 层内存                                        |
| `Store() Memory`         | 获取 Store 层内存                                          |
| `Compressor()`           | 获取上下文压缩器                                           |
| `PromoteToSession(ctx, working)` | 将 WorkingMemory 条目经 Promoter 过滤后写入 SessionMemory |
| `ArchiveToStore(ctx)`    | 将 SessionMemory 条目经 Archiver 过滤后写入 PersistentMemory |

> 提升和归档操作中的错误为非致命错误，不会导致 Run 失败。

## 9. 设计模式

| 模式             | 应用                                                     |
| ---------------- | -------------------------------------------------------- |
| 组合优于继承     | memoryBase 嵌入 syncMemory；压缩器可插拔组合             |
| 函数适配器       | CompressFunc、PromoteFunc、ArchiveFunc 简化接口实现      |
| 建造者模式       | Manager 通过 ManagerOption 函数选项配置                   |
| 策略模式         | Promoter、Archiver、ContextCompressor 均为可替换策略     |
| 上下文传播       | 所有方法使用 context.Context 支持取消                    |

## 10. 文件结构

```
memory/
├── memory.go                        # Store、Memory、Scope、Entry 核心接口
├── base.go                          # memoryBase、syncMemory 共享实现
├── working.go                       # WorkingMemory（单次 Run，无同步）
├── session.go                       # SessionMemory（Session 级，sync.Mutex）
├── persistent.go                    # PersistentMemory（跨 Session，sync.Mutex）
├── mapstore.go                      # MapStore 内存 KV 存储实现
├── compressor.go                    # ContextCompressor 接口、SlidingWindowCompressor
├── compressor_token_budget.go       # TokenBudgetCompressor
├── compressor_importance.go         # ImportanceRankingCompressor、DefaultMessageScorer
├── compressor_summarize_trunc.go    # SummarizeAndTruncCompressor
├── compressor_chain.go              # ChainCompressor
├── token_estimate.go                # TokenEstimator、DefaultTokenEstimator
├── promoter.go                      # Promoter 接口、PromoteAll/PromoteNone
├── archiver.go                      # Archiver 接口、ArchiveAll/ArchiveNone
├── manager.go                       # Manager 编排管理
└── *_test.go                        # 各组件单元测试
```
