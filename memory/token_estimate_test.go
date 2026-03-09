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
	"strings"
	"testing"

	"github.com/vogo/vagent/schema"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		msg      schema.Message
		expected int
	}{
		{
			name:     "empty content",
			msg:      schema.NewUserMessage(""),
			expected: 0,
		},
		{
			name:     "single char",
			msg:      schema.NewUserMessage("a"),
			expected: 1,
		},
		{
			name:     "short text 5 chars",
			msg:      schema.NewUserMessage("hello"),
			expected: 1,
		},
		{
			name:     "8 char text",
			msg:      schema.NewUserMessage("abcdefgh"),
			expected: 2,
		},
		{
			name:     "40 char text",
			msg:      schema.NewUserMessage(strings.Repeat("a", 40)),
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultTokenEstimator(tt.msg)
			if got != tt.expected {
				t.Errorf("estimateTokens() = %d, want %d", got, tt.expected)
			}
		})
	}
}
