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

package schema

// Tool source constants.
const (
	ToolSourceLocal = "local"
	ToolSourceMCP   = "mcp"
	ToolSourceAgent = "agent"
)

// ToolDef describes a tool that can be registered and invoked.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"` // JSON Schema
	// ForceUse indicates whether the LLM must invoke this tool (i.e. forced tool choice).
	// It does NOT refer to parameter-level "required" in JSON Schema.
	ForceUse     bool   `json:"force_use,omitempty"`
	Source       string `json:"source,omitempty"` // e.g. "local", "mcp", "agent"
	MCPServerURI string `json:"mcp_server_uri,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	ReadOnly     bool   `json:"read_only,omitempty"`
}
