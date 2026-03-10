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
	"regexp"
	"time"
)

// Resource type constants.
const (
	ResourceTypeScript    = "script"
	ResourceTypeReference = "reference"
	ResourceTypeAsset     = "asset"
)

// nameRegex validates skill names: lowercase letters, digits, and hyphens only.
// No leading/trailing hyphens, no consecutive hyphens.
var nameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Def defines a skill's metadata and instructions.
type Def struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	License      string            `json:"license,omitempty"`
	AllowedTools []string          `json:"allowed_tools,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Instructions string            `json:"instructions"`
	BasePath     string            `json:"base_path,omitempty"`
	Resources    []Resource        `json:"resources,omitempty"`
}

// Resource describes a file resource associated with a skill.
type Resource struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

// Activation records an active skill for a session.
type Activation struct {
	SkillName   string
	SessionID   string
	ActivatedAt time.Time
	def         *Def // unexported to prevent external mutation
}

// Def returns a copy of the skill definition to prevent external mutation.
func (a *Activation) SkillDef() Def {
	if a.def == nil {
		return Def{}
	}
	return *a.def
}

// ValidateName checks that a skill name contains only lowercase letters, digits, and hyphens.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name must not be empty")
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid skill name %q: must match %s", name, nameRegex.String())
	}
	return nil
}
