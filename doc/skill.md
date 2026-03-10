# Skill 系统设计文档

## 1. 概述

### 1.1 背景

Agent Skills 是 Anthropic 于 2025 年 10 月提出、2025 年 12 月捐赠给 Linux 基金会旗下 Agentic AI Foundation (AAIF) 的开放标准。Microsoft、OpenAI、Atlassian、Figma、Cursor、GitHub 等 26+ 平台已采纳。Skill 为 AI Agent 提供结构化的可复用领域专业知识包，是 Agent 生态的重要补充。

### 1.2 Skill 与 Tool 的区别

| 维度       | Tool                       | Skill                                    |
| ---------- | -------------------------- | ---------------------------------------- |
| 本质       | 可执行代码（函数/API）     | 结构化指令 + 可选脚本/资源               |
| 执行方式   | 确定性 API 调用            | 模型解读指令并遵循                       |
| 作用层     | 执行层（调用函数返回结果） | 上下文层（注入 Prompt 引导行为）         |
| 粒度       | 单一函数                   | 打包的领域专家知识（指令+脚本+参考资料） |
| 可靠性     | 确定性输入输出 Schema      | 取决于模型对指令的理解和遵循             |
| 加载方式   | 注册后常驻                 | 按需激活，渐进式披露                     |

**核心洞察**：Skill 与 Tool 正交——Skill 在 Prompt/上下文层面操作，Tool 在执行层面操作。Skill 可以声明它需要使用哪些 Tool（通过 `allowed_tools`），但 Skill 本身是塑造 Agent 行为的指令，Tool 是执行原语。

### 1.3 设计目标

- 兼容 Agent Skills 开放标准（agentskills.io）规范
- 支持 Skill 的发现、注册、激活、执行、卸载完整生命周期
- 渐进式上下文加载，避免上下文窗口浪费
- 与 vagent 现有 Tool、Agent、Guard、Memory 体系无缝集成
- 安全优先，支持工具白名单、沙箱执行、权限控制

---

## 2. Agent Skills 开放标准

### 2.1 目录结构

```
my-skill/
  SKILL.md          # 必需：YAML frontmatter + Markdown 指令
  scripts/          # 可选：可执行自动化脚本
  references/       # 可选：按需加载的参考文档
  assets/           # 可选：模板和静态资源
```

### 2.2 SKILL.md 格式

```markdown
---
name: pdf-processing
description: Process and analyze PDF documents
license: Apache-2.0
allowed_tools:
  - bash
  - read_file
metadata:
  author: example
  version: 1.0.0
---

## Instructions
[分步指导、示例、边界情况处理...]
```

### 2.3 规范约束

| 约束               | 说明                                                 |
| ------------------ | ---------------------------------------------------- |
| 命名规则           | 目录名须与 `name` 字段一致，仅允许小写字母、数字、连字符 |
| 必填字段           | `name` 和 `description` 为必填 frontmatter 字段      |
| 长度限制           | SKILL.md 建议不超过 500 行                           |
| 加载策略           | SKILL.md 正文在激活时全量加载；scripts/references/assets 按需加载 |
| 路径规则           | 使用相对于 Skill 根目录的路径，保持一级深度           |

---

## 3. 核心模型

### 3.1 Def — Skill 定义

```go
// Def describes a skill that can be discovered, registered and activated.
type Def struct {
    Name         string            `json:"name"`
    Description  string            `json:"description"`
    License      string            `json:"license,omitempty"`
    AllowedTools []string          `json:"allowed_tools,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
    Instructions string            `json:"instructions"`        // SKILL.md 正文
    BasePath     string            `json:"base_path,omitempty"` // Skill 目录路径
    Resources    []Resource        `json:"resources,omitempty"` // 关联资源列表
}
```

| 字段           | 类型                | 说明                                   |
| -------------- | ------------------- | -------------------------------------- |
| Name           | string              | Skill 唯一标识（小写+连字符）          |
| Description    | string              | Skill 功能描述，用于匹配和展示         |
| License        | string              | 许可证类型                             |
| AllowedTools   | []string            | 允许使用的工具白名单（空表示不限制）   |
| Metadata       | map[string]string   | 扩展元数据（author、version 等）       |
| Instructions   | string              | 解析后的 Markdown 指令正文             |
| BasePath       | string              | Skill 目录根路径，用于定位 scripts/references/assets |
| Resources      | []Resource          | 扫描发现的资源文件列表                 |

### 3.2 Resource — 资源引用

```go
// Resource represents a loadable resource within a skill.
type Resource struct {
    Type    string `json:"type"` // "script", "reference", "asset"
    Name    string `json:"name"`
    Path    string `json:"path"`
    Content string `json:"content,omitempty"` // LoadResource 时填充
}
```

### 3.3 Activation — 激活状态

```go
// Activation tracks the activation state of a skill within a session.
type Activation struct {
    SkillName   string
    SessionID   string
    ActivatedAt time.Time
    def         *Def // unexported，防止外部修改 registry 中的原始数据
}

