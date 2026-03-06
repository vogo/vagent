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

import "testing"

func TestContentFilterGuard_Implements(t *testing.T) {
	var _ Guard = (*ContentFilterGuard)(nil)
}

func TestContentFilterGuard_Pass(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"violence", "hate"},
	})

	result, err := g.Check(NewInputMessage("Hello, how are you?"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestContentFilterGuard_BlockInput(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"violence", "hate"},
	})

	result, err := g.Check(NewInputMessage("This contains violence"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}

	if len(result.Violations) != 1 || result.Violations[0] != "violence" {
		t.Errorf("Check() Violations = %v, want [violence]", result.Violations)
	}
}

func TestContentFilterGuard_BlockOutput(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"violence"},
	})

	result, err := g.Check(NewOutputMessage("This contains violence"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}
}

func TestContentFilterGuard_CaseInsensitive(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"violence"},
	})

	result, err := g.Check(NewInputMessage("This contains VIOLENCE"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q for case-insensitive match", result.Action, ActionBlock)
	}
}

func TestContentFilterGuard_CaseSensitive(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"violence"},
		CaseSensitive:   true,
	})

	// Should pass because "VIOLENCE" != "violence" in case-sensitive mode
	result, err := g.Check(NewInputMessage("This contains VIOLENCE"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for case-sensitive non-match", result.Action, ActionPass)
	}

	// Should block for exact match
	result, err = g.Check(NewInputMessage("This contains violence"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q for case-sensitive match", result.Action, ActionBlock)
	}
}

func TestContentFilterGuard_EmptyKeywords(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{})

	result, err := g.Check(NewInputMessage("anything"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for empty keywords", result.Action, ActionPass)
	}
}

func TestContentFilterGuard_EmptyContent(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"violence"},
	})

	result, err := g.Check(NewInputMessage(""))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for empty content", result.Action, ActionPass)
	}
}

func TestContentFilterGuard_ViolationsReportOriginal(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{
		BlockedKeywords: []string{"Violence", "HATE"},
	})

	result, err := g.Check(NewInputMessage("this contains violence and hate speech"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}

	// Violations should report original casing
	if len(result.Violations) != 2 {
		t.Fatalf("Check() Violations length = %d, want 2", len(result.Violations))
	}

	if result.Violations[0] != "Violence" {
		t.Errorf("Violations[0] = %q, want %q (original casing)", result.Violations[0], "Violence")
	}

	if result.Violations[1] != "HATE" {
		t.Errorf("Violations[1] = %q, want %q (original casing)", result.Violations[1], "HATE")
	}
}

func TestContentFilterGuard_Name(t *testing.T) {
	g := NewContentFilterGuard(ContentFilterConfig{})

	if g.Name() != "content_filter" {
		t.Errorf("Name() = %q, want %q", g.Name(), "content_filter")
	}
}
