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
	"sync"
	"time"
)

// BatchOption configures batch evaluation behavior.
type BatchOption func(*batchConfig)

// batchConfig holds internal configuration for batch evaluation.
type batchConfig struct {
	concurrency int
}

// WithConcurrency sets the number of concurrent evaluations.
// If n <= 1, evaluations are run sequentially.
func WithConcurrency(n int) BatchOption {
	return func(cfg *batchConfig) {
		if n > 0 {
			cfg.concurrency = n
		}
	}
}

// BatchEval evaluates multiple cases with a single evaluator.
// All cases are evaluated; a failure in one case does not stop the batch.
// Context cancellation stops further evaluation and returns a partial report with an error.
// Use WithConcurrency to enable parallel evaluation.
func BatchEval(ctx context.Context, evaluator Evaluator, cases []*EvalCase, opts ...BatchOption) (*EvalReport, error) {
	cfg := &batchConfig{concurrency: 1}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.concurrency <= 1 {
		return batchEvalSequential(ctx, evaluator, cases)
	}

	return batchEvalConcurrent(ctx, evaluator, cases, cfg.concurrency)
}

// batchEvalSequential runs evaluations one at a time.
func batchEvalSequential(ctx context.Context, evaluator Evaluator, cases []*EvalCase) (*EvalReport, error) {
	start := time.Now()

	report := &EvalReport{
		TotalCases: len(cases),
	}

	var ctxErr error

	for _, c := range cases {
		if err := ctx.Err(); err != nil {
			ctxErr = err

			break
		}

		result, err := evaluator.Evaluate(ctx, c)
		if err != nil {
			report.ErrorCases++

			report.Results = append(report.Results, &EvalResult{
				CaseID: c.ID,
				Error:  err.Error(),
			})

			continue
		}

		report.Results = append(report.Results, result)

		if result.Passed {
			report.PassedCases++
		} else {
			report.FailedCases++
		}
	}

	computeAvgScore(report)

	report.TotalDuration = time.Since(start).Milliseconds()

	return report, ctxErr
}

// batchEvalConcurrent runs evaluations with bounded concurrency.
func batchEvalConcurrent(ctx context.Context, evaluator Evaluator, cases []*EvalCase, concurrency int) (*EvalReport, error) {
	start := time.Now()

	report := &EvalReport{
		TotalCases: len(cases),
	}

	results := make([]*EvalResult, len(cases))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup

	var ctxErr error

	for i, c := range cases {
		if err := ctx.Err(); err != nil {
			ctxErr = err

			break
		}

		wg.Add(1)

		go func(idx int, ec *EvalCase) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := evaluator.Evaluate(ctx, ec)
			if err != nil {
				results[idx] = &EvalResult{
					CaseID: ec.ID,
					Error:  err.Error(),
				}

				return
			}

			results[idx] = result
		}(i, c)
	}

	wg.Wait()

	for _, r := range results {
		if r == nil {
			continue
		}

		report.Results = append(report.Results, r)

		if r.Error != "" {
			report.ErrorCases++
		} else if r.Passed {
			report.PassedCases++
		} else {
			report.FailedCases++
		}
	}

	computeAvgScore(report)

	report.TotalDuration = time.Since(start).Milliseconds()

	return report, ctxErr
}

// computeAvgScore calculates the average score over non-error results.
func computeAvgScore(report *EvalReport) {
	nonErrorCount := 0
	scoreSum := 0.0

	for _, r := range report.Results {
		if r.Error == "" {
			nonErrorCount++
			scoreSum += r.Score
		}
	}

	if nonErrorCount > 0 {
		report.AvgScore = scoreSum / float64(nonErrorCount)
	}
}
