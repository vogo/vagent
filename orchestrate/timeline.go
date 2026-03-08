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
	"sort"
	"sync"
	"time"
)

// NodeTimeline records the execution timing of a single node.
type NodeTimeline struct {
	NodeID    string        `json:"node_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration_ns"`
	Status    NodeStatus    `json:"status"`
}

// timelineTracker records node execution timing in a thread-safe manner.
type timelineTracker struct {
	mu      sync.Mutex
	starts  map[string]time.Time
	entries []NodeTimeline
}

func newTimelineTracker() *timelineTracker {
	return &timelineTracker{
		starts: make(map[string]time.Time),
	}
}

// recordStart records the start time of a node.
func (t *timelineTracker) recordStart(nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.starts[nodeID] = time.Now()
}

// recordEnd records the end time and status of a node.
func (t *timelineTracker) recordEnd(nodeID string, status NodeStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	start, ok := t.starts[nodeID]
	if !ok {
		start = time.Now()
	}
	end := time.Now()
	t.entries = append(t.entries, NodeTimeline{
		NodeID:    nodeID,
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
		Status:    status,
	})
}

// result returns the timeline sorted by start time.
func (t *timelineTracker) result() []NodeTimeline {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]NodeTimeline, len(t.entries))
	copy(result, t.entries)
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})
	return result
}
