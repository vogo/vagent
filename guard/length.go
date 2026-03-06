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
	"fmt"
	"unicode/utf8"
)

// LengthConfig configures the LengthGuard.
type LengthConfig struct {
	MaxLength int // Maximum character count (rune count)
}

// LengthGuard limits text length.
type LengthGuard struct {
	maxLength int
}

var _ Guard = (*LengthGuard)(nil)

func NewLengthGuard(cfg LengthConfig) *LengthGuard {
	return &LengthGuard{maxLength: cfg.MaxLength}
}

func (g *LengthGuard) Name() string { return "length" }

func (g *LengthGuard) Check(msg *Message) (*Result, error) {
	if g.maxLength <= 0 {
		return Pass(), nil
	}

	runeCount := utf8.RuneCountInString(msg.Content)
	if runeCount > g.maxLength {
		return Block(g.Name(),
			fmt.Sprintf("content length %d exceeds maximum %d", runeCount, g.maxLength)), nil
	}

	return Pass(), nil
}
