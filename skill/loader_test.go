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
	"os"
	"path/filepath"
	"testing"
)

func TestFileLoader_Load_ValidSkill(t *testing.T) {
	loader := &FileLoader{}
	ctx := context.Background()

	def, err := loader.Load(ctx, "testdata/valid-skill")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if def.Name != "valid-skill" {
		t.Errorf("Name = %q, want %q", def.Name, "valid-skill")
	}
	if def.Description != "A valid test skill" {
		t.Errorf("Description = %q, want %q", def.Description, "A valid test skill")
	}
	if def.License != "Apache-2.0" {
		t.Errorf("License = %q, want %q", def.License, "Apache-2.0")
	}
	if len(def.AllowedTools) != 2 {
		t.Fatalf("AllowedTools length = %d, want 2", len(def.AllowedTools))
	}
	if def.AllowedTools[0] != "tool-a" || def.AllowedTools[1] != "tool-b" {
		t.Errorf("AllowedTools = %v", def.AllowedTools)
	}
	if def.Metadata["version"] != "1.0" {
		t.Errorf("Metadata[version] = %q, want %q", def.Metadata["version"], "1.0")
	}
	if def.Instructions == "" {
		t.Error("Instructions should not be empty")
	}
	if def.BasePath != "testdata/valid-skill" {
		t.Errorf("BasePath = %q", def.BasePath)
	}

	// Should have resources from scripts/ and references/.
	if len(def.Resources) < 2 {
		t.Fatalf("Resources length = %d, want >= 2", len(def.Resources))
	}

	foundScript := false
	foundRef := false
	for _, r := range def.Resources {
		if r.Type == ResourceTypeScript && r.Name == "build.sh" {
			foundScript = true
		}
		if r.Type == ResourceTypeReference && r.Name == "guide.md" {
			foundRef = true
		}
	}
	if !foundScript {
		t.Error("missing script resource build.sh")
	}
	if !foundRef {
		t.Error("missing reference resource guide.md")
	}
}

func TestFileLoader_Load_MinimalSkill(t *testing.T) {
	loader := &FileLoader{}
	ctx := context.Background()

	def, err := loader.Load(ctx, "testdata/minimal-skill")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if def.Name != "minimal-skill" {
		t.Errorf("Name = %q, want %q", def.Name, "minimal-skill")
	}
	if def.Description != "Minimal skill" {
		t.Errorf("Description = %q, want %q", def.Description, "Minimal skill")
	}
	if def.License != "" {
		t.Errorf("License = %q, want empty", def.License)
	}
	if len(def.AllowedTools) != 0 {
		t.Errorf("AllowedTools = %v, want empty", def.AllowedTools)
	}
	if def.Instructions != "Just do it." {
		t.Errorf("Instructions = %q, want %q", def.Instructions, "Just do it.")
	}
	if len(def.Resources) != 0 {
		t.Errorf("Resources length = %d, want 0", len(def.Resources))
	}
}

func TestFileLoader_Load_NonexistentDir(t *testing.T) {
	loader := &FileLoader{}
	_, err := loader.Load(context.Background(), "testdata/nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestFileLoader_Load_MalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("no frontmatter"), 0o644)

	loader := &FileLoader{}
	_, err := loader.Load(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for malformed frontmatter")
	}
}

func TestFileLoader_Discover(t *testing.T) {
	loader := &FileLoader{}
	ctx := context.Background()

	skills, err := loader.Discover(ctx, "testdata")
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	// Should find valid-skill, minimal-skill, and bad-name (Discover parses but does not validate).
	if len(skills) != 3 {
		t.Fatalf("Discover found %d skills, want 3", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}

	if !names["valid-skill"] {
		t.Error("missing valid-skill")
	}
	if !names["minimal-skill"] {
		t.Error("missing minimal-skill")
	}
	if !names["Bad-Name"] {
		t.Error("missing Bad-Name (discover should parse but not validate)")
	}
}

func TestFileLoader_Discover_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	loader := &FileLoader{}

	skills, err := loader.Discover(context.Background(), dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("Discover found %d skills, want 0", len(skills))
	}
}

func TestFileLoader_Discover_NonexistentDir(t *testing.T) {
	loader := &FileLoader{}
	_, err := loader.Discover(context.Background(), "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantFM  string
		wantErr bool
	}{
		{
			name:   "normal",
			input:  "---\nname: test\n---\nbody",
			wantFM: "name: test",
		},
		{
			name:    "no delimiter",
			input:   "no frontmatter here",
			wantErr: true,
		},
		{
			name:    "no closing delimiter",
			input:   "---\nname: test\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, _, err := splitFrontmatter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && fm != tt.wantFM {
				t.Errorf("frontmatter = %q, want %q", fm, tt.wantFM)
			}
		})
	}
}
