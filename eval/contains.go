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
	"strings"
	"time"
)

var _ Evaluator = (*ContainsEval)(nil)

// ContainsConfig configures the ContainsEval evaluator.
type ContainsConfig struct {
	// Keywords is the list of keywords to check for in the output.
	Keywords []string
	// PassThreshold is the minimum score required to pass.
	// Default is 1.0 (all keywords must be found).
	PassThreshold float64
}

// ContainsEval checks whether the actual output contains all specified keywords.
// Keywords are provided via ContainsConfig. Matching is case-insensitive.
type ContainsEval struct {
	keywords  []string
	threshold float64
}

// NewContainsEval creates a new ContainsEval with the given configuration.
// Returns an error if cfg is nil.
func NewContainsEval(cfg *ContainsConfig) (*ContainsEval, error) {
	if cfg == nil {
		return nil, errors.New("ContainsEval requires a non-nil config")
	}

	threshold := cfg.PassThreshold
	if threshold <= 0 {
		threshold = 1.0
	}

	return &ContainsEval{keywords: cfg.Keywords, threshold: threshold}, nil
}

// Evaluate implements Evaluator.
func (e *ContainsEval) Evaluate(_ context.Context, c *EvalCase) (*EvalResult, error) {
	start := time.Now()

	if c.Actual == nil {
		return nil, errors.New("contains eval requires a non-nil Actual response")
	}

	actualText := strings.ToLower(lastAssistantText(c.Actual))

	if len(e.keywords) == 0 {
		return &EvalResult{
			CaseID: c.ID,
			Score:  1.0,
			Passed: true,
			Details: []EvalDetail{
				{
					Name:    "contains",
					Score:   1.0,
					Passed:  true,
					Message: "no keywords to check (vacuously true)",
				},
			},
			Duration: time.Since(start).Milliseconds(),
			Usage:    c.Actual.Usage,
		}, nil
	}

	matched := 0
	details := make([]EvalDetail, 0, len(e.keywords))

	for _, keyword := range e.keywords {
		found := strings.Contains(actualText, strings.ToLower(keyword))
		detailScore := 0.0
		msg := fmt.Sprintf("keyword %q not found", keyword)

		if found {
			matched++
			detailScore = 1.0
			msg = fmt.Sprintf("keyword %q found", keyword)
		}

		details = append(details, EvalDetail{
			Name:    keyword,
			Score:   detailScore,
			Passed:  found,
			Message: msg,
		})
	}

	score := float64(matched) / float64(len(e.keywords))
	passed := score >= e.threshold

	return &EvalResult{
		CaseID:   c.ID,
		Score:    score,
		Passed:   passed,
		Details:  details,
		Duration: time.Since(start).Milliseconds(),
		Usage:    c.Actual.Usage,
	}, nil
}
