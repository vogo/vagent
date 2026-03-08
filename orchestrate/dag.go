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
	"fmt"
	"sync"

	"github.com/vogo/aimodel"
	"github.com/vogo/vagent/schema"
)

// ExecuteDAG runs a DAG of nodes with the given config and initial request.
func ExecuteDAG(ctx context.Context, cfg DAGConfig, nodes []Node, req *schema.RunRequest) (*DAGResult, error) {
	if len(nodes) == 0 {
		return &DAGResult{
			NodeResults: map[string]*schema.RunResponse{},
			NodeStatus:  map[string]NodeStatus{},
			FinalOutput: &schema.RunResponse{Messages: req.Messages, SessionID: req.SessionID},
		}, nil
	}

	nodeMap := make(map[string]*Node, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		if _, exists := nodeMap[n.ID]; exists {
			return nil, fmt.Errorf("orchestrate: duplicate node ID %q", n.ID)
		}
		nodeMap[n.ID] = n
	}

	for _, n := range nodes {
		for _, dep := range n.Deps {
			if _, ok := nodeMap[dep]; !ok {
				return nil, fmt.Errorf("orchestrate: node %q depends on unknown node %q", n.ID, dep)
			}
		}
	}

	if err := detectCycle(nodes); err != nil {
		return nil, err
	}

	downstream := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, n := range nodes {
		inDegree[n.ID] = len(n.Deps)
		for _, dep := range n.Deps {
			downstream[dep] = append(downstream[dep], n.ID)
		}
	}

	terminalNodes := make(map[string]bool)
	for _, n := range nodes {
		if len(downstream[n.ID]) == 0 {
			terminalNodes[n.ID] = true
		}
	}

	result := &DAGResult{
		NodeResults: make(map[string]*schema.RunResponse, len(nodes)),
		NodeStatus:  make(map[string]NodeStatus, len(nodes)),
	}
	for _, n := range nodes {
		result.NodeStatus[n.ID] = NodePending
	}

	var totalUsage aimodel.Usage
	hasUsage := false

	var mu sync.Mutex
	parentCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var sem chan struct{}
	if cfg.MaxConcurrency > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrency)
	}

	doneCh := make(chan nodeCompletion, len(nodes))
	pending := len(nodes)

	// scheduleReady checks a ready node's condition, skips it if false (recursively
	// propagating to downstream), or launches it. Caller must hold mu.
	var scheduleReady func(nodeID string)
	scheduleReady = func(nodeID string) {
		node := nodeMap[nodeID]
		if node.Condition != nil {
			upResults := make(map[string]*schema.RunResponse, len(node.Deps))
			for _, dep := range node.Deps {
				upResults[dep] = result.NodeResults[dep]
			}
			if !node.Condition(upResults) {
				result.NodeStatus[nodeID] = NodeSkipped
				pending--
				for _, dsID := range downstream[nodeID] {
					inDegree[dsID]--
					if inDegree[dsID] == 0 && result.NodeStatus[dsID] == NodePending {
						scheduleReady(dsID)
					}
				}
				return
			}
		}
		nodeReq, err := buildNodeInput(node, req, result.NodeResults)
		if err != nil {
			result.NodeStatus[nodeID] = NodeRunning
			doneCh <- nodeCompletion{nodeID: nodeID, err: err}
			return
		}
		launchNode(ctx, node, nodeReq, sem, result, doneCh)
	}

	mu.Lock()
	for _, n := range nodes {
		if inDegree[n.ID] == 0 {
			scheduleReady(n.ID)
		}
	}
	mu.Unlock()

	var firstErr error
	for pending > 0 {
		select {
		case comp := <-doneCh:
			pending--
			mu.Lock()

			if comp.err != nil {
				result.NodeStatus[comp.nodeID] = NodeFailed

				node := nodeMap[comp.nodeID]
				if cfg.ErrorStrategy == Abort || !node.Optional {
					firstErr = fmt.Errorf("orchestrate: node %q failed: %w", comp.nodeID, comp.err)
					running := countRunning(result)
					mu.Unlock()
					cancel()
					drainDoneCh(doneCh, running)
					if hasUsage {
						result.Usage = &totalUsage
					}
					return result, firstErr
				}
				result.NodeStatus[comp.nodeID] = NodeSkipped
			} else {
				result.NodeStatus[comp.nodeID] = NodeDone
				result.NodeResults[comp.nodeID] = comp.resp

				if comp.resp != nil && comp.resp.Usage != nil {
					hasUsage = true
					totalUsage.Add(comp.resp.Usage)
				}

				if cfg.EarlyExitFunc != nil && cfg.EarlyExitFunc(comp.nodeID, comp.resp) {
					running := countRunning(result)
					mu.Unlock()
					cancel()
					drainDoneCh(doneCh, running)
					goto aggregate
				}
			}

			// Check if context was cancelled before launching downstream.
			if ctx.Err() != nil {
				running := countRunning(result)
				mu.Unlock()
				drainDoneCh(doneCh, running)
				if hasUsage {
					result.Usage = &totalUsage
				}
				return result, ctx.Err()
			}

			for _, dsID := range downstream[comp.nodeID] {
				inDegree[dsID]--
				if inDegree[dsID] == 0 && result.NodeStatus[dsID] == NodePending {
					scheduleReady(dsID)
				}
			}

			mu.Unlock()

		case <-ctx.Done():
			mu.Lock()
			running := countRunning(result)
			mu.Unlock()
			drainDoneCh(doneCh, running)
			if hasUsage {
				result.Usage = &totalUsage
			}
			return result, ctx.Err()
		}
	}