// SkillDef returns a copy of the skill definition.
func (a *Activation) SkillDef() Def
```

**设计要点**：`def` 字段为 unexported，`Activate` 时对 registry 中的 `*Def` 做值拷贝存储，通过 `SkillDef()` 方法返回副本，防止外部代码修改 registry 中的原始数据，保证线程安全。

---

## 4. 核心接口

### 4.1 Loader — 加载器

```go
// Loader discovers and loads skill definitions from the filesystem.
type Loader interface {
    Load(ctx context.Context, path string) (*Def, error)
    Discover(ctx context.Context, dir string) ([]*Def, error)
}
```

**实现**：`FileLoader` — 从文件系统加载，解析 SKILL.md 的 YAML frontmatter 和 Markdown 正文，自动扫描 scripts/、references/、assets/ 子目录中的资源文件。

### 4.2 Registry — 注册表

```go
// Registry manages registered skill definitions.
type Registry interface {
    Register(def *Def) error
    Unregister(name string)
    Get(name string) (*Def, bool)
    List() []*Def
    Match(query string) []*Def
}
```

**实现**：`InMemoryRegistry` — 线程安全的内存注册表，通过 `WithValidator(v Validator)` 选项配置注册时的校验器。

- `Register`：校验 def 非 nil、通过 Validator 校验、检查名称唯一性
- `Unregister`：静默移除，不存在时不报错
- `Match`：按查询词对 Name + Description 做 AND 语义的模糊匹配（大小写无关）

### 4.3 Manager — 管理器

```go
// Manager manages skill activations per session.
type Manager interface {
    Activate(ctx context.Context, name string, sessionID string) (*Activation, error)
    Deactivate(ctx context.Context, name string, sessionID string) error
    ActiveSkills(sessionID string) []*Activation
    ClearSession(ctx context.Context, sessionID string)
    LoadResource(ctx context.Context, sessionID string, skillName string,
        resourceType string, resourceName string) (*Resource, error)
}
```

**实现**：`InMemoryManager` — 线程安全的内存管理器。

- `Activate`：从 registry 获取 Def 并做值拷贝，检查重复激活，dispatch `EventSkillActivate` 事件
- `Deactivate`：从激活列表移除，dispatch `EventSkillDeactivate` 事件
- `ClearSession`：一次性清除会话的所有激活，逐个 dispatch deactivate 事件，解决会话结束时的内存泄漏问题
- `LoadResource`：从磁盘读取资源文件内容（无缓存，每次直接读取），dispatch `EventSkillResourceLoad` 事件
- 通过 `WithEventDispatcher(d EventDispatcher)` 选项配置事件回调

### 4.4 Validator — 校验器

```go
// Validator validates a Def before registration.
type Validator interface {
    Validate(def *Def) error
}
```

**内建校验器**（均含 nil 防御检查）：
- `NameValidator`：命名规范校验（小写字母+数字+连字符，正则 `^[a-z0-9]+(-[a-z0-9]+)*$`）
- `SizeValidator`：指令长度限制（最大 500 行）
- `StructureValidator`：目录结构合规（仅允许 scripts/、references/、assets/、SKILL.md）
- `CompositeValidator`：链式组合多个校验器，返回第一个错误
- `DefaultValidator()`：返回 NameValidator + SizeValidator + StructureValidator 组合

---

## 5. Skill 生命周期

```
发现 ──→ 注册 ──→ 验证 ──→ 激活 ──→ 执行 ──→ 卸载/清理
                    │                   │
                    │ frontmatter 校验   │ 指令注入 SystemPrompt
                    │ 工具白名单验证     │ 脚本/资源按需加载
                    │ 目录结构检查       │ Guard 安全检查
