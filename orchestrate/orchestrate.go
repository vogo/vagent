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

package orchestrate

import (
	"context"

	"github.com/vogo/aimodel"
	"github.com/vogo/vagent/schema"
)

// Runner executes a unit of work. agent.Agent satisfies this interface.
type Runner interface {
	Run(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)
}

// ErrorStrategy controls how the DAG engine handles node failures.
type ErrorStrategy int

const (
	Abort ErrorStrategy = iota
	Skip
)

// InputMapFunc maps upstream results to the current node's input.
type InputMapFunc func(upstreamResults map[string]*schema.RunResponse) (*schema.RunRequest, error)

// Node is a single node in a DAG execution graph.
type Node struct {
	ID          string
	Runner      Runner
	Deps        []string
	InputMapper InputMapFunc
	Optional    bool
	Condition   func(upstreamResults map[string]*schema.RunResponse) bool
}

// LoopNode defines a loop with a body runner and termination conditions.
type LoopNode struct {
	Body            Runner
	Condition       func(*schema.RunResponse) bool
	MaxIters        int
	ConvergenceFunc func(prev, curr *schema.RunResponse) bool
}

// DAGConfig holds configuration for DAG execution.
type DAGConfig struct {
	MaxConcurrency int
	ErrorStrategy  ErrorStrategy
	EarlyExitFunc  func(nodeID string, resp *schema.RunResponse) bool
	Aggregator     Aggregator
}

// NodeStatus represents the execution status of a node.
type NodeStatus int

const (
	NodePending NodeStatus = iota
	NodeRunning
	NodeDone
	NodeFailed
	NodeSkipped
)

// DAGResult holds the results of a DAG execution.
type DAGResult struct {
	NodeResults map[string]*schema.RunResponse
	NodeStatus  map[string]NodeStatus
	FinalOutput *schema.RunResponse
	Usage       *aimodel.Usage
}
