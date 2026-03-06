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

import (
	"regexp"
	"strings"
	"testing"
)

func TestPIIGuard_Implements(t *testing.T) {
	var _ Guard = (*PIIGuard)(nil)
}

func TestPIIGuard_Pass(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	result, err := g.Check(NewInputMessage("Hello, how are you?"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestPIIGuard_RewriteEmail(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	result, err := g.Check(NewInputMessage("Contact me at user@example.com please"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if strings.Contains(result.Content, "user@example.com") {
		t.Errorf("Check() Content still contains email: %q", result.Content)
	}

	if !strings.Contains(result.Content, "[REDACTED]") {
		t.Errorf("Check() Content missing [REDACTED]: %q", result.Content)
	}

	found := false
	for _, v := range result.Violations {
		if v == "email" {
			found = true
		}
	}

	if !found {
		t.Errorf("Check() Violations = %v, want to contain 'email'", result.Violations)
	}
}

func TestPIIGuard_RewritePhone(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	result, err := g.Check(NewInputMessage("Call me at 13812345678"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if strings.Contains(result.Content, "13812345678") {
		t.Errorf("Check() Content still contains phone: %q", result.Content)
	}
}

func TestPIIGuard_RewriteMultiple(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	result, err := g.Check(NewInputMessage("Email: user@example.com Phone: 13812345678"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if len(result.Violations) < 2 {
		t.Errorf("Check() Violations = %v, want at least 2", result.Violations)
	}
}

func TestPIIGuard_CustomReplacement(t *testing.T) {
	g := NewPIIGuard(PIIConfig{
		Patterns:    DefaultPIIPatterns(),
		Replacement: "***",
	})

	result, err := g.Check(NewInputMessage("Email: user@example.com"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !strings.Contains(result.Content, "***") {
		t.Errorf("Check() Content = %q, want to contain '***'", result.Content)
	}

	if strings.Contains(result.Content, "[REDACTED]") {
		t.Errorf("Check() Content should not contain [REDACTED]: %q", result.Content)
	}
}

func TestPIIGuard_DefaultReplacement(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	result, err := g.Check(NewInputMessage("Email: user@example.com"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !strings.Contains(result.Content, "[REDACTED]") {
		t.Errorf("Check() Content = %q, want to contain '[REDACTED]'", result.Content)
	}
}

func TestPIIGuard_NilPattern(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewPIIGuard with nil pattern did not panic")
		}
	}()

	NewPIIGuard(PIIConfig{
		Patterns: []PatternRule{
			{Name: "email", Pattern: regexp.MustCompile(`test`)},
			{Name: "nil_pattern", Pattern: nil},
		},
	})
}

func TestPIIGuard_OutputCheck(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	result, err := g.Check(NewOutputMessage("Your email is user@example.com"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if strings.Contains(result.Content, "user@example.com") {
		t.Errorf("Check() Content still contains email: %q", result.Content)
	}
}

func TestPIIGuard_Name(t *testing.T) {
	g := NewPIIGuard(PIIConfig{Patterns: DefaultPIIPatterns()})

	if g.Name() != "pii" {
		t.Errorf("Name() = %q, want %q", g.Name(), "pii")
	}
}

func TestDefaultPIIPatterns(t *testing.T) {
	patterns := DefaultPIIPatterns()
	if len(patterns) == 0 {
		t.Fatalf("DefaultPIIPatterns() returned empty slice")
	}

	for i, p := range patterns {
		if p.Name == "" {
			t.Errorf("DefaultPIIPatterns()[%d].Name is empty", i)
		}

		if p.Pattern == nil {
			t.Errorf("DefaultPIIPatterns()[%d].Pattern is nil", i)
		}
	}
}

func TestPIIGuard_SliceCopyDefense(t *testing.T) {
	patterns := []PatternRule{
		{Name: "email", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)},
	}

	g := NewPIIGuard(PIIConfig{Patterns: patterns})

	// Mutate the original slice after construction
	patterns[0] = PatternRule{Name: "mutated", Pattern: regexp.MustCompile(`nomatch_pattern_xyz`)}

	// The guard should still detect emails using the original pattern
	result, err := g.Check(NewInputMessage("Email: user@example.com"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("Check() Action = %q, want %q after external mutation", result.Action, ActionRewrite)
	}
}