```

### 5.1 发现 (Discovery)

`Loader.Discover(ctx, dir)` 扫描指定目录，查找包含 `SKILL.md` 的子目录。支持：
- 本地文件系统目录扫描
- 项目内 `.skills/` 约定目录
- 可扩展支持远程仓库（未来）

### 5.2 注册 (Registration)

`Registry.Register(def)` 将 Def 加入注册表：
- 校验 def 非 nil
- 通过 Validator 链校验（如已配置）
- 检查名称唯一性，防止冲突

### 5.3 验证 (Validation)

注册时通过 Validator 链执行验证：
- Frontmatter 必填字段完整性
- SKILL.md 行数是否超过 500 行限制
- 目录结构合规性（scripts/、references/、assets/ 规范）

### 5.4 激活 (Activation)

`Manager.Activate()` 将 Skill 注入 Agent 上下文：
- 从 registry 获取 Def 并做值拷贝（防止外部修改）
- 检查重复激活（同一 session 同一 skill 不可重复激活）
- 记录激活状态（Activation）
- 触发 `EventSkillActivate` 事件（Hook 系统）

LLMAgent 在 Run/RunStream 时自动处理已激活的 Skill：
- 将 `Instructions` 追加到 Agent 的 SystemPrompt（包裹在 `<skill name="...">` XML 标签内）
- 根据 `AllowedTools` 过滤可用工具

### 5.5 执行 (Execution)

Agent 在 Skill 指令引导下执行任务：
- 按指令步骤操作
- 按需加载 scripts/references/assets（`LoadResource`，每次从磁盘读取）
- Guard 检查 Skill 上下文中的输入输出

### 5.6 卸载 (Deactivation)

任务完成后释放 Skill 上下文：
- `Deactivate`：从 session 激活列表中移除单个 Skill
- `ClearSession`：清除 session 的所有 Skill 激活，防止会话结束时的内存泄漏
- 触发 `EventSkillDeactivate` 事件
- 下次 Run 时 SystemPrompt 不再包含已卸载 Skill 的指令

---

## 6. 与 vagent 现有体系集成

### 6.1 架构位置

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Application Layer                          │
├─────────────────────────────────────────────────────────────────────┤
│                          Service Layer                             │
├─────────────────────────────────────────────────────────────────────┤
│                        Guardrails Layer                            │
├────────────────┬────────────────┬────────────────┬─────────────────┤
│   Agent Layer  │  Memory Layer  │  Tool Layer    │  Skill Layer    │
│  ┌───────────┐ │ ┌────────────┐ │ ┌────────────┐ │ ┌────────────┐ │
│  │ LLMAgent  │ │ │  Working   │ │ │ Tool Reg.  │ │ │ Registry   │ │
│  │ Workflow  │ │ │  Session   │ │ │ Tool Exec. │ │ │ Manager    │ │
│  │ Router    │ │ │  Store     │ │ │ Built-in   │ │ │ FileLoader │ │
│  │ Custom    │ │ └────────────┘ │ └────────────┘ │ └────────────┘ │
│  └───────────┘ │                │                │                 │
├────────────────┴────────────────┴────────────────┴─────────────────┤
│                         MCP / Hook Layer                           │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.2 与 Agent 集成

**LLMAgent**：Skill 的核心消费者。通过 `llmagent.WithSkillManager(m)` 配置。

```go
// LLMAgent 通过 Option 注入 skill.Manager
a := llmagent.New(cfg,
    llmagent.WithSkillManager(manager),
    llmagent.WithChatCompleter(cc),
    llmagent.WithToolRegistry(toolReg),
)
```

在 `Run`/`RunStream` 执行时，LLMAgent 自动：
1. 调用 `injectSkillInstructions`：将所有 active skill 的 Instructions 以 `<skill name="...">` XML 标签形式追加到 SystemPrompt
2. 调用 `mergeSkillToolFilter`：根据 active skill 的 AllowedTools 过滤工具列表

**工具过滤语义**：
- 所有 active skill 均未声明 AllowedTools → 不过滤，传递所有工具
- **任一** active skill 未声明 AllowedTools（表示无限制）→ 不过滤，传递所有工具
- 所有 active skill 均声明了 AllowedTools → 取所有 skill 的 AllowedTools 并集
- 若存在 request-level toolFilter → 与 skill 并集做交集

**RouterAgent**：可根据 Skill 匹配结果路由请求。RouteFunc 可参考 Registry.Match() 选择最合适的 Agent + Skill 组合。

**WorkflowAgent**：DAG 节点可在不同步骤激活/卸载不同 Skill，实现渐进式专家切换。

### 6.3 与 Tool 集成

Skill 通过 `allowed_tools` 声明可使用的工具。LLMAgent 在 Run 时通过 `mergeSkillToolFilter` 方法将 skill 的工具白名单与 request-level filter 合并，最终传入 `prepareAITools` 进行过滤。`prepareAITools` 本身保持纯函数签名 `(filter []string) []aimodel.Tool`，不依赖 session 状态。

### 6.4 与 Memory 集成

- **Session Memory**：记录 Skill 激活历史，支持跨 Run 保持 Skill 状态
- **Working Memory**：Skill 指令在 Working Memory 中占用空间，需纳入 ContextCompressor 管理
- **Persistent Store**：可存储 Skill 执行效果评估，用于优化后续 Skill 选择

### 6.5 与 Guard 集成

Skill 输入输出同样经过 Guard 检查链：
- InputGuard：检查 Skill 激活请求的合法性
- OutputGuard：检查 Skill 引导下生成内容的安全性
- 可通过 StructureValidator 验证 Skill 目录结构合规性

### 6.6 与 Hook 集成

通过 `EventDispatcher` 回调函数派发事件（避免 skill 包直接依赖 hook 包）：

| 事件类型               | 说明                  | 数据                              |
| ---------------------- | --------------------- | --------------------------------- |
| `skill_discover`       | Skill 发现            | 目录路径、发现数量                |
| `skill_activate`       | Skill 激活            | SkillName、SessionID              |
| `skill_deactivate`     | Skill 卸载            | SkillName、SessionID              |
| `skill_resource_load`  | 资源按需加载          | SkillName、ResourceType、ResourceName |

### 6.7 与 Service 集成

Service 通过两种方式配置 Skill 系统：

- `service.WithSkillDir(dir)` — 在 `Start` 时自动发现并注册目录下的 Skill，内部创建 Registry 和 Manager
- `service.WithSkillManager(m)` — 直接注入外部创建的 Manager

**互斥约束**：若同时设置 `WithSkillManager` 和 `WithSkillDir`，`discoverSkills` 会跳过自动发现，避免外部 Manager 与内部 Registry 不一致。

---

## 7. 安全设计

### 7.1 威胁分析

> 行业审计显示 41.7% 的 Skill 包含安全漏洞，82.4% 的 LLM 会在 peer agent 请求下执行恶意工具调用。

| 威胁               | 风险等级 | 缓解措施                             |
| ------------------ | -------- | ------------------------------------ |
| 恶意 Skill 注入    | 高       | 来源验证、签名校验                   |
| 工具越权调用       | 高       | allowed_tools 白名单强制执行         |
| 文件系统逃逸       | 高       | 沙箱执行、路径限制、符号链接防护     |
| Prompt 注入        | 中       | 现有 PromptInjectionGuard 覆盖       |
| 上下文窗口耗尽     | 中       | 500 行限制、上下文压缩器管理         |
| 网络外联           | 中       | 脚本网络访问控制                     |
| 配置篡改           | 中       | 禁止 Skill 修改 Hook、配置、其他 Skill |
| Registry 数据篡改  | 中       | Activation 存储 Def 副本，unexported 字段 |

### 7.2 安全控制

```go
// Validator validates skill definitions before registration.
type Validator interface {
    Validate(def *Def) error
}
```

**内建校验器**（均含 nil 防御检查）：
- `NameValidator`：命名规范校验
- `SizeValidator`：指令长度限制（MaxInstructionLines = 500）
- `StructureValidator`：目录结构合规（仅允许 scripts/、references/、assets/、SKILL.md）
- `CompositeValidator`：链式组合，`DefaultValidator()` 返回以上三者的组合

### 7.3 脚本沙箱

Skill 脚本执行通过沙箱环境隔离（未来扩展）：
- 限制文件系统访问范围（仅 Skill 目录和工作目录）
- 限制网络访问（可配置）
- 执行超时控制
- 资源使用限制（CPU、内存）

---

## 8. 模块结构

```
vagent/
├── skill/                    # Skill 系统
│   ├── skill.go              # Def、Resource、Activation 模型定义
│   ├── skill_test.go         # 模型单元测试
│   ├── loader.go             # Loader 接口与 FileLoader 实现
│   ├── loader_test.go        # 加载器单元测试
│   ├── registry.go           # Registry 接口与 InMemoryRegistry 实现
│   ├── registry_test.go      # 注册表单元测试
│   ├── manager.go            # Manager 接口与 InMemoryManager 实现
│   ├── manager_test.go       # 管理器单元测试
│   ├── validator.go          # Validator 接口与内建校验器
│   ├── validator_test.go     # 校验器单元测试
│   └── testdata/             # 测试用 Skill 目录
│       ├── valid-skill/      # 完整 Skill 示例（含 scripts/、references/）
│       ├── minimal-skill/    # 最小 Skill 示例
│       └── bad-name/         # 命名不合规 Skill（测试 Discover 不做校验）
├── schema/
│   └── event.go              # Skill 相关事件类型和数据结构
├── agent/llmagent/
│   └── llm.go                # WithSkillManager、injectSkillInstructions、mergeSkillToolFilter
├── service/
│   └── service.go            # WithSkillDir、WithSkillManager、discoverSkills
└── integrations/skill_tests/ # Skill 集成测试
    └── skill_test.go
