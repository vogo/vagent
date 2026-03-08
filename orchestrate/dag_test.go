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
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vogo/aimodel"
	"github.com/vogo/vagent/schema"
)

// mockRunner is a test helper that executes a function.
type mockRunner struct {
	fn func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)
}

func (m *mockRunner) Run(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
	return m.fn(ctx, req)
}

func newMockRunner(fn func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error)) *mockRunner {
	return &mockRunner{fn: fn}
}

func appendRunner(suffix string) *mockRunner {
	return newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
		text := ""
		if len(req.Messages) > 0 {
			text = req.Messages[0].Content.Text()
		}
		return &schema.RunResponse{
			Messages: []schema.Message{schema.NewUserMessage(text + suffix)},
		}, nil
	})
}

func passthroughRunner() *mockRunner {
	return newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
		return &schema.RunResponse{Messages: req.Messages}, nil
	})
}

func usageRunner(prompt, completion, total int) *mockRunner {
	return newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
		return &schema.RunResponse{
			Messages: req.Messages,
			Usage:    &aimodel.Usage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: total},
		}, nil
	})
}

func errorRunner(err error) *mockRunner {
	return newMockRunner(func(_ context.Context, _ *schema.RunRequest) (*schema.RunResponse, error) {
		return nil, err
	})
}

func nilResponseRunner() *mockRunner {
	return newMockRunner(func(_ context.Context, _ *schema.RunRequest) (*schema.RunResponse, error) {
		return nil, nil
	})
}

func makeReq(text string) *schema.RunRequest {
	return &schema.RunRequest{
		Messages:  []schema.Message{schema.NewUserMessage(text)},
		SessionID: "test-session",
	}
}

func TestExecuteDAG_Linear(t *testing.T) {
	// A -> B -> C
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B"), Deps: []string{"A"}},
		{ID: "C", Runner: appendRunner("-C"), Deps: []string{"B"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := result.FinalOutput.Messages[0].Content.Text()
	if got != "start-A-B-C" {
		t.Errorf("got %q, want %q", got, "start-A-B-C")
	}
	for _, id := range []string{"A", "B", "C"} {
		if result.NodeStatus[id] != NodeDone {
			t.Errorf("node %s status = %d, want NodeDone", id, result.NodeStatus[id])
		}
	}
}

func TestExecuteDAG_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B"), Deps: []string{"A"}},
		{ID: "C", Runner: appendRunner("-C"), Deps: []string{"A"}},
		{ID: "D", Runner: passthroughRunner(), Deps: []string{"B", "C"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// D gets messages from B and C concatenated in Deps declaration order (B, C).
	got := result.FinalOutput.Messages
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	// B's output: "start-A-B", C's output: "start-A-C"
	texts := []string{got[0].Content.Text(), got[1].Content.Text()}
	if texts[0] != "start-A-B" || texts[1] != "start-A-C" {
		t.Errorf("got %v, want [start-A-B, start-A-C]", texts)
	}
}

func TestExecuteDAG_FanOut(t *testing.T) {
	// A -> B, A -> C, A -> D
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B"), Deps: []string{"A"}},
		{ID: "C", Runner: appendRunner("-C"), Deps: []string{"A"}},
		{ID: "D", Runner: appendRunner("-D"), Deps: []string{"A"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{Aggregator: ConcatMessagesAggregator()}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All three are terminal nodes.
	if len(result.FinalOutput.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.FinalOutput.Messages))
	}
}

func TestExecuteDAG_FanIn(t *testing.T) {
	// A, B, C -> D
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B")},
		{ID: "C", Runner: appendRunner("-C")},
		{ID: "D", Runner: passthroughRunner(), Deps: []string{"A", "B", "C"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// D gets messages from A, B, C in Deps declaration order.
	got := result.FinalOutput.Messages
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	texts := []string{got[0].Content.Text(), got[1].Content.Text(), got[2].Content.Text()}
	if texts[0] != "start-A" || texts[1] != "start-B" || texts[2] != "start-C" {
		t.Errorf("got %v", texts)
	}
}

func TestExecuteDAG_CycleDetection(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: passthroughRunner(), Deps: []string{"C"}},
		{ID: "B", Runner: passthroughRunner(), Deps: []string{"A"}},
		{ID: "C", Runner: passthroughRunner(), Deps: []string{"B"}},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle: %v", err)
	}
}

func TestExecuteDAG_DuplicateNodeID(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: passthroughRunner()},
		{ID: "A", Runner: passthroughRunner()},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err == nil {
		t.Fatal("expected duplicate ID error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention duplicate: %v", err)
	}
}

func TestExecuteDAG_MissingDep(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: passthroughRunner(), Deps: []string{"Z"}},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err == nil {
		t.Fatal("expected missing dep error")
	}
	if !strings.Contains(err.Error(), "unknown node") {
		t.Errorf("error should mention unknown node: %v", err)
	}
}

func TestExecuteDAG_EmptyNodes(t *testing.T) {
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nil, makeReq("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalOutput.Messages[0].Content.Text() != "hello" {
		t.Errorf("expected input message echoed back")
	}
}

func TestExecuteDAG_AbortOnFailure(t *testing.T) {
	var bCalled atomic.Bool
	nodes := []Node{
		{ID: "A", Runner: errorRunner(errors.New("fail"))},
		{ID: "B", Runner: newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
			bCalled.Store(true)
			time.Sleep(50 * time.Millisecond)
			return &schema.RunResponse{Messages: req.Messages}, nil
		})},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{ErrorStrategy: Abort}, nodes, makeReq("start"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fail") {
		t.Errorf("error should contain original: %v", err)
	}
}

