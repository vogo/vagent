# MCP 支持

vagent 同时支持 MCP Client 和 MCP Server 两种角色，通过 `github.com/modelcontextprotocol/go-sdk` 实现。

## 1. 概述

```
┌──────────────────────────────────────────────────────────┐
│                        vagent                            │
│                                                          │
│  ┌─────────────┐                     ┌─────────────┐    │
│  │  MCP Client │                     │  MCP Server │    │
│  │  消费外部工具 │                     │  暴露 Agent  │    │
│  └──────┬──────┘                     └──────┬──────┘    │
│         │                                   │            │
└─────────┼───────────────────────────────────┼────────────┘
          │                                   │
          ▼                                   ▼
  ┌───────────────┐                  ┌────────────────┐
  │ 外部 MCP 服务  │                  │ 外部 MCP 客户端  │
  │ (文件系统、    │                  │ (IDE、CLI 等)   │
  │  数据库等)     │                  │                │
  └───────────────┘                  └────────────────┘
```

## 2. MCP Client（消费外部工具）

### 接口

```go
type Lifecycle interface {
    Connect(ctx context.Context, transport mcp.Transport) error
    Disconnect() error
    Ping(ctx context.Context) error
}

type MCPClient interface {
    tool.ExternalToolCaller  // CallTool(ctx, name, args) (ToolResult, error)
    Lifecycle
    ListTools(ctx context.Context) ([]schema.ToolDef, error)
}
```

### Client 实现

```go
type Client struct { /* internal: client, session, serverURI, mu */ }

func NewClient(serverURI string) *Client
```

| 方法          | 说明                                                       |
| ------------- | ---------------------------------------------------------- |
| `Connect`     | 通过 initialize 握手协商能力                                |
| `Disconnect`  | 关闭会话                                                    |
| `ListTools`   | 调用 tools/list 获取工具列表，转为 `schema.ToolDef`        |
| `CallTool`    | 通过 tools/call 代理执行，转为 `schema.ToolResult` 返回    |
| `Ping`        | 健康检查                                                    |

## 3. MCP Server（暴露 Agent 能力）

### 接口

```go
type MCPServer interface {
    Serve(ctx context.Context, transport mcp.Transport) error
    RegisterAgent(a agent.Agent) error
    RegisterTool(reg ToolRegistration) error
    Server() *mcp.Server
}
```

### ToolRegistration

```go
type ToolRegistration struct {
    Name        string
    Description string
    InputSchema any  // JSON Schema
    Handler     func(ctx context.Context, args map[string]any) (schema.ToolResult, error)
}
```

### Server 实现

```go
type Server struct { /* internal: server, mu */ }

func NewServer() *Server
```

| 方法            | 说明                                                     |
| --------------- | -------------------------------------------------------- |
| `Serve`         | 在指定 transport 上运行 MCP 服务（阻塞）                |
| `RegisterAgent` | 注册 Agent 为 MCP 工具（Name=ID, Description=Description）|
| `RegisterTool`  | 注册自定义工具处理器                                     |
| `Server`        | 获取底层 go-sdk Server 实例                              |

## 4. 传输层

| 传输方式        | 说明                                         | 适用场景           |
| --------------- | -------------------------------------------- | ------------------ |
| stdio           | 标准输入输出，JSON-RPC 消息                  | 子进程模式         |
| SSE             | Server-Sent Events + HTTP POST               | Web 集成           |
| Streamable HTTP | 单 HTTP 端点，支持流式响应                   | 现代 HTTP 客户端   |

## 5. 认证与授权

MCP 通信涉及外部系统交互，支持认证授权机制。

### Authenticator 接口

```go
type Authenticator interface {
    Authenticate(ctx context.Context, credentials Credentials) (*Identity, error)
}
```

### Authorizer 接口

```go
type Authorizer interface {
    Authorize(ctx context.Context, identity *Identity, action string) (bool, error)
}
```

### 数据模型

**Credentials**

| 字段      | 类型   | 说明                                   |
| --------- | ------ | -------------------------------------- |
| Type      | string | 凭证类型：api_key / bearer / mtls      |
| Token     | string | 凭证内容                               |
| Metadata  | map    | 扩展字段                               |

**Identity**

| 字段      | 类型     | 说明                     |
| --------- | -------- | ------------------------ |
| ID        | string   | 身份标识                 |
| Name      | string   | 身份名称                 |
| Roles     | []string | 角色列表                 |
| Metadata  | map      | 扩展字段                 |

### 内建认证方式

| 认证方式         | 说明                                             | 适用场景           |
| ---------------- | ------------------------------------------------ | ------------------ |
| API Key          | 通过请求头或查询参数传递静态密钥                 | 简单部署           |
| Bearer Token     | OAuth2 / JWT Token 验证                          | 企业集成           |
| mTLS             | 双向 TLS 证书认证                                | 高安全要求         |
| NoAuth           | 不认证（仅限开发/测试环境）                      | 本地开发           |

### 认证流程

```
外部 MCP 客户端请求
       │
       ▼
┌──────────────────┐
│  Transport 层    │
│  提取 Credentials│
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  Authenticator   │ ──→ 失败 ──→ 返回 401
└──────┬───────────┘
       │ 成功
       ▼
┌──────────────────┐
│  Authorizer      │ ──→ 拒绝 ──→ 返回 403
└──────┬───────────┘
       │ 允许
       ▼
   处理 MCP 请求
```

### Client 凭证配置

| 配置项                | 类型   | 说明                             |
| --------------------- | ------ | -------------------------------- |
| MCP.Clients[].Auth.Type   | string | 认证类型                         |
| MCP.Clients[].Auth.Token  | string | 凭证内容（支持环境变量引用）     |
| MCP.Clients[].Auth.Header | string | 自定义认证头名称（默认 Authorization）|
