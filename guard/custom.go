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

// CheckFunc is a custom guard check function type.
type CheckFunc func(msg *Message) (*Result, error)

// CustomGuard wraps a user-provided check function.
type CustomGuard struct {
	name string
	fn   CheckFunc
}

var _ Guard = (*CustomGuard)(nil)

// NewCustomGuard creates a CustomGuard. Panics if fn is nil.
func NewCustomGuard(name string, fn CheckFunc) *CustomGuard {
	if fn == nil {
		panic("vagent: NewCustomGuard requires a non-nil function")
	}

	return &CustomGuard{name: name, fn: fn}
}

func (g *CustomGuard) Name() string { return g.name }

func (g *CustomGuard) Check(msg *Message) (*Result, error) {
	return g.fn(msg)
}
