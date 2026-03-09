# vagent 架构文档

## 1. 项目概述

### 定位

vagent 是一个 Go 语言 Agent 开发框架，用于构建基于大语言模型（LLM）的智能代理系统。支持嵌入现有系统或独立部署。

### 目标

- 提供灵活的 Agent 抽象，支持多种 Agent 类型和编排模式
- 通过 `github.com/vogo/aimodel` 统一调用多家大模型（OpenAI、Anthropic、Gemini 等）
- 支持 MCP（Model Context Protocol）协议，实现工具互操作
- 最小外部依赖（仅依赖 `aimodel` 和 `github.com/modelcontextprotocol/go-sdk`）
- 可嵌入、可独立部署

### 设计原则

| 原则         | 说明                                           |
| ------------ | ---------------------------------------------- |
| 组合优于继承 | 通过接口组合构建复杂 Agent，避免深层继承       |
| 最小依赖     | 仅依赖 `aimodel` 和 `mcp/go-sdk`，不引入 ORM、Web 框架等 |
| 上下文驱动   | 所有操作通过 `context.Context` 传递，支持取消  |
| 可观测       | 内建 Hook 机制，支持事件追踪和轨迹采集         |
| 协议兼容     | 遵循 MCP 协议规范，与外部工具生态互通          |
| 安全优先     | 内建 Guardrails 机制，输入输出安全检查前置     |

---

## 2. 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Application Layer                          │
│                  (嵌入式调用 / HTTP API / CLI)                     │
├─────────────────────────────────────────────────────────────────────┤
│                          Service Layer                             │
│              (配置管理 / HTTP Server / 生命周期管理)               │
├────────────────────────────────────────────────────────────────────┤
│                        Guardrails Layer                            │
│            (InputGuard / OutputGuard / 安全检查链)                 │
├────────────────┬────────────────┬───────────────────────────────────┤
│   Agent Layer  │  Memory Layer  │         Tool Layer               │
│  ┌───────────┐ │ ┌────────────┐ │  ┌────────────┐ ┌─────────────┐ │
│  │ LLMAgent  │ │ │  Working   │ │  │ Tool Reg.  │ │ MCP Client  │ │
│  │ Workflow  │ │ │  Session   │ │  │ Tool Exec. │ │ MCP Server  │ │
│  │ Router    │ │ │  Store     │ │  │ Built-in   │ │ Transport   │ │
│  │ Custom    │ │ └────────────┘ │  └────────────┘ └─────────────┘ │
│  └───────────┘ │                │                                  │
├────────────────┴────────────────┴───────────────────────────────────┤
│                       Hook / Observability                         │
│              (Hook / 轨迹采集 / 日志 / 指标)                       │
├─────────────────────────────────────────────────────────────────────┤
│                     外部依赖 (aimodel / mcp go-sdk)                │
│  (ChatCompleter / ComposeClient / Stream / Protocol / MCP)         │
└─────────────────────────────────────────────────────────────────────┘
```

### 层次说明

| 层级           | 职责                                                 |
| -------------- | ---------------------------------------------------- |
| Application    | 对外入口：嵌入式 API 调用、HTTP 接口、命令行工具     |
| Service        | 配置加载、Server 启停、Agent 注册与生命周期管理      |
| Guardrails     | 安全检查层，输入输出过滤、内容审核、注入防护         |
| Agent          | 核心智能体逻辑，包含多种 Agent 类型和编排策略        |
| Memory         | 上下文管理，三级内存架构                             |
| Tool           | 工具定义、注册、执行，MCP 协议集成                   |
| Hook           | 横切关注点，事件驱动的可观测性                       |
| aimodel        | 底层大模型调用，提供 ChatCompleter 统一接口、Protocol 协议分发、ComposeClient 多模型调度 |
| mcp/go-sdk     | MCP 协议 Go SDK，提供 MCP 协议基础实现               |

---

## 3. 模块结构

```
vagent/
├── agent/          # Agent 核心定义与各类型实现
├── guard/          # Guardrails 安全检查（输入/输出过滤）
├── prompt/         # Prompt 模板引擎（变量插值/条件渲染/版本管理）
├── largemodel/     # LLM 调用中间件链（重试/缓存/限流/降级）
├── memory/         # 三级内存系统
├── tool/           # 工具注册与执行
├── mcp/            # MCP 协议支持（Client + Server）
│   ├── client/     # MCP Client — 消费外部工具
│   └── server/     # MCP Server — 暴露 Agent 能力
├── hook/           # Hook 与可观测性
├── orchestrate/    # 编排引擎（顺序/并行/条件）
├── service/        # 服务层（配置/HTTP/生命周期）
├── eval/           # 评估框架（质量回归测试/评分）
├── schema/         # 共享数据模型定义
└── doc/            # 文档
```

### 包依赖关系

```
service ──→ guard ──→ agent ──→ memory
  │                     │──→ tool ──→ mcp/client
  │                     │──→ hook
  │                     │──→ orchestrate
  │                     │──→ prompt
  │                     └──→ largemodel ──→ aimodel
  │
  └──→ mcp/server ──→ agent

