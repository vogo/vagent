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
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillFileName = "SKILL.md"

// Loader loads skill definitions from the filesystem.
type Loader interface {
	Load(ctx context.Context, path string) (*Def, error)
	Discover(ctx context.Context, dir string) ([]*Def, error)
}

// frontmatter holds the YAML frontmatter fields parsed from a SKILL.md file.
type frontmatter struct {
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	License      string            `yaml:"license"`
	AllowedTools []string          `yaml:"allowed_tools"`
	Metadata     map[string]string `yaml:"metadata"`
}

// FileLoader loads skills from the filesystem.
type FileLoader struct{}

// Compile-time check: FileLoader implements Loader.
var _ Loader = (*FileLoader)(nil)

// Load reads a SKILL.md from the given directory path and returns a Def.
func (l *FileLoader) Load(_ context.Context, path string) (*Def, error) {
	skillFile := filepath.Join(path, skillFileName)

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("read skill file %q: %w", skillFile, err)
	}

	def, err := parseSkillFile(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse skill file %q: %w", skillFile, err)
	}

	def.BasePath = path

	// Scan for resources.
	def.Resources = scanResources(path)

	return def, nil
}

// Discover scans a directory for subdirectories containing SKILL.md files
// and returns all successfully parsed skills.
func (l *FileLoader) Discover(ctx context.Context, dir string) ([]*Def, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read skill directory %q: %w", dir, err)
	}

	var skills []*Def

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		subDir := filepath.Join(dir, e.Name())
		skillFile := filepath.Join(subDir, skillFileName)

		if _, statErr := os.Stat(skillFile); statErr != nil {
			continue
		}

		def, loadErr := l.Load(ctx, subDir)
		if loadErr != nil {
			slog.Warn("skill: skip loading", "dir", subDir, "error", loadErr)
			continue
		}

		skills = append(skills, def)
	}

	return skills, nil
}

// parseSkillFile parses a SKILL.md file content into a Def.
func parseSkillFile(content string) (*Def, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var meta frontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return &Def{
		Name:         meta.Name,
		Description:  meta.Description,
		License:      meta.License,
		AllowedTools: meta.AllowedTools,
		Metadata:     meta.Metadata,
		Instructions: strings.TrimSpace(body),
	}, nil
}

// splitFrontmatter splits a SKILL.md file into YAML frontmatter and markdown body.
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return "", "", fmt.Errorf("missing frontmatter delimiter")
	}

	// Find the closing delimiter.
	rest := content[3:]
	before, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", "", fmt.Errorf("missing closing frontmatter delimiter")
	}

	fm := strings.TrimSpace(before)
	body := after // skip "\n---"

	return fm, body, nil
}

// scanResources scans the scripts/, references/, and assets/ subdirectories for resources.
func scanResources(basePath string) []Resource {
	dirs := map[string]string{
		"scripts":    ResourceTypeScript,
		"references": ResourceTypeReference,
		"assets":     ResourceTypeAsset,
	}

	var resources []Resource

	for dirName, resType := range dirs {
		dirPath := filepath.Join(basePath, dirName)

		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue // directory doesn't exist
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}

			resources = append(resources, Resource{
				Type: resType,
				Name: e.Name(),
				Path: filepath.Join(dirPath, e.Name()),
			})
		}
	}

	return resources
}
