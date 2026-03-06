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
	"errors"
	"strings"
	"testing"
)

func TestCustomGuard_Implements(t *testing.T) {
	var _ Guard = (*CustomGuard)(nil)
}

func TestCustomGuard_Delegates(t *testing.T) {
	var receivedMsg *Message

	g := NewCustomGuard("my_guard", func(msg *Message) (*Result, error) {
		receivedMsg = msg
		return Pass(), nil
	})

	msg := &Message{Direction: DirectionInput, Content: "test input", AgentID: "agent1"}
	result, err := g.Check(msg)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != ActionPass {
		t.Fatalf("Check() Action = %q, want %q", result.Action, ActionPass)
	}

	if receivedMsg != msg {
		t.Errorf("Check() did not receive the correct message")
	}
}

func TestCustomGuard_Name(t *testing.T) {
	g := NewCustomGuard("my_custom_guard", func(_ *Message) (*Result, error) {
		return Pass(), nil
	})

	if g.Name() != "my_custom_guard" {
		t.Errorf("Name() = %q, want %q", g.Name(), "my_custom_guard")
	}
}

func TestCustomGuard_Error(t *testing.T) {
	testErr := errors.New("custom error")
	g := NewCustomGuard("err_guard", func(_ *Message) (*Result, error) {
		return nil, testErr
	})

	_, err := g.Check(NewInputMessage("test"))
	if err == nil {
		t.Fatalf("Check() error = nil, want error")
	}

	if !errors.Is(err, testErr) {
		t.Errorf("Check() error = %v, want %v", err, testErr)
	}
}

func TestCustomGuard_NilFunc(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("NewCustomGuard(nil) did not panic")
		}

		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is not a string: %v", r)
		}

		if !strings.Contains(msg, "non-nil function") {
			t.Errorf("panic message = %q, want to contain 'non-nil function'", msg)
		}
	}()

	NewCustomGuard("test", nil)
}