func TestExecuteDAG_SkipOptionalFailure(t *testing.T) {
	// A -> B (optional, fails) -> C; A -> D
	// With Skip strategy, B fails but is optional, so D should still complete.
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: errorRunner(errors.New("optional fail")), Deps: []string{"A"}, Optional: true},
		{ID: "D", Runner: appendRunner("-D"), Deps: []string{"A"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{ErrorStrategy: Skip}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeStatus["B"] != NodeSkipped {
		t.Errorf("B status = %d, want NodeSkipped", result.NodeStatus["B"])
	}
	if result.NodeStatus["D"] != NodeDone {
		t.Errorf("D status = %d, want NodeDone", result.NodeStatus["D"])
	}
}

func TestExecuteDAG_MaxConcurrency(t *testing.T) {
	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32

	runner := newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
		cur := currentConcurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old {
				break
			}
			if maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		currentConcurrent.Add(-1)
		return &schema.RunResponse{Messages: req.Messages}, nil
	})

	nodes := []Node{
		{ID: "A", Runner: runner},
		{ID: "B", Runner: runner},
		{ID: "C", Runner: runner},
		{ID: "D", Runner: runner},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{MaxConcurrency: 2}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxConcurrent.Load() > 2 {
		t.Errorf("max concurrent = %d, want <= 2", maxConcurrent.Load())
	}
}

func TestExecuteDAG_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	nodes := []Node{
		{ID: "A", Runner: newMockRunner(func(ctx context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
			cancel()
			return &schema.RunResponse{Messages: req.Messages}, nil
		})},
		{ID: "B", Runner: passthroughRunner(), Deps: []string{"A"}},
	}
	_, err := ExecuteDAG(ctx, DAGConfig{}, nodes, makeReq("start"))
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestExecuteDAG_InputMapper(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B")},
		{ID: "C", Runner: passthroughRunner(), Deps: []string{"A", "B"},
			InputMapper: func(upstream map[string]*schema.RunResponse) (*schema.RunRequest, error) {
				aText := upstream["A"].Messages[0].Content.Text()
				bText := upstream["B"].Messages[0].Content.Text()
				return &schema.RunRequest{
					Messages: []schema.Message{schema.NewUserMessage(aText + "+" + bText)},
				}, nil
			},
		},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := result.FinalOutput.Messages[0].Content.Text()
	if got != "start-A+start-B" {
		t.Errorf("got %q, want %q", got, "start-A+start-B")
	}
}

func TestExecuteDAG_DefaultMultiDepMerge(t *testing.T) {
	// Without InputMapper, multi-dep nodes get messages concatenated in Deps declaration order.
	nodes := []Node{
		{ID: "X", Runner: appendRunner("-X")},
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "D", Runner: passthroughRunner(), Deps: []string{"X", "A"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Deps declaration order: X, A
	got := result.FinalOutput.Messages
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Content.Text() != "start-X" {
		t.Errorf("first message = %q, want %q", got[0].Content.Text(), "start-X")
	}
	if got[1].Content.Text() != "start-A" {
		t.Errorf("second message = %q, want %q", got[1].Content.Text(), "start-A")
	}
}

func TestExecuteDAG_Aggregation(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B")},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{Aggregator: ConcatMessagesAggregator()}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.FinalOutput.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.FinalOutput.Messages))
	}
}

func TestExecuteDAG_EarlyExit(t *testing.T) {
	var bCalled atomic.Bool
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
			bCalled.Store(true)
			time.Sleep(50 * time.Millisecond)
			return &schema.RunResponse{Messages: req.Messages}, nil
		}), Deps: []string{"A"}},
	}
	cfg := DAGConfig{
		EarlyExitFunc: func(nodeID string, _ *schema.RunResponse) bool {
			return nodeID == "A"
		},
	}
	result, err := ExecuteDAG(context.Background(), cfg, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeStatus["A"] != NodeDone {
		t.Errorf("A should be Done")
	}
}

