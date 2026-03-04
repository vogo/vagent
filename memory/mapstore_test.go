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

package memory

import (
	"context"
	"testing"
	"time"
)

func TestMapStore_GetSetDelete(t *testing.T) {
	s := NewMapStore()
	ctx := context.Background()

	// Miss.
	v, ok, err := s.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ok || v != nil {
		t.Fatalf("Get miss: got ok=%v v=%v", ok, v)
	}

	// Set + hit.
	if err := s.Set(ctx, "k", "hello", 0); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	v, ok, err = s.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !ok || v != "hello" {
		t.Fatalf("Get hit: got ok=%v v=%v", ok, v)
	}

	// Delete + miss.
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	v, ok, _ = s.Get(ctx, "k")
	if ok || v != nil {
		t.Fatalf("Get after delete: got ok=%v v=%v", ok, v)
	}
}

func TestMapStore_ListPrefix(t *testing.T) {
	s := NewMapStore()
	ctx := context.Background()

	_ = s.Set(ctx, "msg:1", "a", 0)
	_ = s.Set(ctx, "msg:2", "b", 0)
	_ = s.Set(ctx, "other", "c", 0)

	entries, err := s.List(ctx, "msg:")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("List len = %d, want 2", len(entries))
	}
}

func TestMapStore_Clear(t *testing.T) {
	s := NewMapStore()
	ctx := context.Background()

	_ = s.Set(ctx, "a", 1, 0)
	_ = s.Set(ctx, "b", 2, 0)
	_ = s.Clear(ctx)

	entries, _ := s.List(ctx, "")
	if len(entries) != 0 {
		t.Fatalf("List after Clear len = %d, want 0", len(entries))
	}
}

func TestMapStore_BatchGetSet(t *testing.T) {
	s := NewMapStore()
	ctx := context.Background()

	batch := map[string]any{"x": 1, "y": 2}
	if err := s.BatchSet(ctx, batch, 0); err != nil {
		t.Fatalf("BatchSet error: %v", err)
	}

	result, err := s.BatchGet(ctx, []string{"x", "y", "z"})
	if err != nil {
		t.Fatalf("BatchGet error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("BatchGet len = %d, want 2", len(result))
	}
	if result["x"] != 1 {
		t.Errorf("BatchGet[x] = %v, want 1", result["x"])
	}
}

func TestMapStore_TTLExpiry(t *testing.T) {
	s := NewMapStore()
	ctx := context.Background()

	_ = s.Set(ctx, "k", "v", 1)
	s.SetCreatedAtForTest("k", time.Now().Add(-2*time.Second))

	v, ok, _ := s.Get(ctx, "k")
	if ok || v != nil {
		t.Errorf("expired entry should be gone: ok=%v v=%v", ok, v)
	}
}

func TestMapStore_TTLExpiryList(t *testing.T) {
	s := NewMapStore()
	ctx := context.Background()

	_ = s.Set(ctx, "alive", "yes", 0)
	_ = s.Set(ctx, "dead", "no", 1)
	s.SetCreatedAtForTest("dead", time.Now().Add(-2*time.Second))

	entries, _ := s.List(ctx, "")
	if len(entries) != 1 {
		t.Fatalf("List len = %d, want 1", len(entries))
	}
	if entries[0].Key != "alive" {
		t.Errorf("entry key = %q, want %q", entries[0].Key, "alive")
	}
}

func TestMapStore_SetCreatedAtForTest_Missing(t *testing.T) {
	s := NewMapStore()
	// Should be a no-op, not panic.
	s.SetCreatedAtForTest("missing", time.Now())
}
