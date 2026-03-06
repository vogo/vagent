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
	"strings"
	"testing"
)

func TestLengthGuard_Implements(t *testing.T) {
	var _ Guard = (*LengthGuard)(nil)
}

func TestLengthGuard_Pass(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 100})

	result, err := g.Check(NewInputMessage("short text"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionPass)
	}
}

func TestLengthGuard_Block(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 5})

	result, err := g.Check(NewInputMessage("this is too long"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}

	if !strings.Contains(result.Reason, "exceeds maximum") {
		t.Errorf("Check() Reason = %q, want to contain 'exceeds maximum'", result.Reason)
	}
}

func TestLengthGuard_ExactLimit(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 5})

	result, err := g.Check(NewInputMessage("abcde"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for exact limit", result.Action, ActionPass)
	}
}

func TestLengthGuard_ZeroMax(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 0})

	result, err := g.Check(NewInputMessage("anything"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for MaxLength=0", result.Action, ActionPass)
	}
}

func TestLengthGuard_NegativeMax(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: -1})

	result, err := g.Check(NewInputMessage("anything"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for MaxLength<0", result.Action, ActionPass)
	}
}

func TestLengthGuard_Unicode(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 3})

	// 3 CJK characters = 3 runes (but more bytes)
	result, err := g.Check(NewInputMessage("你好世"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for 3 rune CJK", result.Action, ActionPass)
	}

	// 4 CJK characters should block
	result, err = g.Check(NewInputMessage("你好世界"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q for 4 rune CJK exceeding limit 3", result.Action, ActionBlock)
	}
}

func TestLengthGuard_EmptyContent(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 10})

	result, err := g.Check(NewInputMessage(""))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q for empty content", result.Action, ActionPass)
	}
}

func TestLengthGuard_OutputCheck(t *testing.T) {
	g := NewLengthGuard(LengthConfig{MaxLength: 5})

	result, err := g.Check(NewOutputMessage("this is too long"))
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionBlock {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionBlock)
	}
}

func TestLengthGuard_Name(t *testing.T) {
	g := NewLengthGuard(LengthConfig{})

	if g.Name() != "length" {
		t.Errorf("Name() = %q, want %q", g.Name(), "length")
	}
}
