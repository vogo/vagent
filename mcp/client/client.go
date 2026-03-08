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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vogo/vagent/schema"
	"github.com/vogo/vagent/tool"
)

// Lifecycle manages the connection lifecycle of an MCP client.
type Lifecycle interface {
	Connect(ctx context.Context, transport mcp.Transport) error
	Disconnect() error
	Ping(ctx context.Context) error
}

// MCPClient combines tool calling with connection lifecycle management.
type MCPClient interface {
	tool.ExternalToolCaller
	Lifecycle
	ListTools(ctx context.Context) ([]schema.ToolDef, error)
}

// Client implements MCPClient using the official go-sdk.
type Client struct {
	client    *mcp.Client
	session   *mcp.ClientSession
	serverURI string
	mu        sync.RWMutex
}

// Compile-time check.
var _ MCPClient = (*Client)(nil)

// NewClient creates a new MCP client.
func NewClient(serverURI string) *Client {
	c := mcp.NewClient(&mcp.Implementation{
		Name:    "vagent-mcp-client",
		Version: "1.0.0",
	}, nil)
	return &Client{
		client:    c,
		serverURI: serverURI,
	}
}

// Connect establishes a connection to the server via the given transport.
func (c *Client) Connect(ctx context.Context, transport mcp.Transport) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	c.session = session
	return nil
}

// Disconnect closes the session.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		err := c.session.Close()
		c.session = nil
		return err
	}
	return nil
}

// ListTools sends tools/list and converts the response to schema.ToolDef slice.
func (c *Client) ListTools(ctx context.Context) ([]schema.ToolDef, error) {
	c.mu.RLock()
	s := c.session
	c.mu.RUnlock()

	if s == nil {
		return nil, fmt.Errorf("not connected")
	}

	result, err := s.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	defs := make([]schema.ToolDef, len(result.Tools))
	for i, t := range result.Tools {
		defs[i] = schema.ToolDef{
			Name:         t.Name,
			Description:  t.Description,
			Parameters:   t.InputSchema,
			Source:       schema.ToolSourceMCP,
			MCPServerURI: c.serverURI,
		}
	}
	return defs, nil
}

// CallTool sends tools/call and converts the response to schema.ToolResult.
func (c *Client) CallTool(ctx context.Context, name, args string) (schema.ToolResult, error) {
	c.mu.RLock()
	s := c.session
	c.mu.RUnlock()

	if s == nil {
		return schema.ToolResult{}, fmt.Errorf("not connected")
	}

	var argsObj any
	if err := json.Unmarshal([]byte(args), &argsObj); err != nil {
		return schema.ToolResult{}, fmt.Errorf("invalid tool arguments JSON: %w", err)
	}

	result, err := s.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: argsObj,
	})
	if err != nil {
		return schema.ToolResult{}, fmt.Errorf("call tool: %w", err)
	}

	parts := make([]schema.ContentPart, len(result.Content))
	for i, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			parts[i] = schema.ContentPart{Type: "text", Text: tc.Text}
		}
	}

	return schema.ToolResult{
		Content: parts,
		IsError: result.IsError,
	}, nil
}

// Ping sends a ping request to the server.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	s := c.session
	c.mu.RUnlock()

	if s == nil {
		return fmt.Errorf("not connected")
	}

	return s.Ping(ctx, &mcp.PingParams{})
}
