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

package memory

import (
	"context"

	"github.com/vogo/vage/schema"
)

// ContextCompressor compresses a message history to fit within a token budget.
type ContextCompressor interface {
	Compress(ctx context.Context, messages []schema.Message, maxTokens int) ([]schema.Message, error)
}

// CompressFunc is a function adapter for ContextCompressor.
type CompressFunc func(ctx context.Context, messages []schema.Message, maxTokens int) ([]schema.Message, error)

// Compress implements ContextCompressor.
func (f CompressFunc) Compress(ctx context.Context, messages []schema.Message, maxTokens int) ([]schema.Message, error) {
	return f(ctx, messages, maxTokens)
}

// SlidingWindowCompressor keeps the last N messages, then delegates to
// TokenBudgetCompressor for token-based trimming within the window.
type SlidingWindowCompressor struct {
	windowSize int
	budget     *TokenBudgetCompressor
}

// NewSlidingWindowCompressor creates a compressor that keeps the last windowSize messages.
// Panics if windowSize is not positive.
func NewSlidingWindowCompressor(windowSize int) *SlidingWindowCompressor {
	if windowSize <= 0 {
		panic("memory: window size must be positive")
	}

	return &SlidingWindowCompressor{
		windowSize: windowSize,
		budget:     NewTokenBudgetCompressor(),
	}
}

// WithTokenEstimator sets a custom token estimator for the internal token budget check.
func (c *SlidingWindowCompressor) WithTokenEstimator(est TokenEstimator) *SlidingWindowCompressor {
	c.budget.WithTokenEstimator(est)

	return c
}

// Compress returns the last windowSize messages, optionally trimmed to fit within maxTokens.
// If maxTokens is 0 or negative, no token budget is applied.
func (c *SlidingWindowCompressor) Compress(ctx context.Context, messages []schema.Message, maxTokens int) ([]schema.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// First apply window size limit.
	start := 0
	if len(messages) > c.windowSize {
		start = len(messages) - c.windowSize
	}

	windowed := messages[start:]

	// If maxTokens == 0 (unlimited) or input is empty, return windowed messages as-is.
	if maxTokens <= 0 || len(windowed) == 0 {
		return windowed, nil
	}

	// Delegate token budget trimming to TokenBudgetCompressor.
	return c.budget.Compress(ctx, windowed, maxTokens)
}
