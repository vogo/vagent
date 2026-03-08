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
	"sync"
	"time"
)

// BackpressureConfig configures adaptive concurrency control.
type BackpressureConfig struct {
	InitialConcurrency int           // Starting concurrency level.
	MinConcurrency     int           // Minimum concurrency level.
	MaxConcurrency     int           // Maximum concurrency level.
	LatencyThreshold   time.Duration // Latency above this triggers concurrency decrease.
	AdjustInterval     time.Duration // How often to adjust concurrency.
}

// backpressureController implements adaptive concurrency control using AIMD
// (Additive Increase, Multiplicative Decrease).
type backpressureController struct {
	mu             sync.Mutex
	cfg            BackpressureConfig
	concurrency    int
	active         int
	waiters        []chan struct{}
	latencies      []time.Duration
	lastAdjustTime time.Time
}

func newBackpressureController(cfg *BackpressureConfig) *backpressureController {
	c := *cfg
	if c.InitialConcurrency <= 0 {
		c.InitialConcurrency = 1
	}
	if c.MinConcurrency <= 0 {
		c.MinConcurrency = 1
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = c.InitialConcurrency * 4
	}
	if c.InitialConcurrency < c.MinConcurrency {
		c.InitialConcurrency = c.MinConcurrency
	}
	if c.InitialConcurrency > c.MaxConcurrency {
		c.InitialConcurrency = c.MaxConcurrency
	}
	if c.AdjustInterval <= 0 {
		c.AdjustInterval = time.Second
	}

	return &backpressureController{
		cfg:            c,
		concurrency:    c.InitialConcurrency,
		lastAdjustTime: time.Now(),
	}
}

// acquire blocks until a concurrency slot is available or ctx is cancelled.
func (bp *backpressureController) acquire(ctx context.Context) error {
	bp.mu.Lock()
	if bp.active < bp.concurrency {
		bp.active++
		bp.mu.Unlock()
		return nil
	}
	// Need to wait.
	ch := make(chan struct{}, 1)
	bp.waiters = append(bp.waiters, ch)
	bp.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		// Remove ourselves from waiters.
		bp.mu.Lock()
		for i, w := range bp.waiters {
			if w == ch {
				bp.waiters = append(bp.waiters[:i], bp.waiters[i+1:]...)
				break
			}
		}
		bp.mu.Unlock()
		return ctx.Err()
	}
}

// release releases a slot and records the latency for adaptive adjustment.
func (bp *backpressureController) release(latency time.Duration) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.latencies = append(bp.latencies, latency)
	bp.maybeAdjust()

	// Wake a waiter if available and under concurrency limit.
	if len(bp.waiters) > 0 && bp.active <= bp.concurrency {
		w := bp.waiters[0]
		bp.waiters = bp.waiters[1:]
		w <- struct{}{}
		// active count stays the same (transferred to waiter).
	} else {
		bp.active--
		// Check again if we can wake a waiter after decrementing.
		if len(bp.waiters) > 0 && bp.active < bp.concurrency {
			w := bp.waiters[0]
			bp.waiters = bp.waiters[1:]
			bp.active++
			w <- struct{}{}
		}
	}
}

// maybeAdjust checks if enough time has passed and adjusts concurrency.
// Caller must hold bp.mu.
func (bp *backpressureController) maybeAdjust() {
	if time.Since(bp.lastAdjustTime) < bp.cfg.AdjustInterval {
		return
	}
	if len(bp.latencies) == 0 {
		return
	}

	var total time.Duration
	for _, l := range bp.latencies {
		total += l
	}
	avgLatency := total / time.Duration(len(bp.latencies))
	bp.latencies = bp.latencies[:0]
	bp.lastAdjustTime = time.Now()

	if avgLatency > bp.cfg.LatencyThreshold {
		// Multiplicative decrease: halve concurrency.
		bp.concurrency = max(bp.concurrency/2, bp.cfg.MinConcurrency)
	} else {
		// Additive increase: add 1.
		bp.concurrency = min(bp.concurrency+1, bp.cfg.MaxConcurrency)
	}
}

// currentConcurrency returns the current concurrency limit.
func (bp *backpressureController) currentConcurrency() int {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.concurrency
}
