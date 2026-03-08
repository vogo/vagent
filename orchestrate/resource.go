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
	"sort"
	"sync"
	"time"
)

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64   // tokens per second
	capacity float64   // max tokens (burst size = rate)
	tokens   float64   // current tokens
	lastTime time.Time // last refill time
}

func newTokenBucket(ratePerSecond float64) *tokenBucket {
	return &tokenBucket{
		rate:     ratePerSecond,
		capacity: ratePerSecond, // burst = rate
		tokens:   ratePerSecond, // start full
		lastTime: time.Now(),
	}
}

// wait blocks until a token is available or ctx is cancelled.
func (tb *tokenBucket) wait(ctx context.Context) error {
	for {
		tb.mu.Lock()
		tb.refill()
		if tb.tokens >= 1.0 {
			tb.tokens--
			tb.mu.Unlock()
			return nil
		}
		// Calculate exact wait time for one token to become available.
		deficit := 1.0 - tb.tokens
		waitDur := max(time.Duration(deficit/tb.rate*float64(time.Second)), time.Millisecond)
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// retry
		}
	}
}

// refill adds tokens based on elapsed time. Caller must hold tb.mu.
func (tb *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastTime = now
}

// resourceManager manages per-resource-tag concurrency limits and rate limits.
type resourceManager struct {
	concurrencySems map[string]chan struct{} // tag -> semaphore channel
	rateLimiters    map[string]*tokenBucket  // tag -> rate limiter
}

func newResourceManager(limits map[string]int, rateLimits map[string]float64) *resourceManager {
	rm := &resourceManager{
		concurrencySems: make(map[string]chan struct{}),
		rateLimiters:    make(map[string]*tokenBucket),
	}
	for tag, limit := range limits {
		rm.concurrencySems[tag] = make(chan struct{}, limit)
	}
	for tag, rate := range rateLimits {
		rm.rateLimiters[tag] = newTokenBucket(rate)
	}
	return rm
}

// acquire acquires concurrency slots and rate limit tokens for all resource tags.
// Tags are sorted before acquisition to prevent ABBA deadlocks when multiple
// nodes acquire overlapping tags concurrently.
func (rm *resourceManager) acquire(ctx context.Context, tags []string) error {
	if rm == nil || len(tags) == 0 {
		return nil
	}

	// Sort tags to ensure consistent acquisition order and prevent deadlocks.
	sorted := make([]string, len(tags))
	copy(sorted, tags)
	sort.Strings(sorted)

	// Acquire concurrency slots.
	acquired := make([]string, 0, len(sorted))
	for _, tag := range sorted {
		sem, ok := rm.concurrencySems[tag]
		if !ok {
			continue
		}
		select {
		case sem <- struct{}{}:
			acquired = append(acquired, tag)
		case <-ctx.Done():
			// Release any acquired slots.
			rm.releaseConcurrency(acquired)
			return ctx.Err()
		}
	}

	// Acquire rate limit tokens.
	for _, tag := range sorted {
		rl, ok := rm.rateLimiters[tag]
		if !ok {
			continue
		}
		if err := rl.wait(ctx); err != nil {
			rm.releaseConcurrency(acquired)
			return err
		}
	}

	return nil
}

// release releases concurrency slots for all resource tags.
func (rm *resourceManager) release(tags []string) {
	if rm == nil || len(tags) == 0 {
		return
	}
	rm.releaseConcurrency(tags)
}

func (rm *resourceManager) releaseConcurrency(tags []string) {
	for _, tag := range tags {
		sem, ok := rm.concurrencySems[tag]
		if !ok {
			continue
		}
		select {
		case <-sem:
		default:
		}
	}
}