agent, tool, largemodel ──→ aimodel (ChatCompleter / ComposeClient)
mcp/* ──→ github.com/modelcontextprotocol/go-sdk (外部)
schema ←── (所有包共享)
```

### 各包职责

| 包            | 职责                                                   | 详细设计                         |
| ------------- | ------------------------------------------------------ | -------------------------------- |
| `schema`      | 共享模型定义：Message、Event、ToolDef 等               | [schema.md](schema.md)          |
| `guard`       | Guardrails 安全层：输入/输出检查链、内容审核、注入防护 | [guard.md](guard.md)            |
| `prompt`      | Prompt 模板引擎：变量插值、条件渲染、模板版本管理      | —                                |
| `largemodel`  | LLM 调用中间件链：重试、缓存、限流、降级、日志         | [largemodel.md](largemodel.md)  |
| `agent`       | Agent 接口、基础实现、各类型 Agent                     | [agent.md](agent.md)            |
| `memory`      | Working/Session/Store 三级内存接口与实现               | [memory.md](memory.md)          |
| `tool`        | 工具注册表、工具执行器、Agent-as-Tool                  | [tool.md](tool.md)              |
| `mcp/client`  | MCP 客户端，发现并调用外部 MCP 服务暴露的工具         | [mcp.md](mcp.md)                |
| `mcp/server`  | MCP 服务端，将 Agent 能力通过 MCP 协议暴露             | [mcp.md](mcp.md)                |
| `hook`        | Hook 定义与分发，支持轨迹采集                          | [hook.md](hook.md)              |
| `orchestrate` | 多 Agent 编排引擎，支持顺序/并行/条件模式              | [orchestrate.md](orchestrate.md)|
| `service`     | 服务配置、HTTP 路由、Server 启停管理                   | [service.md](service.md)        |
| `eval`        | 评估框架：EvalCase/EvalResult、内建评估器              | [eval.md](eval.md)              |

---

## 4. 核心模型定义

详细设计参见 [schema.md](schema.md)。

核心类型概览：

| 类型           | 说明                                                     |
| -------------- | -------------------------------------------------------- |
| `Message`      | 对话消息，内嵌 `aimodel.Message`，增加 AgentID/Timestamp/Metadata |
| `ContentPart`  | 内容片段，支持 text/json/image/file                      |
| `ToolDef`      | 工具定义，支持 local/mcp/agent 三种来源                  |
| `ToolResult`   | 工具执行结果                                             |
| `RunRequest`   | Agent 执行请求                                           |
| `RunResponse`  | Agent 执行响应，含 StopReason                            |
| `RunOptions`   | 运行时参数覆盖                                           |
| `Event`        | 系统事件，用于 Hook 和流式输出                           |
| `RunStream`    | 流式事件流，pull 模式                                    |

---

## 5. 接口规范

### 5.1 Agent 接口

详细设计参见 [agent.md](agent.md)。

### 5.2 Memory 接口

详细设计参见 [memory.md](memory.md)。

### 5.3 ToolExecutor / ToolRegistry 接口

详细设计参见 [tool.md](tool.md)。

### 5.4 Guard 接口

详细设计参见 [guard.md](guard.md)。

### 5.5 Hook 接口

详细设计参见 [hook.md](hook.md)。

### 5.6 编排接口

详细设计参见 [orchestrate.md](orchestrate.md)。

### 5.7 MCP 接口

详细设计参见 [mcp.md](mcp.md)。

### 5.8 PromptTemplate 接口

Prompt 模板接口，支持变量插值和条件渲染。

| 方法      | 参数                          | 返回值          | 说明                             |
| --------- | ----------------------------- | --------------- | -------------------------------- |
| Render    | ctx, vars map[string]any      | (string, error) | 渲染模板，返回最终 prompt 字符串 |
| Name      | —                             | string          | 模板名称标识                     |
| Version   | —                             | string          | 模板版本号                       |

**PromptTemplate 模型**

| 字段      | 类型     | 说明                                         |
| --------- | -------- | -------------------------------------------- |
| Name      | string   | 模板名称                                     |
| Version   | string   | 版本号（语义化版本）                         |
| Template  | string   | 模板内容，支持 `{{.VarName}}` 变量插值       |
| Variables | []VarDef | 变量定义列表（名称、类型、默认值、是否必填） |
| Metadata  | map      | 扩展元数据                                   |

**VarDef**

| 字段      | 类型   | 说明                     |
| --------- | ------ | ------------------------ |
| Name      | string | 变量名                   |
| Type      | string | 类型：string / int / bool|
| Default   | any    | 默认值                   |
| Required  | bool   | 是否必填                 |
| Desc      | string | 变量说明                 |

> Agent 的 `SystemPrompt` 字段类型从 `string` 升级为 `PromptTemplate`。框架同时提供 `StringPrompt(s string) PromptTemplate` 便捷函数，将纯文本包装为模板，兼容简单场景。

### 5.9 LLM Middleware 接口

详细设计参见 [largemodel.md](largemodel.md)。

---

## 6. Agent 类型

详细设计参见 [agent.md](agent.md)。

四种内建 Agent 类型：LLMAgent（ReAct 式工具调用）、WorkflowAgent（顺序/DAG/循环编排）、RouterAgent（基于 RouteFunc 动态路由）、CustomAgent（委托用户函数）。所有 Agent 均通过接口组合构建，LLMAgent 原生支持流式，其余通过 RunToStream 适配。

---

## 7. 编排机制 (Orchestration)

详细设计参见 [orchestrate.md](orchestrate.md)。

编排引擎基于 DAG（有向无环图）模型，支持顺序/并行/条件分支执行、循环节点（LoopNode）、动态节点生成（DynamicSpawnNode）、检查点恢复、补偿回滚（Saga 模式）、背压控制（AIMD）、优先级调度（关键路径法）和细粒度资源管理。WorkflowAgent 使用此引擎编排多 Agent 协作。

---

## 8. 内存系统 (Memory)

详细设计参见 [memory.md](memory.md)。

内存系统采用三级架构：Working Memory（单次 Run）→ Session Memory（会话级）→ Persistent Memory（跨会话持久），通过 Promoter 和 Archiver 策略控制层级间的数据流转，通过 ContextCompressor 管理上下文窗口压缩。Manager 编排整个三级内存生命周期。

---

## 9. 工具系统 (Tools)

详细设计参见 [tool.md](tool.md)。

工具系统提供统一的工具注册表（ToolRegistry），支持本地工具、MCP 远程工具和 Agent-as-Tool 三种来源。通过 ToolExecutor 路由到对应处理器执行，结果封装为 ToolResult 回传给 LLM。

---

## 10. MCP 支持

详细设计参见 [mcp.md](mcp.md)。

vagent 同时支持 MCP Client（消费外部工具）和 MCP Server（暴露 Agent 能力），通过 `github.com/modelcontextprotocol/go-sdk` 实现，支持 stdio、SSE、Streamable HTTP 三种传输方式。

---

## 11. Hook 与可观测性

详细设计参见 [hook.md](hook.md)。

通过同步/异步 Hook 机制采集 Agent 执行轨迹，支持日志记录、指标采集、轨迹追踪。内建 Token 预算控制机制。

---

## 12. LLM 调用中间件链

详细设计参见 [largemodel.md](largemodel.md)。

在 Agent 与 aimodel 之间插入中间件层，基于装饰器模式构建。内建 Log、CircuitBreaker、RateLimit、Retry、Timeout、Cache、Metrics 七种中间件。

---

## 13. 服务层 (Service)

详细设计参见 [service.md](service.md)。

支持嵌入式和独立部署两种模式。独立部署时提供 RESTful HTTP 接口，支持同步/流式/异步三种 Agent 执行方式。

---

## 14. Guardrails 安全检查

详细设计参见 [guard.md](guard.md)。

输入/输出安全检查链，内建 PromptInjection、ContentFilter、PII、Topic、Length、Custom 六种 Guard。

---

## 15. 评估框架 (Eval)

详细设计参见 [eval.md](eval.md)。

内建评估框架，支持 ExactMatch、Contains、LLMJudge、ToolCall、Latency、Cost 六种评估器和组合评估。

---

## 16. Harness Engineering

### 模块化设计

- 每个模块（Agent、Memory、Tool、MCP）通过接口解耦
- 各模块可独立测试、独立替换
- 新增 Agent 类型只需实现 Agent 接口
- 新增工具只需实现 ToolHandler 并注册

### 验证循环

```
定义 Agent ──→ 编写工具 ──→ 测试单步 ──→ 测试编排 ──→ 端到端验证
    ▲                                                      │
    └──────────────────── 反馈调整 ◄────────────────────────┘
```

| 验证层级     | 验证内容                               | 方法                     |
| ------------ | -------------------------------------- | ------------------------ |
| 单元         | 单个工具、单个 Agent 的行为            | 标准单元测试             |
| 集成         | Agent + 工具 + 内存协作                | Mock LLM 测试            |
| 编排         | 多 Agent 编排的正确性                  | 场景测试                 |
| 端到端       | 完整 Service 的请求-响应               | 真实 LLM 调用            |

### 上下文工程

| 实践                 | 说明                                                         |
| -------------------- | ------------------------------------------------------------ |
| 系统提示词模板化     | 通过 `prompt` 包的 PromptTemplate 管理复杂提示词             |
| 上下文窗口管理       | Working Memory 自动截断过长上下文，保留关键信息              |
| 工具描述优化         | 工具的 Description 直接影响 LLM 选择准确率，需精心编写       |
| Few-shot 示例注入    | 在系统提示词中嵌入示例，引导 LLM 输出格式                   |
| 结果后处理           | 对 LLM 输出做格式校验和内容提取                             |