```

### 包依赖关系

```
agent/llmagent ──→ skill (Manager 接口)
skill ──→ schema (Event 类型)
service ──→ skill (Registry、Manager、FileLoader)
```

> 注意：skill 包不依赖 tool 包。工具过滤逻辑在 agent/llmagent 中实现，skill 包仅声明 AllowedTools 字段。

---

## 9. 使用示例

### 9.1 基础用法

```go
// 1. 创建 Skill 加载器和注册表
loader := &skill.FileLoader{}
registry := skill.NewRegistry(skill.WithValidator(skill.DefaultValidator()))

// 2. 发现并注册项目 Skills
skills, _ := loader.Discover(ctx, ".skills/")
for _, s := range skills {
    registry.Register(s)
}

// 3. 创建 Skill 管理器
manager := skill.NewManager(registry)

// 4. 创建 Agent 并关联 Skill 管理器
a := llmagent.New(cfg,
    llmagent.WithSkillManager(manager),
    llmagent.WithChatCompleter(cc),
    llmagent.WithToolRegistry(toolReg),
)

// 5. 激活 Skill（在 Run 前或 Run 过程中动态激活）
manager.Activate(ctx, "pdf-processing", sessionID)

// 6. 执行 Agent
resp, _ := a.Run(ctx, req)

