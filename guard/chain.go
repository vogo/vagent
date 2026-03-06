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
	"context"
	"fmt"
)

// RunGuards sequentially executes the guard chain.
//   - Empty chain returns Pass().
//   - Any guard returning error interrupts and returns that error.
//   - Any guard returning block interrupts and returns that Result.
//   - Guard returning rewrite replaces msg.Content, continues to next guard.
//   - If any rewrites occurred, returns a Rewrite result with final content.
//   - All guards passing with no rewrites returns Pass().
//   - Returns error if a guard returns nil result or an unknown Action.
func RunGuards(ctx context.Context, msg *Message, guards ...Guard) (*Result, error) {
	if len(guards) == 0 {
		return Pass(), nil
	}

	var rewritten bool
	var allViolations []string

	for _, g := range guards {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		result, err := g.Check(msg)
		if err != nil {
			return nil, err
		}

		if result == nil {
			return nil, fmt.Errorf("vagent: guard %q returned nil result", g.Name())
		}

		switch result.Action {
		case ActionBlock:
			return result, nil
		case ActionRewrite:
			msg.Content = result.Content
			rewritten = true
			allViolations = append(allViolations, result.Violations...)
		case ActionPass:
			// continue
		default:
			return nil, fmt.Errorf("vagent: guard %q returned unknown action %q", g.Name(), result.Action)
		}
	}

	if rewritten {
		return &Result{
			Action:     ActionRewrite,
			Content:    msg.Content,
			Reason:     "content modified by guards",
			Violations: allViolations,
		}, nil
	}

	return Pass(), nil
}
