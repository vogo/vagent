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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vogo/vage/agent"
	"github.com/vogo/vage/schema"
)

// ToolRegistration describes a tool to register on the MCP server.
type ToolRegistration struct {
	Name        string
	Description string
	InputSchema any
	Handler     func(ctx context.Context, args map[string]any) (schema.ToolResult, error)
}

// MCPServer is the interface for an MCP protocol server.
type MCPServer interface {
	Serve(ctx context.Context, transport mcp.Transport) error
	RegisterAgent(a agent.Agent) error
	RegisterTool(reg ToolRegistration) error
	Server() *mcp.Server
}

// Server implements MCPServer using the official go-sdk.
type Server struct {
	server *mcp.Server
	mu     sync.RWMutex
}

// Compile-time check.
var _ MCPServer = (*Server)(nil)

// NewServer creates a new MCP server.
func NewServer() *Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "vage-mcp-server",
		Version: "1.0.0",
	}, nil)
	return &Server{server: s}
}

// Server returns the underlying go-sdk Server for advanced usage.
func (s *Server) Server() *mcp.Server {
	return s.server
}

// Serve runs the server on the given transport (blocking).
func (s *Server) Serve(ctx context.Context, transport mcp.Transport) error {
	return s.server.Run(ctx, transport)
}

// RegisterAgent registers a vage Agent as an MCP tool.
// The tool name is the agent ID and the description is the agent description.
func (s *Server) RegisterAgent(a agent.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.server.AddTool(&mcp.Tool{
		Name:        a.ID(),
		Description: a.Description(),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "Input text for the agent",
				},
			},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input := ""
		if req.Params.Arguments != nil {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err == nil {
				if v, ok := args["input"]; ok {
					input = fmt.Sprintf("%v", v)
				} else {
					input = string(req.Params.Arguments)
				}
			} else {
				input = string(req.Params.Arguments)
			}
		}

		runReq := &schema.RunRequest{
			Messages: []schema.Message{schema.NewUserMessage(input)},
		}

		resp, err := a.Run(ctx, runReq)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
		}

		text := ""
		if len(resp.Messages) > 0 {
			text = resp.Messages[0].Content.Text()
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	})

	return nil
}

// RegisterTool registers a custom tool handler on the server.
func (s *Server) RegisterTool(reg ToolRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.server.AddTool(&mcp.Tool{
		Name:        reg.Name,
		Description: reg.Description,
		InputSchema: reg.InputSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				}, nil
			}
		}

		result, err := reg.Handler(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
		}

		content := make([]mcp.Content, len(result.Content))
		for i, p := range result.Content {
			content[i] = &mcp.TextContent{Text: p.Text}
		}

		return &mcp.CallToolResult{
			Content: content,
			IsError: result.IsError,
		}, nil
	})

	return nil
}
