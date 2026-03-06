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
	"context"
	"errors"
	"strings"
	"testing"
)

func passGuard(name string) Guard {
	return NewCustomGuard(name, func(_ *Message) (*Result, error) {
		return Pass(), nil
	})
}

func blockGuard(name string) Guard {
	return NewCustomGuard(name, func(_ *Message) (*Result, error) {
		return Block(name, "blocked"), nil
	})
}

func rewriteGuard(name, newContent string) Guard {
	return NewCustomGuard(name, func(_ *Message) (*Result, error) {
		return Rewrite(name, newContent, "rewritten"), nil
	})
}

func errorGuard(name string, err error) Guard {
	return NewCustomGuard(name, func(_ *Message) (*Result, error) {
		return nil, err
	})
}

// --- RunGuards Tests ---

func TestRunGuards_Empty(t *testing.T) {
	result, err := RunGuards(context.Background(), NewInputMessage("hello"))
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestRunGuards_AllPass(t *testing.T) {
	result, err := RunGuards(context.Background(), NewInputMessage("hello"),
		passGuard("g1"), passGuard("g2"), passGuard("g3"))
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestRunGuards_Block(t *testing.T) {
	result, err := RunGuards(context.Background(), NewInputMessage("hello"),
		blockGuard("blocker"))
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionBlock)
	}

	if result.GuardName != "blocker" {
		t.Errorf("RunGuards() GuardName = %q, want %q", result.GuardName, "blocker")
	}
}

func TestRunGuards_BlockMiddle(t *testing.T) {
	called := false
	afterBlock := NewCustomGuard("after", func(_ *Message) (*Result, error) {
		called = true
		return Pass(), nil
	})

	result, err := RunGuards(context.Background(), NewInputMessage("hello"),
		passGuard("g1"), blockGuard("g2"), afterBlock)
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionBlock)
	}

	if called {
		t.Errorf("guard after block should not have been called")
	}
}

func TestRunGuards_Rewrite(t *testing.T) {
	var seenContent string
	observer := NewCustomGuard("observer", func(msg *Message) (*Result, error) {
		seenContent = msg.Content
		return Pass(), nil
	})

	msg := NewInputMessage("original")
	result, err := RunGuards(context.Background(), msg,
		rewriteGuard("rw", "modified"), observer)
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	// Chain returns ActionRewrite when rewrites occurred.
	if result.Action != ActionRewrite {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if seenContent != "modified" {
		t.Errorf("next guard saw content %q, want %q", seenContent, "modified")
	}

	if msg.Content != "modified" {
		t.Errorf("msg.Content = %q, want %q", msg.Content, "modified")
	}
}

func TestRunGuards_MultipleRewrite(t *testing.T) {
	msg := NewInputMessage("original")
	result, err := RunGuards(context.Background(), msg,
		rewriteGuard("rw1", "first"), rewriteGuard("rw2", "second"))
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if msg.Content != "second" {
		t.Errorf("msg.Content = %q, want %q", msg.Content, "second")
	}
}

func TestRunGuards_RewriteEmptyContent(t *testing.T) {
	msg := NewInputMessage("original")
	result, err := RunGuards(context.Background(), msg,
		rewriteGuard("rw", ""))
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if msg.Content != "" {
		t.Errorf("msg.Content = %q, want empty", msg.Content)
	}
}

func TestRunGuards_Error(t *testing.T) {
	testErr := errors.New("guard error")
	_, err := RunGuards(context.Background(), NewInputMessage("hello"),
		passGuard("g1"), errorGuard("g2", testErr))
	if err == nil {
		t.Fatalf("RunGuards() error = nil, want error")
	}

	if !errors.Is(err, testErr) {
		t.Errorf("RunGuards() error = %v, want %v", err, testErr)
	}
}

func TestRunGuards_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunGuards(ctx, NewInputMessage("hello"), passGuard("g1"))
	if err == nil {
		t.Fatalf("RunGuards() error = nil, want context.Canceled")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunGuards() error = %v, want context.Canceled", err)
	}
}

func TestRunGuards_NilResult(t *testing.T) {
	nilGuard := NewCustomGuard("nil_guard", func(_ *Message) (*Result, error) {
		return nil, nil
	})

	_, err := RunGuards(context.Background(), NewInputMessage("hello"), nilGuard)
	if err == nil {
		t.Fatalf("RunGuards() error = nil, want error for nil result")
	}

	if !strings.Contains(err.Error(), "nil result") {
		t.Errorf("RunGuards() error = %v, want error containing 'nil result'", err)
	}
}

func TestRunGuards_UnknownAction(t *testing.T) {
	unknownGuard := NewCustomGuard("unknown", func(_ *Message) (*Result, error) {
		return &Result{Action: "invalid_action"}, nil
	})

	_, err := RunGuards(context.Background(), NewInputMessage("hello"), unknownGuard)
	if err == nil {
		t.Fatalf("RunGuards() error = nil, want error for unknown action")
	}

	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("RunGuards() error = %v, want error containing 'unknown action'", err)
	}
}

func TestRunGuards_OutputMessage(t *testing.T) {
	result, err := RunGuards(context.Background(), NewOutputMessage("hello"),
		passGuard("g1"), passGuard("g2"))
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestRunGuards_RewriteCollectsViolations(t *testing.T) {
	g1 := NewCustomGuard("g1", func(_ *Message) (*Result, error) {
		return Rewrite("g1", "modified1", "reason1", "v1"), nil
	})
	g2 := NewCustomGuard("g2", func(_ *Message) (*Result, error) {
		return Rewrite("g2", "modified2", "reason2", "v2", "v3"), nil
	})

	result, err := RunGuards(context.Background(), NewInputMessage("original"), g1, g2)
	if err != nil {
		t.Fatalf("RunGuards() error = %v", err)
	}

	if result.Action != ActionRewrite {
		t.Fatalf("RunGuards() Action = %q, want %q", result.Action, ActionRewrite)
	}

	if len(result.Violations) != 3 {
		t.Fatalf("RunGuards() Violations = %v, want 3 violations", result.Violations)
	}

	if result.Violations[0] != "v1" || result.Violations[1] != "v2" || result.Violations[2] != "v3" {
		t.Errorf("RunGuards() Violations = %v, want [v1 v2 v3]", result.Violations)
	}
}