// 7. 会话结束时清理所有 Skill 激活
manager.ClearSession(ctx, sessionID)
```

### 9.2 动态 Skill 选择

```go
// RouterAgent 根据任务描述匹配 Skill
routeFunc := func(ctx context.Context, req *schema.RunRequest) (*routeragent.RouteResult, error) {
    query := req.Messages[len(req.Messages)-1].Content.Text()
    matched := registry.Match(query)
    if len(matched) > 0 {
        manager.Activate(ctx, matched[0].Name, req.SessionID)
    }
    return &routeragent.RouteResult{Agent: targetAgent}, nil
}
```

### 9.3 Workflow 中的 Skill 切换

```go
// DAG 节点间切换不同 Skill
nodes := []orchestrate.Node{
    {ID: "analyze", Runner: analysisAgent, /* 激活 data-analysis skill */},
    {ID: "visualize", Runner: vizAgent, Deps: []string{"analyze"}, /* 激活 visualization skill */},
    {ID: "report", Runner: reportAgent, Deps: []string{"visualize"}, /* 激活 report-writing skill */},
}
```

### 9.4 Service 自动发现

```go
// Service 启动时自动发现 Skill
svc := service.New(service.Config{Addr: ":8080"},
    service.WithSkillDir("./skills"),
)
svc.Start(ctx)

// 或注入外部管理器（此时跳过自动发现）
svc := service.New(service.Config{Addr: ":8080"},
    service.WithSkillManager(myManager),
)
```

---

## 10. Skill 组合模式

| 模式         | 说明                                       | vagent 对应                  |
| ------------ | ------------------------------------------ | ---------------------------- |
| 顺序链       | Skill 按序激活，前一个的输出作为后一个输入  | WorkflowAgent 顺序模式       |
| 并行执行     | 多个 Skill 同时激活处理独立子任务           | DAG 并行节点                 |
| 层级嵌套     | 父 Skill 分解目标为子 Skill                | 嵌套 Agent + Skill 组合      |
| 路由分派     | 根据意图选择合适的 Skill                   | RouterAgent + Match()        |
| 渐进式披露   | 按需加载 Skill，避免上下文浪费             | Manager.Activate/Deactivate  |
| 生成-评审    | 一个 Skill 生成内容，另一个验证             | 循环节点 + 双 Skill 切换     |

---

## 11. 参考资料

- [Agent Skills 规范](https://agentskills.io/specification)
- [Agent Skills GitHub 仓库](https://github.com/agentskills/agentskills)
- [Anthropic Agent Skills 文档](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)
- [Microsoft Semantic Kernel Plugins](https://learn.microsoft.com/en-us/semantic-kernel/concepts/plugins/)
- [LangChain Skills](https://blog.langchain.com/langchain-skills/)
- [OpenAI Agent Skills (Codex)](https://developers.openai.com/codex/skills/)
