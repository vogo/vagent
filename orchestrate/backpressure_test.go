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
	"testing"
	"time"
)

func TestBackpressureController_InitialConcurrency(t *testing.T) {
	bp := newBackpressureController(&BackpressureConfig{
		InitialConcurrency: 5,
		MinConcurrency:     1,
		MaxConcurrency:     10,
		LatencyThreshold:   100 * time.Millisecond,
		AdjustInterval:     50 * time.Millisecond,
	})

	if bp.currentConcurrency() != 5 {
		t.Errorf("initial concurrency = %d, want 5", bp.currentConcurrency())
	}
}

func TestBackpressureController_Defaults(t *testing.T) {
	bp := newBackpressureController(&BackpressureConfig{})
	if bp.currentConcurrency() != 1 {
		t.Errorf("default concurrency = %d, want 1", bp.currentConcurrency())
	}
	if bp.cfg.MinConcurrency != 1 {
		t.Errorf("default min = %d, want 1", bp.cfg.MinConcurrency)
	}
}

func TestBackpressureController_AdditiveIncrease(t *testing.T) {
	bp := newBackpressureController(&BackpressureConfig{
		InitialConcurrency: 2,
		MinConcurrency:     1,
		MaxConcurrency:     10,
		LatencyThreshold:   100 * time.Millisecond,
		AdjustInterval:     1 * time.Millisecond,
	})

	// Acquire and release with low latency.
	if err := bp.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)  // Ensure adjust interval passes.
	bp.release(10 * time.Millisecond) // Low latency.

	// Should have increased by 1.
	if bp.currentConcurrency() != 3 {
		t.Errorf("concurrency after low latency = %d, want 3", bp.currentConcurrency())
	}
}

func TestBackpressureController_MultiplicativeDecrease(t *testing.T) {
	bp := newBackpressureController(&BackpressureConfig{
		InitialConcurrency: 8,
		MinConcurrency:     1,
		MaxConcurrency:     16,
		LatencyThreshold:   50 * time.Millisecond,
		AdjustInterval:     1 * time.Millisecond,
	})

	if err := bp.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	bp.release(100 * time.Millisecond) // High latency, above threshold.

	// Should have halved: 8 / 2 = 4.
	if bp.currentConcurrency() != 4 {
		t.Errorf("concurrency after high latency = %d, want 4", bp.currentConcurrency())
	}
}

func TestBackpressureController_MaxConcurrency(t *testing.T) {
	bp := newBackpressureController(&BackpressureConfig{
		InitialConcurrency: 9,
		MinConcurrency:     1,
		MaxConcurrency:     10,
		LatencyThreshold:   100 * time.Millisecond,
		AdjustInterval:     1 * time.Millisecond,
	})

	if err := bp.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	bp.release(10 * time.Millisecond) // Low latency, increase to 10.

	if err := bp.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	bp.release(10 * time.Millisecond) // Low latency, should cap at 10.

	if bp.currentConcurrency() > 10 {
		t.Errorf("concurrency should not exceed max, got %d", bp.currentConcurrency())
	}
}

func TestBackpressureController_MinConcurrency(t *testing.T) {
	bp := newBackpressureController(&BackpressureConfig{
		InitialConcurrency: 2,
		MinConcurrency:     2,
		MaxConcurrency:     10,
		LatencyThreshold:   50 * time.Millisecond,
		AdjustInterval:     1 * time.Millisecond,
	})

	if err := bp.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	bp.release(100 * time.Millisecond) // High latency, try to decrease.

	// 2 / 2 = 1, but min is 2.
	if bp.currentConcurrency() < 2 {
		t.Errorf("concurrency should not go below min, got %d", bp.currentConcurrency())
	}
}

func TestDAG_BackpressureIntegration(t *testing.T) {
	nodes := []Node{
		{ID: "root", Runner: passthroughRunner()},
		{ID: "A", Runner: appendRunner("-A"), Deps: []string{"root"}},
		{ID: "B", Runner: appendRunner("-B"), Deps: []string{"root"}},
		{ID: "C", Runner: appendRunner("-C"), Deps: []string{"root"}},
	}

	cfg := DAGConfig{
		BackpressureCfg: &BackpressureConfig{
			InitialConcurrency: 2,
			MinConcurrency:     1,
			MaxConcurrency:     4,
			LatencyThreshold:   1 * time.Second,
			AdjustInterval:     100 * time.Millisecond,
		},
	}

	result, err := ExecuteDAG(context.Background(), cfg, nodes, makeReq("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, id := range []string{"A", "B", "C"} {
		if result.NodeStatus[id] != NodeDone {
			t.Errorf("node %s status = %d, want NodeDone", id, result.NodeStatus[id])
		}
	}
}
