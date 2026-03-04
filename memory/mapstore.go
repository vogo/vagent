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
	"strings"
	"time"
)

// storeRecord is the internal representation of a stored value inside MapStore.
type storeRecord struct {
	value     any
	createdAt time.Time
	ttl       int64
}

func (r *storeRecord) isExpired() bool {
	if r.ttl <= 0 {
		return false
	}
	return time.Since(r.createdAt) > time.Duration(r.ttl)*time.Second
}

// Compile-time checks.
var (
	_ Store      = (*MapStore)(nil)
	_ BatchStore = (*MapStore)(nil)
)

// MapStore is an in-memory Store backed by a plain Go map.
// It is not safe for concurrent use; callers must provide their own locking.
type MapStore struct {
	entries map[string]*storeRecord
}

// NewMapStore creates a new MapStore.
func NewMapStore() *MapStore {
	return &MapStore{entries: make(map[string]*storeRecord)}
}

func (s *MapStore) Get(_ context.Context, key string) (any, bool, error) {
	r, ok := s.entries[key]
	if !ok {
		return nil, false, nil
	}
	if r.isExpired() {
		delete(s.entries, key)
		return nil, false, nil
	}
	return r.value, true, nil
}

func (s *MapStore) Set(_ context.Context, key string, value any, ttl int64) error {
	s.entries[key] = &storeRecord{
		value:     value,
		createdAt: time.Now(),
		ttl:       ttl,
	}
	return nil
}

func (s *MapStore) Delete(_ context.Context, key string) error {
	delete(s.entries, key)
	return nil
}

func (s *MapStore) List(_ context.Context, prefix string) ([]StoreEntry, error) {
	result := make([]StoreEntry, 0, len(s.entries))
	for k, r := range s.entries {
		if r.isExpired() {
			delete(s.entries, k)
			continue
		}
		if strings.HasPrefix(k, prefix) {
			result = append(result, StoreEntry{
				Key:       k,
				Value:     r.value,
				CreatedAt: r.createdAt,
				TTL:       r.ttl,
			})
		}
	}
	return result, nil
}

func (s *MapStore) Clear(_ context.Context) error {
	s.entries = make(map[string]*storeRecord)
	return nil
}

func (s *MapStore) BatchGet(ctx context.Context, keys []string) (map[string]any, error) {
	result := make(map[string]any, len(keys))
	for _, key := range keys {
		if v, ok, _ := s.Get(ctx, key); ok {
			result[key] = v
		}
	}
	return result, nil
}

func (s *MapStore) BatchSet(_ context.Context, entries map[string]any, ttl int64) error {
	now := time.Now()
	for key, value := range entries {
		s.entries[key] = &storeRecord{
			value:     value,
			createdAt: now,
			ttl:       ttl,
		}
	}
	return nil
}

// SetCreatedAtForTest backdates the CreatedAt of the given key. Intended for
// testing TTL expiry without real sleeps.
func (s *MapStore) SetCreatedAtForTest(key string, t time.Time) {
	if r, ok := s.entries[key]; ok {
		r.createdAt = t
	}
}
