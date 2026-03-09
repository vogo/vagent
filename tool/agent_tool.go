/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tool

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/vogo/aimodel"
	"github.com/vogo/vagent/agent"
	"github.com/vogo/vagent/schema"
)

// ArgExtractor extracts the input string from parsed tool arguments.
// The default extractor reads the "input" field; custom extractors can
// handle richer parameter schemas.
type ArgExtractor func(parsed map[string]any) (string, error)

// defaultArgExtractor extracts the "input" string field from parsed arguments.
func defaultArgExtractor(parsed map[string]any) (string, error) {
	inputVal, ok := parsed["input"]
	if !ok {
		return "", errMissingInput
	}

	input, ok := inputVal.(string)
	if !ok {
		return "", errMissingInput
	}

	return input, nil
}

// agentToolConfig holds configuration for registering an agent as a tool.
type agentToolConfig struct {
	name         string
	description  string
	parameters   any
	argExtractor ArgExtractor
}

// AgentToolOption is a functional option for configuring agent-as-tool registration.
type AgentToolOption func(*agentToolConfig)

// WithAgentToolName overrides the tool name (defaults to agent.Name()).
func WithAgentToolName(name string) AgentToolOption {
	return func(c *agentToolConfig) { c.name = name }
}

// WithAgentToolDescription overrides the tool description (defaults to agent.Description()).
func WithAgentToolDescription(desc string) AgentToolOption {
	return func(c *agentToolConfig) { c.description = desc }
}

// WithAgentToolParameters overrides the JSON Schema parameters.
// When using a custom schema, also provide WithAgentToolArgExtractor to
// match the new schema, otherwise the default extractor (which reads "input") is used.
func WithAgentToolParameters(params any) AgentToolOption {
	return func(c *agentToolConfig) { c.parameters = params }
}

// WithAgentToolArgExtractor overrides how raw JSON arguments are converted
// to the input string sent to the agent. This is useful when the parameter
// schema contains fields beyond the default "input" property.
func WithAgentToolArgExtractor(fn ArgExtractor) AgentToolOption {
	return func(c *agentToolConfig) { c.argExtractor = fn }
}

// defaultAgentToolParams returns the default JSON Schema for agent tool parameters.
// A function is used (not a package-level variable) to prevent accidental mutation of shared state.
func defaultAgentToolParams() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "The input text to send to the agent",
			},
		},
		"required": []string{"input"},
	}
}

// RegisterAgentAsTool registers an Agent as a callable tool in the registry.
func (r *Registry) RegisterAgentAsTool(ag agent.Agent, opts ...AgentToolOption) error {
	cfg := agentToolConfig{
		name:         ag.Name(),
		description:  ag.Description(),
		parameters:   defaultAgentToolParams(),
		argExtractor: defaultArgExtractor,
	}

	for _, o := range opts {
		o(&cfg)
	}

	def := schema.ToolDef{
		Name:        cfg.name,
		Description: cfg.description,
		Parameters:  cfg.parameters,
		Source:      schema.ToolSourceAgent,
		AgentID:     ag.ID(),
	}

	handler := newAgentToolHandler(ag, cfg.argExtractor)

	return r.registerIfAbsent(def, handler)
}

// agentToolError is a sentinel type for agent tool argument errors.
type agentToolError struct{ msg string }

func (e *agentToolError) Error() string { return e.msg }

var errMissingInput = &agentToolError{msg: "agent tool: 'input' field must be a non-empty string"}

// newAgentToolHandler creates a ToolHandler closure that delegates to the given agent.
//
// Error policy: agent execution errors are returned as ToolResult with IsError=true
// rather than as Go errors. This keeps the error visible to the LLM in a tool-calling
// loop so it can retry or inform the user, instead of aborting the entire chain.
func newAgentToolHandler(ag agent.Agent, extract ArgExtractor) ToolHandler {
	return func(ctx context.Context, _, args string) (schema.ToolResult, error) {
		// Parse args.
		var parsed map[string]any
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			return schema.ErrorResult("", "agent tool: invalid arguments: "+err.Error()), nil
		}

		// Extract input using the configured extractor.
		input, err := extract(parsed)
		if err != nil {
			return schema.ErrorResult("", err.Error()), nil
		}

		// Build request and call agent.
		req := schema.RunRequest{
			Messages: []schema.Message{schema.NewUserMessage(input)},
		}

		resp, err := ag.Run(ctx, &req)
		if err != nil {
			return schema.ErrorResult("", "agent tool: execution failed: "+err.Error()), nil
		}

		// Convert response: filter to assistant messages, extract text, skip empty.
		var parts []string
		for _, msg := range resp.Messages {
			if msg.Role == aimodel.RoleAssistant {
				text := msg.Content.Text()
				if text != "" {
					parts = append(parts, text)
				}
			}
		}

		return schema.TextResult("", strings.Join(parts, "\n")), nil
	}
}