aggregate:
	if hasUsage {
		result.Usage = &totalUsage
	}

	termResults := make(map[string]*schema.RunResponse)
	for id := range terminalNodes {
		if result.NodeStatus[id] == NodeDone {
			termResults[id] = result.NodeResults[id]
		}
	}

	// If no terminal nodes completed (e.g. early exit before they ran),
	// fall back to all completed node results.
	if len(termResults) == 0 {
		for id, status := range result.NodeStatus {
			if status == NodeDone {
				termResults[id] = result.NodeResults[id]
			}
		}
	}

	agg := cfg.Aggregator
	if agg == nil {
		agg = ConcatMessagesAggregator()
	}

	finalOutput, err := agg.Aggregate(parentCtx, termResults)
	if err != nil {
		return result, fmt.Errorf("orchestrate: aggregation failed: %w", err)
	}
	result.FinalOutput = finalOutput

	return result, nil
}

type nodeCompletion struct {
	nodeID string
	resp   *schema.RunResponse
	err    error
}

// launchNode starts a goroutine to run a node. Caller must hold mu.
func launchNode(ctx context.Context, node *Node, req *schema.RunRequest,
	sem chan struct{}, result *DAGResult, doneCh chan<- nodeCompletion) {

	result.NodeStatus[node.ID] = NodeRunning

	go func() {
		if sem != nil {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				doneCh <- nodeCompletion{nodeID: node.ID, err: ctx.Err()}
				return
			}
		}

		resp, err := node.Runner.Run(ctx, req)
		if err != nil {
			doneCh <- nodeCompletion{nodeID: node.ID, err: err}
			return
		}
		if resp == nil {
			doneCh <- nodeCompletion{nodeID: node.ID, err: fmt.Errorf("nil response from node %q", node.ID)}
			return
		}
		doneCh <- nodeCompletion{nodeID: node.ID, resp: resp}
	}()
}

func buildNodeInput(node *Node, originalReq *schema.RunRequest, results map[string]*schema.RunResponse) (*schema.RunRequest, error) {
	if node.InputMapper != nil {
		upstreamResults := make(map[string]*schema.RunResponse, len(node.Deps))
		for _, dep := range node.Deps {
			upstreamResults[dep] = results[dep]
		}
		mapped, err := node.InputMapper(upstreamResults)
		if err != nil {
			return nil, fmt.Errorf("orchestrate: InputMapper for node %q: %w", node.ID, err)
		}
		return mapped, nil
	}

	if len(node.Deps) == 0 {
		return originalReq, nil
	}

	if len(node.Deps) == 1 {
		depResp := results[node.Deps[0]]
		if depResp != nil {
			return &schema.RunRequest{
				Messages:  depResp.Messages,
				SessionID: originalReq.SessionID,
				Options:   originalReq.Options,
				Metadata:  originalReq.Metadata,
			}, nil
		}
		return originalReq, nil
	}

	// Multiple deps: concatenate messages in Deps declaration order.
	var msgs []schema.Message
	for _, dep := range node.Deps {
		if r := results[dep]; r != nil {
			msgs = append(msgs, r.Messages...)
		}
	}
	return &schema.RunRequest{
		Messages:  msgs,
		SessionID: originalReq.SessionID,
		Options:   originalReq.Options,
		Metadata:  originalReq.Metadata,
	}, nil
}

func detectCycle(nodes []Node) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := make(map[string]int)
	adj := make(map[string][]string)
	for _, n := range nodes {
		color[n.ID] = white
		for _, dep := range n.Deps {
			adj[dep] = append(adj[dep], n.ID)
		}
	}

	var visit func(id string) error
	visit = func(id string) error {
		color[id] = gray
		for _, next := range adj[id] {
			switch color[next] {
			case gray:
				return fmt.Errorf("orchestrate: cycle detected involving node %q", next)
			case white:
				if err := visit(next); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for _, n := range nodes {
		if color[n.ID] == white {
			if err := visit(n.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// countRunning returns the number of nodes currently in NodeRunning status.
func countRunning(result *DAGResult) int {
	running := 0
	for _, s := range result.NodeStatus {
		if s == NodeRunning {
			running++
		}
	}
	return running
}

// drainDoneCh reads count completions from the channel to clean up running goroutines.
func drainDoneCh(doneCh <-chan nodeCompletion, count int) {
	for count > 0 {
		<-doneCh
		count--
	}
}
