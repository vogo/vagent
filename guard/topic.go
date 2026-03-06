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

package guard

import "strings"

// TopicConfig configures the TopicGuard.
type TopicConfig struct {
	AllowedTopics []string
	BlockedTopics []string
	CaseSensitive bool
}

// TopicGuard restricts conversation topics.
// Works for both input and output directions.
type TopicGuard struct {
	allowed       []string // normalized
	blocked       []string // normalized
	origBlocked   []string // original blocked topics for violation reporting
	caseSensitive bool
}

var _ Guard = (*TopicGuard)(nil)

func NewTopicGuard(cfg TopicConfig) *TopicGuard {
	normalize := func(topics []string, cs bool) []string {
		out := make([]string, len(topics))
		for i, t := range topics {
			if cs {
				out[i] = t
			} else {
				out[i] = strings.ToLower(t)
			}
		}

		return out
	}

	origBlocked := make([]string, len(cfg.BlockedTopics))
	copy(origBlocked, cfg.BlockedTopics)

	return &TopicGuard{
		allowed:       normalize(cfg.AllowedTopics, cfg.CaseSensitive),
		blocked:       normalize(cfg.BlockedTopics, cfg.CaseSensitive),
		origBlocked:   origBlocked,
		caseSensitive: cfg.CaseSensitive,
	}
}

func (g *TopicGuard) Name() string { return "topic" }

func (g *TopicGuard) Check(msg *Message) (*Result, error) {
	text := msg.Content
	if !g.caseSensitive {
		text = strings.ToLower(msg.Content)
	}

	// 1. Check blocked topics first, collect all violations.
	var violations []string

	for i, topic := range g.blocked {
		if strings.Contains(text, topic) {
			violations = append(violations, g.origBlocked[i])
		}
	}

	if len(violations) > 0 {
		return Block(g.Name(), "blocked topic detected", violations...), nil
	}

	// 2. Check allowed topics (only when list is non-empty).
	if len(g.allowed) > 0 {
		for _, topic := range g.allowed {
			if strings.Contains(text, topic) {
				return Pass(), nil
			}
		}

		return Block(g.Name(), "topic not in allowed list"), nil
	}

	return Pass(), nil
}
