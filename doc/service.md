# 服务层 (Service)

`service` 包提供 HTTP 服务、Agent 注册与生命周期管理、异步任务管理。

## 1. 使用模式

vagent 支持两种使用模式：

| 模式       | 说明                                                   | 典型场景             |
| ---------- | ------------------------------------------------------ | -------------------- |
| 嵌入式     | 作为库引入，直接调用 Agent API                         | 后端服务、CLI 工具   |
| 独立部署   | 启动 HTTP Server，通过 API 接口访问                    | 微服务、Agent 平台   |

### 嵌入式使用

```
应用程序
  │
  ├── 创建 Agent 实例
  ├── 注册工具
  ├── 配置内存
  └── 调用 agent.Run(ctx, &RunRequest{...})
      或便捷函数 RunText(ctx, agentID, input)
```

### 独立部署

```
配置文件 ──→ Service 启动 ──→ HTTP Server
                │
                ├── 加载 Agent 定义
                ├── 注册工具
                ├── 初始化 MCP Client/Server
                └── 监听 HTTP 请求
```

## 2. Service

### Config

```go
type Config struct {
    Addr string  // 监听地址，如 ":8080"，":0" 表示随机端口
}
```

### Option

```go
func WithToolRegistry(r tool.ToolRegistry) Option
func WithMaxRequestSize(n int64) Option       // 默认 4MB
func WithMaxTasks(n int) Option               // 默认 1000
func WithHeartbeatInterval(seconds int) Option // SSE 心跳间隔，默认 15s
```

### Service 方法

```go
func New(cfg Config, opts ...Option) *Service
```

| 方法             | 说明                                   |
| ---------------- | -------------------------------------- |
| `RegisterAgent`  | 注册 Agent 到服务                      |
| `Handler`        | 返回 HTTP Handler（用于测试）          |
| `Start`          | 启动 HTTP 服务                         |
| `Shutdown`       | 优雅关停                               |
| `ListenAddr`     | 获取实际监听地址                       |

## 3. HTTP 接口

所有请求通过 `requestIDMiddleware` 注入/透传 `X-Request-ID`。

| 方法   | 路径                        | 说明                               |
| ------ | --------------------------- | ---------------------------------- |
| GET    | /v1/health                  | 健康检查                           |
| GET    | /v1/agents                  | 列出所有 Agent（按 ID 排序）       |
| GET    | /v1/agents/{id}             | 获取 Agent 详情                    |
| POST   | /v1/agents/{id}/run         | 同步执行 Agent                     |
| POST   | /v1/agents/{id}/stream      | 流式执行 Agent（SSE）              |
| POST   | /v1/agents/{id}/async       | 异步执行 Agent，返回 202 + taskID  |
| GET    | /v1/tools                   | 列出所有已注册工具                 |
| GET    | /v1/tasks/{taskID}          | 查询异步任务状态和结果             |
| POST   | /v1/tasks/{taskID}/cancel   | 取消异步任务                       |

> **异步模式**：`POST /v1/agents/{id}/async` 立即返回 `202 Accepted` 和 `taskID`，客户端通过 `GET /v1/tasks/{taskID}` 轮询结果。

## 4. 异步任务管理

### TaskStatus

```go
const (
    TaskStatusPending   = "pending"
    TaskStatusRunning   = "running"
    TaskStatusCompleted = "completed"
    TaskStatusFailed    = "failed"
    TaskStatusCancelled = "cancelled"
)
```

### Task

```go
type Task struct {
    ID        string              `json:"id"`
    AgentID   string              `json:"agent_id"`
    Status    TaskStatus          `json:"status"`
    Response  *schema.RunResponse `json:"response,omitempty"`
    Error     string              `json:"error,omitempty"`
    CreatedAt time.Time           `json:"created_at"`
    UpdatedAt time.Time           `json:"updated_at"`
}
```

### TaskStore

内存任务存储，支持容量上限和淘汰策略（淘汰最老的已完成/失败任务）。

```go
func NewTaskStore(maxTasks int) *TaskStore
```

| 方法           | 说明                                   |
| -------------- | -------------------------------------- |
| `Create`       | 创建 pending 任务                      |
| `Get`          | 获取任务（返回副本）                   |
| `UpdateStatus` | 更新任务状态                           |
| `SetResult`    | 设置任务结果或错误                     |
| `Cancel`       | 取消 pending/running 任务              |

## 5. 配置体系

| 配置项           | 类型     | 说明                                   |
| ---------------- | -------- | -------------------------------------- |
| Server.Addr      | string   | 监听地址                               |
| Agents           | []Agent  | Agent 定义列表                         |
| Tools            | []Tool   | 全局工具列表                           |
| Models           | []ModelConfig     | 模型配置列表                  |
| Models[].Name    | string            | 模型标识名                    |
| Models[].APIKey  | string            | API Key（支持环境变量引用）   |
| Models[].BaseURL | string            | API 基础地址                  |
| Models[].Protocol| string            | 协议类型：openai / anthropic  |
| Compose          | ComposeConfig     | 多模型调度配置                |
| Compose.Strategy | string            | 调度策略：failover / random / weighted |
| Compose.Models   | []ComposeModelRef | 参与调度的模型引用及权重      |
| MCP.Clients      | []MCPClientConfig | MCP 客户端连接配置（含认证）  |
| MCP.Server       | MCPServerConfig   | MCP 服务端配置                |
| MCP.Server.Auth  | AuthConfig        | MCP 服务端认证配置            |
| Memory.Store     | StoreConfig       | 持久化存储配置                |
| Guard.Input      | []GuardConfig     | 输入 Guard 链配置              |
| Guard.Output     | []GuardConfig     | 输出 Guard 链配置              |
| Token.DailyLimit | int               | 全局每日 token 上限           |
| Token.PerMinute  | int               | 全局每分钟 token 速率限制     |
| LLM.Middlewares  | []string          | 启用的 LLM 中间件列表         |
| Hooks            | []string          | 启用的 Hook 类型               |
