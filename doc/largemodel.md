# LLM 调用中间件链 (LargeModel)

`largemodel` 包在 Agent 与 `aimodel.ChatCompleter` 之间插入中间件层，基于装饰器模式构建。

## 1. 核心接口

### Middleware

```go
type Middleware interface {
    Wrap(next aimodel.ChatCompleter) aimodel.ChatCompleter
}
```

适配器：`MiddlewareFunc func(next aimodel.ChatCompleter) aimodel.ChatCompleter`。

### Model

包装 `ChatCompleter` 并应用中间件链。

```go
type Model struct { /* internal */ }

func New(base aimodel.ChatCompleter, opts ...ModelOption) *Model
func WithMiddleware(mws ...Middleware) ModelOption
```

### 链构建

```go
func Chain(base aimodel.ChatCompleter, mws ...Middleware) aimodel.ChatCompleter
func DefaultChain(base aimodel.ChatCompleter) aimodel.ChatCompleter
```

## 2. 中间件链执行流程

```
Agent
  │
  ▼
┌──────────┐  ┌────────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────────────┐
│  Log MW  │→│ CircuitBreaker │→│ RateLimit MW│→│  Retry MW   │→│ Timeout MW  │→│  Cache MW   │→│ aimodel.Client     │
│  (日志)  │  │  (熔断)        │  │  (限流)     │  │  (重试)     │  │  (超时)     │  │  (缓存)     │  │ 或 ComposeClient   │
└──────────┘  └────────────────┘  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘  └────────────────────┘
```

## 3. 内建中间件

### LogMiddleware

记录 LLM 调用的请求/响应/耗时/用量。

```go
func NewLogMiddleware(opts ...LogOption) *LogMiddleware
func WithLogger(l *slog.Logger) LogOption
```

### CircuitBreakerMiddleware

熔断保护，连续失败时自动断开，支持半开探测和自动恢复。三态模型：Closed → Open → HalfOpen。

```go
func NewCircuitBreakerMiddleware(opts ...CircuitBreakerOption) *CircuitBreakerMiddleware
func WithFailureThreshold(n int) CircuitBreakerOption      // 默认 5
func WithResetTimeout(d time.Duration) CircuitBreakerOption // 默认 30s
```

错误变量：`ErrCircuitOpen`。

### RateLimitMiddleware

滑动窗口限流，支持 request/min 和 token/min。

```go
func NewRateLimitMiddleware(opts ...RateLimitOption) *RateLimitMiddleware
func WithRequestsPerMin(n int) RateLimitOption
func WithTokensPerMin(n int) RateLimitOption
```

错误变量：`ErrRateLimited`。

### RetryMiddleware

自动重试失败的 LLM 调用，支持指数退避和自定义退避策略。

```go
func NewRetryMiddleware(opts ...RetryOption) *RetryMiddleware
func WithMaxRetries(n int) RetryOption          // 默认 3
func WithBaseDelay(d time.Duration) RetryOption // 默认 1s
func WithMaxDelay(d time.Duration) RetryOption  // 默认 30s
func WithBackoff(b BackoffStrategy) RetryOption
```

可重试状态码：429, 500, 502, 503。同时检测网络错误和 EOF。

```go
type BackoffStrategy interface {
    Delay(attempt int) time.Duration
}

type ExponentialBackoff struct {
    BaseDelay time.Duration
    MaxDelay  time.Duration
    Jitter    float64
}
```

### TimeoutMiddleware

为非流式 LLM 调用设置超时，流式请求直接透传。

```go
func NewTimeoutMiddleware(d time.Duration) *TimeoutMiddleware
```

### CacheMiddleware

缓存相同输入的 LLM 调用结果，使用 SHA-256 哈希作为缓存键。流式请求直接透传。

```go
type Cache interface {
    Get(ctx context.Context, key string) (*aimodel.ChatResponse, bool)
    Set(ctx context.Context, key string, resp *aimodel.ChatResponse, ttl time.Duration)
}

func NewCacheMiddleware(c Cache, opts ...CacheOption) *CacheMiddleware
func WithCacheTTL(d time.Duration) CacheOption
```

内建实现：`MapCache`（内存缓存，支持 TTL 和 LRU 淘汰）。

```go
func NewMapCache(opts ...MapCacheOption) *MapCache
func WithMaxEntries(n int) MapCacheOption
```

### MetricsMiddleware

通过事件分发函数发送 LLM 调用指标（`llm_call_start` / `llm_call_end` / `llm_call_error`）。

```go
type DispatchFunc func(ctx context.Context, event schema.Event)

func NewMetricsMiddleware(dispatch DispatchFunc) *MetricsMiddleware
```

## 4. 流式请求行为

各中间件对流式请求（`ChatCompletionStream`）的处理策略：

| 中间件                   | 流式行为                                                   |
| ------------------------ | ---------------------------------------------------------- |
| LogMiddleware            | 记录流创建的开始和错误，不追踪流内事件                     |
| CircuitBreakerMiddleware | 门控流创建，仅观察流创建错误                               |
| RateLimitMiddleware      | 流创建前检查频率限制，不追踪流式 token 用量                |
| RetryMiddleware          | 流创建失败时重试，不重试流中途错误                         |
| TimeoutMiddleware        | 直接透传，不对流式请求施加超时                             |
| CacheMiddleware          | 直接透传，不缓存流式请求                                   |
| MetricsMiddleware        | 仅测量流连接建立阶段的延迟                                 |

## 5. aimodel 集成

vagent 通过 `github.com/vogo/aimodel` 统一调用多家大模型。

### 核心接口复用

| aimodel 类型               | vagent 使用位置                            |
| -------------------------- | ------------------------------------------ |
| `ChatCompleter`            | largemodel 中间件链的基础接口              |
| `Client`                   | 单模型后端                                 |
| `composes.ComposeClient`   | 多模型调度（Failover/Random/Weighted）     |
| `ChatRequest` / `ChatResponse` | LLM 调用请求/响应                     |
| `Stream` / `StreamChunk`   | 流式响应读取（Recv/Close）                 |
| `Tool` / `FunctionDefinition` | 工具定义传递给 LLM                     |
| `ToolCall` / `FunctionCall`| 工具调用请求解析                           |
| `Usage`                    | Token 用量统计                             |
| `APIError` / `ModelError`  | 错误处理和重试判断                         |

### 集成架构

```
vagent Agent
    │
    ▼
largemodel 中间件链 (Log → CircuitBreaker → RateLimit → Retry → Timeout → Cache)
    │
    ▼ (ChatCompleter 接口)
    │
    ├── aimodel.Client (单模型)
    │     Protocol: openai / anthropic
    │
    └── composes.ComposeClient (多模型调度)
          Strategy: failover / random / weight
          ├── aimodel.Client (Model A)
          ├── aimodel.Client (Model B)
          └── aimodel.Client (Model C)
          内建健康管理 + 指数退避恢复
```

### 错误处理集成

| aimodel 错误类型    | vagent 处理方式                              |
| ------------------- | -------------------------------------------- |
| `APIError`          | 根据 StatusCode 判断可重试性（429/500/503 重试，400/401/403 直接失败）|
| `ModelError`        | 在 ComposeClient 场景下标记模型不健康，切换备用模型 |
| `MultiError`        | 所有模型均失败时，聚合错误信息返回           |
| `ErrNoActiveModels` | 触发 TokenBudgetExhausted 或降级事件         |

> **多模型降级**：不需要单独的 FallbackMiddleware，直接使用 `composes.ComposeClient`（Failover 策略）作为底层 `ChatCompleter`。
