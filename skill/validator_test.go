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

package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNameValidator(t *testing.T) {
	v := NameValidator{}

	if err := v.Validate(&Def{Name: "valid-name"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := v.Validate(&Def{Name: ""}); err == nil {
		t.Error("expected error for empty name")
	}
	if err := v.Validate(&Def{Name: "Bad-Name"}); err == nil {
		t.Error("expected error for uppercase name")
	}
	if err := v.Validate(nil); err == nil {
		t.Error("expected error for nil definition")
	}
}

func TestSizeValidator(t *testing.T) {
	v := SizeValidator{}

	if err := v.Validate(&Def{Name: "ok", Instructions: "one line"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := v.Validate(&Def{Name: "ok", Instructions: ""}); err != nil {
		t.Errorf("unexpected error for empty instructions: %v", err)
	}
	if err := v.Validate(nil); err == nil {
		t.Error("expected error for nil definition")
	}

	// Exactly 500 lines should pass.
	lines500 := strings.Repeat("line\n", 499) + "last"
	if err := v.Validate(&Def{Name: "ok", Instructions: lines500}); err != nil {
		t.Errorf("unexpected error for 500 lines: %v", err)
	}

	// 501 lines should fail.
	lines501 := strings.Repeat("line\n", 500) + "last"
	if err := v.Validate(&Def{Name: "big", Instructions: lines501}); err == nil {
		t.Error("expected error for 501 lines")
	}
}

func TestStructureValidator(t *testing.T) {
	v := StructureValidator{}

	// Empty BasePath should pass.
	if err := v.Validate(&Def{Name: "ok"}); err != nil {
		t.Errorf("unexpected error for empty BasePath: %v", err)
	}

	// Nil should fail.
	if err := v.Validate(nil); err == nil {
		t.Error("expected error for nil definition")
	}

	// Valid structure.
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "references"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("test"), 0o644)

	if err := v.Validate(&Def{Name: "ok", BasePath: dir}); err != nil {
		t.Errorf("unexpected error for valid structure: %v", err)
	}

	// Invalid structure with unexpected directory.
	_ = os.MkdirAll(filepath.Join(dir, "unexpected"), 0o755)
	if err := v.Validate(&Def{Name: "bad", BasePath: dir}); err == nil {
		t.Error("expected error for unexpected directory")
	}
}

func TestCompositeValidator(t *testing.T) {
	cv := NewCompositeValidator(NameValidator{}, SizeValidator{})

	if err := cv.Validate(&Def{Name: "ok", Instructions: "hello"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// First validator fails.
	if err := cv.Validate(&Def{Name: "BAD"}); err == nil {
		t.Error("expected error from name validator")
	}

	// Second validator fails.
	bigInstr := strings.Repeat("line\n", 500) + "last"
	if err := cv.Validate(&Def{Name: "ok", Instructions: bigInstr}); err == nil {
		t.Error("expected error from size validator")
	}
}

func TestDefaultValidator(t *testing.T) {
	v := DefaultValidator()
	if err := v.Validate(&Def{Name: "test-skill", Instructions: "hello"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
