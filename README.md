# vage

[![Build](https://github.com/vogo/vage/actions/workflows/build.yml/badge.svg)](https://github.com/vogo/vage/actions/workflows/build.yml)
[![codecov](https://codecov.io/gh/vogo/vage/branch/main/graph/badge.svg)](https://codecov.io/gh/vogo/vage)


A Go framework for building LLM-based intelligent agent systems.

## Features

- **Composable Agents** — TaskAgent (ReAct tool-calling), RouterAgent (routing), WorkflowAgent (DAG orchestration), and CustomAgent (user-defined)
- **DAG Orchestration** — Parallel execution, loops, conditionals, compensation (Saga), checkpointing, backpressure, priority scheduling
- **Three-Level Memory** — Working (request) → Session (conversation) → Store (persistent), with context compression and token budgets
- **Security Guardrails** — Prompt injection, content filter, PII, topic, length, and custom guards
- **LLM Middleware** — Decorator chain: logging, circuit breaker, rate limiting, retry, timeout, cache, metrics
- **Tool System** — Local functions, MCP remote tools, agent-as-tool, built-in bash tool with process isolation
- **Agent Skills** — Compatible with the [Agent Skills](https://agentskills.io) open standard
- **MCP Protocol** — Client (consume external tools) and server (expose agent capabilities)
- **Evaluation** — ExactMatch, Contains, LLMJudge, ToolCall, Latency, Cost evaluators
- **HTTP Service** — REST endpoints for sync, streaming, and async agent execution


## Installation

```bash
go get github.com/vogo/vage
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	"github.com/vogo/vage/agent"
	"github.com/vogo/vage/schema"
)

func main() {
	a := agent.New("greeter", "Greeter", "A simple greeting agent",
		func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
			return schema.TextRunResponse("Hello! How can I help you?"), nil
		},
	)

	resp, err := agent.RunText(context.Background(), a, "Hi")
	if err != nil {
		panic(err)
	}
	fmt.Println(resp)
}
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
