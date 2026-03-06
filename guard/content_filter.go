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

import "strings"

// ContentFilterConfig configures the ContentFilterGuard.
type ContentFilterConfig struct {
	BlockedKeywords []string
	CaseSensitive   bool
}

// ContentFilterGuard filters harmful content by keyword matching.
type ContentFilterGuard struct {
	keywords      []string // stored normalized (lowered if case-insensitive)
	origKeywords  []string // original keywords for violation reporting
	caseSensitive bool
}

var _ Guard = (*ContentFilterGuard)(nil)

func NewContentFilterGuard(cfg ContentFilterConfig) *ContentFilterGuard {
	keywords := make([]string, len(cfg.BlockedKeywords))
	origKeywords := make([]string, len(cfg.BlockedKeywords))

	for i, kw := range cfg.BlockedKeywords {
		origKeywords[i] = kw
		if cfg.CaseSensitive {
			keywords[i] = kw
		} else {
			keywords[i] = strings.ToLower(kw)
		}
	}

	return &ContentFilterGuard{
		keywords:      keywords,
		origKeywords:  origKeywords,
		caseSensitive: cfg.CaseSensitive,
	}
}

func (g *ContentFilterGuard) Name() string { return "content_filter" }

func (g *ContentFilterGuard) Check(msg *Message) (*Result, error) {
	if len(g.keywords) == 0 {
		return Pass(), nil
	}

	text := msg.Content
	if !g.caseSensitive {
		text = strings.ToLower(msg.Content)
	}

	var violations []string

	for i, kw := range g.keywords {
		if strings.Contains(text, kw) {
			violations = append(violations, g.origKeywords[i])
		}
	}

	if len(violations) > 0 {
		return Block(g.Name(), "blocked content detected", violations...), nil
	}

	return Pass(), nil
}
