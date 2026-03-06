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

func TestTopicGuard_Implements(t *testing.T) {
	var _ Guard = (*TopicGuard)(nil)
}

func TestTopicGuard_Pass(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		AllowedTopics: []string{"weather", "news"},
	})

	result, err := g.Check(NewInputMessage("What is the weather today?"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestTopicGuard_BlockedTopic(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		BlockedTopics: []string{"weapons", "drugs"},
	})

	result, err := g.Check(NewInputMessage("Tell me about weapons"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}

	if len(result.Violations) != 1 || result.Violations[0] != "weapons" {
		t.Errorf("Check() Violations = %v, want [weapons]", result.Violations)
	}
}

func TestTopicGuard_BlockedMultiple(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		BlockedTopics: []string{"weapons", "drugs"},
	})

	result, err := g.Check(NewInputMessage("Tell me about weapons and drugs"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}

	if len(result.Violations) != 2 {
		t.Fatalf("Check() Violations length = %d, want 2", len(result.Violations))
	}

	if result.Violations[0] != "weapons" || result.Violations[1] != "drugs" {
		t.Errorf("Check() Violations = %v, want [weapons drugs]", result.Violations)
	}
}

func TestTopicGuard_NotAllowed(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		AllowedTopics: []string{"weather", "news"},
	})

	result, err := g.Check(NewInputMessage("Tell me a joke"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q for not-allowed topic", result.Action, ActionBlock)
	}

	if result.Reason != "topic not in allowed list" {
		t.Errorf("Check() Reason = %q, want %q", result.Reason, "topic not in allowed list")
	}
}

func TestTopicGuard_BlockedPriority(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		AllowedTopics: []string{"science"},
		BlockedTopics: []string{"weapons"},
	})

	// Content matches both allowed ("science") and blocked ("weapons")
	result, err := g.Check(NewInputMessage("science of weapons"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q (blocked should take priority)", result.Action, ActionBlock)
	}
}

func TestTopicGuard_EmptyAllowed(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		// No AllowedTopics, no BlockedTopics
	})

	result, err := g.Check(NewInputMessage("anything at all"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for empty allowed list", result.Action, ActionPass)
	}
}

func TestTopicGuard_CaseInsensitive(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		BlockedTopics: []string{"weapons"},
	})

	result, err := g.Check(NewInputMessage("Tell me about WEAPONS"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q for case-insensitive match", result.Action, ActionBlock)
	}
}

func TestTopicGuard_CaseSensitive(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		BlockedTopics: []string{"weapons"},
		CaseSensitive: true,
	})

	// Should pass because "WEAPONS" != "weapons" in case-sensitive mode
	result, err := g.Check(NewInputMessage("Tell me about WEAPONS"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for case-sensitive non-match", result.Action, ActionPass)
	}
}

func TestTopicGuard_Name(t *testing.T) {
	g := NewTopicGuard(TopicConfig{})

	if g.Name() != "topic" {
		t.Errorf("Name() = %q, want %q", g.Name(), "topic")
	}
}

func TestTopicGuard_OutputDirection(t *testing.T) {
	g := NewTopicGuard(TopicConfig{
		BlockedTopics: []string{"weapons"},
	})

	// TopicGuard now works for both input and output.
	result, err := g.Check(NewOutputMessage("Here is info about weapons"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q for output with blocked topic", result.Action, ActionBlock)
	}
}
