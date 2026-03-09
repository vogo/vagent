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

package eval

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var _ Evaluator = (*LatencyEval)(nil)

// LatencyEval checks whether the agent response time is within an acceptable threshold.
type LatencyEval struct {
	thresholdMs int64
}

// NewLatencyEval creates a new LatencyEval with the given threshold in milliseconds.
// Returns an error if threshold is zero or negative.
func NewLatencyEval(thresholdMs int64) (*LatencyEval, error) {
	if thresholdMs <= 0 {
		return nil, errors.New("latency threshold must be greater than zero")
	}

	return &LatencyEval{thresholdMs: thresholdMs}, nil
}

// Evaluate implements Evaluator.
// It reads the agent's response duration from c.Actual.Duration and scores accordingly.
func (e *LatencyEval) Evaluate(_ context.Context, c *EvalCase) (*EvalResult, error) {
	start := time.Now()

	if c.Actual == nil {
		return nil, errors.New("latency eval requires a non-nil Actual response")
	}

	actualDuration := c.Actual.Duration
	score := clamp(1.0-float64(actualDuration)/float64(2*e.thresholdMs), 0, 1)
	passed := actualDuration <= e.thresholdMs

	return &EvalResult{
		CaseID: c.ID,
		Score:  score,
		Passed: passed,
		Details: []EvalDetail{
			{
				Name:    "latency",
				Score:   score,
				Passed:  passed,
				Message: fmt.Sprintf("duration %dms vs threshold %dms", actualDuration, e.thresholdMs),
			},
		},
		Duration: time.Since(start).Milliseconds(),
		Usage:    c.Actual.Usage,
	}, nil
}
