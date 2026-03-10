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
	"os"
	"strings"
)

// MaxInstructionLines is the maximum number of lines allowed in skill instructions.
const MaxInstructionLines = 500

// Validator validates a Def.
type Validator interface {
	Validate(def *Def) error
}

// NameValidator validates the skill name format.
type NameValidator struct{}

func (NameValidator) Validate(def *Def) error {
	if def == nil {
		return fmt.Errorf("skill definition must not be nil")
	}
	return ValidateName(def.Name)
}

// SizeValidator checks that the instructions do not exceed MaxInstructionLines.
type SizeValidator struct{}

func (SizeValidator) Validate(def *Def) error {
	if def == nil {
		return fmt.Errorf("skill definition must not be nil")
	}
	if def.Instructions == "" {
		return nil
	}
	lines := strings.Count(def.Instructions, "\n") + 1
	if lines > MaxInstructionLines {
		return fmt.Errorf("skill %q instructions have %d lines, max %d", def.Name, lines, MaxInstructionLines)
	}
	return nil
}

// StructureValidator checks that only allowed subdirectories exist under BasePath.
type StructureValidator struct{}

func (StructureValidator) Validate(def *Def) error {
	if def == nil {
		return fmt.Errorf("skill definition must not be nil")
	}
	if def.BasePath == "" {
		return nil
	}

	entries, err := os.ReadDir(def.BasePath)
	if err != nil {
		return fmt.Errorf("read skill directory %q: %w", def.BasePath, err)
	}

	allowed := map[string]bool{
		"scripts":    true,
		"references": true,
		"assets":     true,
		"SKILL.md":   true,
	}

	for _, e := range entries {
		if !allowed[e.Name()] {
			return fmt.Errorf("skill %q has unexpected entry %q in %s", def.Name, e.Name(), def.BasePath)
		}
	}

	return nil
}

// CompositeValidator chains multiple validators and returns the first error.
type CompositeValidator struct {
	validators []Validator
}

// NewCompositeValidator creates a CompositeValidator from the given validators.
func NewCompositeValidator(vs ...Validator) *CompositeValidator {
	return &CompositeValidator{validators: vs}
}

func (c *CompositeValidator) Validate(def *Def) error {
	for _, v := range c.validators {
		if err := v.Validate(def); err != nil {
			return err
		}
	}
	return nil
}

// DefaultValidator returns the default composite validator chain.
func DefaultValidator() *CompositeValidator {
	return NewCompositeValidator(NameValidator{}, SizeValidator{}, StructureValidator{})
}
