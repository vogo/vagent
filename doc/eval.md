# 评估框架 (Eval)

`eval` 包提供 Agent 质量回归测试和持续优化的评估框架。

## 1. 核心接口

### Evaluator

```go
type Evaluator interface {
    Evaluate(ctx context.Context, c *EvalCase) (*EvalResult, error)
}
```

适配器：`EvalFunc func(ctx context.Context, c *EvalCase) (*EvalResult, error)`。

## 2. 数据模型

### EvalCase

```go
type EvalCase struct {
    ID       string               // 用例标识
    Input    *schema.RunRequest   // 输入请求
    Expected *schema.RunResponse  // 期望输出（可选）
    Actual   *schema.RunResponse  // 实际输出
    Criteria []string             // 评估标准列表
    Tags     []string             // 标签（分组和过滤）
}
```

### EvalResult

```go
type EvalResult struct {
    CaseID   string          // 对应的用例 ID
    Score    float64         // 综合评分（0-1）
    Passed   bool            // 是否通过
    Details  []EvalDetail    // 各维度评分详情
    Duration int64           // 评估耗时（毫秒）
    Usage    *aimodel.Usage  // Token 用量
    Error    string          // 错误信息
}
```

### EvalDetail

```go
type EvalDetail struct {
    Name    string   // 维度名称
    Score   float64  // 维度评分（0-1）
    Passed  bool     // 是否通过
    Message string   // 说明
}
```

### EvalReport

```go
type EvalReport struct {
    Results       []*EvalResult // 各用例结果
    TotalCases    int           // 总用例数
    PassedCases   int           // 通过数
    FailedCases   int           // 失败数
    ErrorCases    int           // 错误数
    AvgScore      float64       // 平均评分
    TotalDuration int64         // 总耗时（毫秒）
}
```

## 3. 内建评估器

### ExactMatchEval

精确匹配期望输出。

```go
func NewExactMatchEval() (*ExactMatchEval, error)
```

### ContainsEval

检查输出是否包含指定关键词。

```go
type ContainsConfig struct {
    Keywords      []string // 检查的关键词列表
    PassThreshold float64  // 最低通过分数（默认 1.0）
}

func NewContainsEval(cfg *ContainsConfig) (*ContainsEval, error)
```

### LLMJudgeEval

使用 LLM 作为评判者评分。

```go
func NewLLMJudgeEval(completer aimodel.ChatCompleter, model string) (*LLMJudgeEval, error)
```

### ToolCallEval

验证工具调用序列是否符合预期。

```go
type ToolCallConfig struct {
    StrictArgs bool  // 是否比较工具参数
}

func NewToolCallEval(cfg *ToolCallConfig) (*ToolCallEval, error)
```

### LatencyEval

检查响应时间是否在阈值内。

```go
func NewLatencyEval(thresholdMs int64) (*LatencyEval, error)
```

### CostEval

检查 Token 用量是否在预算内。

```go
type CostConfig struct {
    Budget             int  // 最大 token 数
    FailOnMissingUsage bool // Usage 为 nil 时是否失败
}

func NewCostEval(cfg *CostConfig) (*CostEval, error)
```

## 4. 组合评估器

### CompositeEvaluator

组合多个加权评估器。

```go
type WeightedEvaluator struct {
    Evaluator Evaluator
    Weight    float64
}

type CompositeConfig struct {
    FailFast bool  // 首个错误即停止
}

func NewCompositeEvaluator(cfg *CompositeConfig, evaluators ...WeightedEvaluator) (*CompositeEvaluator, error)
```

## 5. 批量评估

```go
func BatchEval(ctx context.Context, evaluator Evaluator, cases []*EvalCase, opts ...BatchOption) (*EvalReport, error)
func WithConcurrency(n int) BatchOption
```

## 6. 一体化运行与评估

```go
type AgentRunFunc func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)

func RunAndEvaluate(ctx context.Context, runFn AgentRunFunc, evaluator Evaluator, cases []*EvalCase, opts ...BatchOption) (*EvalReport, error)
```

将 Agent 执行和评估合并为单一工作流：先运行 Agent 获取 Actual 结果，再交给评估器评分。