func TestExecuteDAG_ConditionalNode(t *testing.T) {
	// A -> B (conditional: only if A output contains "yes"), A -> C
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-no")},
		{ID: "B", Runner: appendRunner("-B"), Deps: []string{"A"},
			Condition: func(upstream map[string]*schema.RunResponse) bool {
				return strings.Contains(upstream["A"].Messages[0].Content.Text(), "yes")
			},
		},
		{ID: "C", Runner: appendRunner("-C"), Deps: []string{"A"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeStatus["B"] != NodeSkipped {
		t.Errorf("B status = %d, want NodeSkipped", result.NodeStatus["B"])
	}
	if result.NodeStatus["C"] != NodeDone {
		t.Errorf("C status = %d, want NodeDone", result.NodeStatus["C"])
	}
}

func TestExecuteDAG_UsageAccumulation(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: usageRunner(10, 20, 30)},
		{ID: "B", Runner: usageRunner(5, 15, 20)},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if result.Usage.PromptTokens != 15 {
		t.Errorf("PromptTokens = %d, want 15", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 35 {
		t.Errorf("CompletionTokens = %d, want 35", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 50 {
		t.Errorf("TotalTokens = %d, want 50", result.Usage.TotalTokens)
	}
}

func TestExecuteDAG_NilResponse(t *testing.T) {
	nodes := []Node{
		{ID: "A", Runner: nilResponseRunner()},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err == nil {
		t.Fatal("expected error for nil response")
	}
	if !strings.Contains(err.Error(), "nil response") {
		t.Errorf("error should mention nil response: %v", err)
	}
}

func TestExecuteDAG_ChainedConditionalSkip(t *testing.T) {
	// A -> B (condition: false) -> C (condition: false) -> D
	// B and C should both be skipped, D should still run with original input.
	nodes := []Node{
		{ID: "A", Runner: appendRunner("-A")},
		{ID: "B", Runner: appendRunner("-B"), Deps: []string{"A"},
			Condition: func(_ map[string]*schema.RunResponse) bool { return false },
		},
		{ID: "C", Runner: appendRunner("-C"), Deps: []string{"B"},
			Condition: func(_ map[string]*schema.RunResponse) bool { return false },
		},
		{ID: "D", Runner: appendRunner("-D"), Deps: []string{"C"}},
	}
	result, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeStatus["B"] != NodeSkipped {
		t.Errorf("B status = %d, want NodeSkipped", result.NodeStatus["B"])
	}
	if result.NodeStatus["C"] != NodeSkipped {
		t.Errorf("C status = %d, want NodeSkipped", result.NodeStatus["C"])
	}
	if result.NodeStatus["D"] != NodeDone {
		t.Errorf("D status = %d, want NodeDone", result.NodeStatus["D"])
	}
}

func TestExecuteDAG_ParallelExecution(t *testing.T) {
	// Verify that independent nodes actually run in parallel.
	var mu sync.Mutex
	var order []string

	makeTracker := func(id string, delay time.Duration) *mockRunner {
		return newMockRunner(func(_ context.Context, req *schema.RunRequest) (*schema.RunResponse, error) {
			mu.Lock()
			order = append(order, id+"-start")
			mu.Unlock()
			time.Sleep(delay)
			mu.Lock()
			order = append(order, id+"-end")
			mu.Unlock()
			return &schema.RunResponse{Messages: req.Messages}, nil
		})
	}

	nodes := []Node{
		{ID: "A", Runner: makeTracker("A", 50*time.Millisecond)},
		{ID: "B", Runner: makeTracker("B", 50*time.Millisecond)},
	}
	_, err := ExecuteDAG(context.Background(), DAGConfig{}, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both should start before either ends (parallel).
	if len(order) != 4 {
		t.Fatalf("expected 4 events, got %d: %v", len(order), order)
	}
	// The first two should be starts.
	startCount := 0
	for _, e := range order[:2] {
		if strings.HasSuffix(e, "-start") {
			startCount++
		}
	}
	if startCount != 2 {
		t.Errorf("expected both nodes to start before either ends, got order: %v", order)
	}
}
