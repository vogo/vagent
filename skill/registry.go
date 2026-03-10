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
	"fmt"
	"strings"
	"sync"
)

// Registry manages registered skill definitions.
type Registry interface {
	Register(def *Def) error
	Unregister(name string)
	Get(name string) (*Def, bool)
	List() []*Def
	Match(query string) []*Def
}

// RegistryOption configures an InMemoryRegistry during construction.
type RegistryOption func(*InMemoryRegistry)

// WithValidator sets the validator used when registering skills.
func WithValidator(v Validator) RegistryOption {
	return func(r *InMemoryRegistry) { r.validator = v }
}

// InMemoryRegistry is a thread-safe in-memory skill registry.
type InMemoryRegistry struct {
	mu        sync.RWMutex
	skills    map[string]*Def
	validator Validator
}

// Compile-time check: InMemoryRegistry implements Registry.
var _ Registry = (*InMemoryRegistry)(nil)

// NewRegistry creates an empty InMemoryRegistry with the given options.
func NewRegistry(opts ...RegistryOption) *InMemoryRegistry {
	r := &InMemoryRegistry{skills: make(map[string]*Def)}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Register validates and stores a skill definition. Rejects duplicates.
func (r *InMemoryRegistry) Register(def *Def) error {
	if def == nil {
		return fmt.Errorf("skill definition must not be nil")
	}

	if r.validator != nil {
		if err := r.validator.Validate(def); err != nil {
			return fmt.Errorf("validate skill %q: %w", def.Name, err)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[def.Name]; exists {
		return fmt.Errorf("skill %q already registered", def.Name)
	}

	r.skills[def.Name] = def
	return nil
}

// Unregister removes a skill by name. Silently succeeds if the name does not exist.
func (r *InMemoryRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
}

// Get returns the skill definition with the given name.
func (r *InMemoryRegistry) Get(name string) (*Def, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.skills[name]
	return def, ok
}

// List returns all registered skill definitions.
func (r *InMemoryRegistry) List() []*Def {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]*Def, 0, len(r.skills))
	for _, def := range r.skills {
		defs = append(defs, def)
	}
	return defs
}

// Match returns skills whose Name or Description contains all query words (case-insensitive, AND semantics).
func (r *InMemoryRegistry) Match(query string) []*Def {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return r.List()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*Def
	for _, def := range r.skills {
		nameLower := strings.ToLower(def.Name)
		descLower := strings.ToLower(def.Description)
		combined := nameLower + " " + descLower

		allMatch := true
		for _, w := range words {
			if !strings.Contains(combined, w) {
				allMatch = false
				break
			}
		}
		if allMatch {
			results = append(results, def)
		}
	}

	return results
}
