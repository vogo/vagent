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

import "regexp"

// PIIConfig configures the PIIGuard.
type PIIConfig struct {
	Patterns    []PatternRule
	Replacement string // defaults to "[REDACTED]"
}

// PIIGuard detects and redacts personally identifiable information.
type PIIGuard struct {
	patterns    []PatternRule
	replacement string
}

var _ Guard = (*PIIGuard)(nil)

// NewPIIGuard creates a PIIGuard.
// Panics if any Pattern is nil (programming error).
// The patterns slice is copied defensively.
func NewPIIGuard(cfg PIIConfig) *PIIGuard {
	patterns := make([]PatternRule, len(cfg.Patterns))
	copy(patterns, cfg.Patterns)

	for i, p := range patterns {
		if p.Pattern == nil {
			panic("vage: NewPIIGuard: pattern " + itoa(i) + " (" + p.Name + ") has nil regexp")
		}
	}

	replacement := cfg.Replacement
	if replacement == "" {
		replacement = "[REDACTED]"
	}

	return &PIIGuard{patterns: patterns, replacement: replacement}
}

func (g *PIIGuard) Name() string { return "pii" }

// redact applies all PII patterns to content, returning the redacted text
// and the list of matched pattern names.
func (g *PIIGuard) redact(content string) (string, []string) {
	var violations []string

	result := content

	for _, p := range g.patterns {
		replaced := p.Pattern.ReplaceAllString(result, g.replacement)
		if replaced != result {
			violations = append(violations, p.Name)
			result = replaced
		}
	}

	return result, violations
}

func (g *PIIGuard) Check(msg *Message) (*Result, error) {
	redacted, violations := g.redact(msg.Content)
	if len(violations) > 0 {
		return Rewrite(g.Name(), redacted, "PII detected and redacted", violations...), nil
	}

	return Pass(), nil
}

// DefaultPIIPatterns returns common PII detection patterns.
func DefaultPIIPatterns() []PatternRule {
	return []PatternRule{
		{Name: "email", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)},
		{Name: "phone", Pattern: regexp.MustCompile(`1[3-9]\d{9}`)},
		{Name: "id_card", Pattern: regexp.MustCompile(`\d{17}[\dXx]`)},
	}
}
