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

var _ Evaluator = (*CostEval)(nil)

// CostConfig configures the CostEval evaluator.
type CostConfig struct {
	// Budget is the maximum number of tokens allowed.
	Budget int
	// FailOnMissingUsage controls behavior when Usage data is nil.
	// If true, evaluation fails when usage data is missing.
	// If false (default), missing usage is treated as passing.
	FailOnMissingUsage bool
}

// CostEval checks whether token usage is within a specified budget.
type CostEval struct {
	budget             int
	failOnMissingUsage bool
}

// NewCostEval creates a new CostEval with the given configuration.
// Returns an error if cfg is nil or budget is zero or negative.
func NewCostEval(cfg *CostConfig) (*CostEval, error) {
	if cfg == nil {
		return nil, errors.New("CostEval requires a non-nil config")
	}

	if cfg.Budget <= 0 {
		return nil, errors.New("cost budget must be greater than zero")
	}

	return &CostEval{
		budget:             cfg.Budget,
		failOnMissingUsage: cfg.FailOnMissingUsage,
	}, nil
}

// Evaluate implements Evaluator.
func (e *CostEval) Evaluate(_ context.Context, c *EvalCase) (*EvalResult, error) {
	start := time.Now()

	if c.Actual == nil {
		return nil, errors.New("cost eval requires a non-nil Actual response")
	}

	if c.Actual.Usage == nil {
		if e.failOnMissingUsage {
			return &EvalResult{
				CaseID: c.ID,
				Score:  0,
				Passed: false,
				Details: []EvalDetail{
					{
						Name:    "cost",
						Score:   0,
						Passed:  false,
						Message: "no usage data available and FailOnMissingUsage is enabled",
					},
				},
				Duration: time.Since(start).Milliseconds(),
			}, nil
		}

		return &EvalResult{
			CaseID: c.ID,
			Score:  1.0,
			Passed: true,
			Details: []EvalDetail{
				{
					Name:    "cost",
					Score:   1.0,
					Passed:  true,
					Message: "no usage data available",
				},
			},
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}

	totalTokens := c.Actual.Usage.TotalTokens
	score := clamp(1.0-float64(totalTokens)/float64(2*e.budget), 0, 1)
	passed := totalTokens <= e.budget

	return &EvalResult{
		CaseID: c.ID,
		Score:  score,
		Passed: passed,
		Details: []EvalDetail{
			{
				Name:    "cost",
				Score:   score,
				Passed:  passed,
				Message: fmt.Sprintf("used %d tokens vs budget %d", totalTokens, e.budget),
			},
		},
		Duration: time.Since(start).Milliseconds(),
		Usage:    c.Actual.Usage,
	}, nil
}
