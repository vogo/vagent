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

var _ Evaluator = (*ExactMatchEval)(nil)

// ExactMatchEval compares actual output text against expected output text
// for exact string equality.
type ExactMatchEval struct{}

// NewExactMatchEval creates a new ExactMatchEval.
func NewExactMatchEval() (*ExactMatchEval, error) {
	return &ExactMatchEval{}, nil
}

// Evaluate implements Evaluator. It returns an error if Expected or Actual is nil.
func (e *ExactMatchEval) Evaluate(_ context.Context, c *EvalCase) (*EvalResult, error) {
	start := time.Now()

	if c.Expected == nil {
		return nil, errors.New("exact match requires a non-nil Expected response")
	}

	if c.Actual == nil {
		return nil, errors.New("exact match requires a non-nil Actual response")
	}

	expectedText := lastAssistantText(c.Expected)
	actualText := lastAssistantText(c.Actual)

	score := 0.0
	passed := false
	msg := fmt.Sprintf("expected %q but got %q", expectedText, actualText)

	if actualText == expectedText {
		score = 1.0
		passed = true
		msg = "exact match"
	}

	return &EvalResult{
		CaseID: c.ID,
		Score:  score,
		Passed: passed,
		Details: []EvalDetail{
			{
				Name:    "exact_match",
				Score:   score,
				Passed:  passed,
				Message: msg,
			},
		},
		Duration: time.Since(start).Milliseconds(),
		Usage:    c.Actual.Usage,
	}, nil
}
