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
	"sync"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	def := &Def{Name: "my-skill", Description: "test"}

	if err := r.Register(def); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	got, ok := r.Get("my-skill")
	if !ok {
		t.Fatal("Get returned false")
	}
	if got.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", got.Name, "my-skill")
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get returned true for nonexistent skill")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	def := &Def{Name: "dup", Description: "test"}

	if err := r.Register(def); err != nil {
		t.Fatalf("first Register error: %v", err)
	}
	if err := r.Register(def); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Fatal("expected error for nil definition")
	}
}

func TestRegistry_RegisterWithValidator(t *testing.T) {
	r := NewRegistry(WithValidator(NameValidator{}))

	if err := r.Register(&Def{Name: "valid-name"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Register(&Def{Name: "BAD"}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Def{Name: "to-remove"})

	r.Unregister("to-remove")
	_, ok := r.Get("to-remove")
	if ok {
		t.Error("skill should be removed after Unregister")
	}

	// Unregister nonexistent should not panic.
	r.Unregister("nonexistent")
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Def{Name: "skill-a"})
	_ = r.Register(&Def{Name: "skill-b"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List length = %d, want 2", len(list))
	}
}

func TestRegistry_Match(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Def{Name: "pdf-processing", Description: "Process PDF files"})
	_ = r.Register(&Def{Name: "csv-parser", Description: "Parse CSV data"})
	_ = r.Register(&Def{Name: "pdf-reader", Description: "Read PDF documents"})

	// Match single word.
	results := r.Match("pdf")
	if len(results) != 2 {
		t.Errorf("Match('pdf') = %d results, want 2", len(results))
	}

	// Match multiple words (AND).
	results = r.Match("pdf process")
	if len(results) != 1 {
		t.Errorf("Match('pdf process') = %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].Name != "pdf-processing" {
		t.Errorf("Match('pdf process')[0].Name = %q, want %q", results[0].Name, "pdf-processing")
	}

	// Match no results.
	results = r.Match("excel")
	if len(results) != 0 {
		t.Errorf("Match('excel') = %d results, want 0", len(results))
	}

	// Empty query returns all.
	results = r.Match("")
	if len(results) != 3 {
		t.Errorf("Match('') = %d results, want 3", len(results))
	}
}

func TestRegistry_Match_CaseInsensitive(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Def{Name: "my-skill", Description: "A PDF Tool"})

	results := r.Match("PDF")
	if len(results) != 1 {
		t.Errorf("case-insensitive match failed: got %d results", len(results))
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "skill-" + string(rune('a'+i%26))
			_ = r.Register(&Def{Name: name})
			r.Get(name)
			r.List()
			r.Match("skill")
			r.Unregister(name)
		}(i)
	}
	wg.Wait()
}
