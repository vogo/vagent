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

import "testing"

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"pdf-processing", false},
		{"my-skill", false},
		{"skill123", false},
		{"a", false},
		{"a1b2", false},
		{"my-cool-skill", false},
		{"", true},
		{"My-Skill", true},
		{"my skill", true},
		{"-leading", true},
		{"trailing-", true},
		{"double--hyphen", true},
		{"UPPERCASE", true},
		{"has_underscore", true},
		{"has.dot", true},
		{"has/slash", true},
		{"has@special", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestDef_Fields(t *testing.T) {
	def := Def{
		Name:         "test-skill",
		Description:  "A test skill",
		License:      "Apache-2.0",
		AllowedTools: []string{"tool-a", "tool-b"},
		Metadata:     map[string]string{"version": "1.0"},
		Instructions: "Do something.",
		BasePath:     "/tmp/test",
		Resources: []Resource{
			{Type: ResourceTypeScript, Name: "build.sh", Path: "/tmp/test/scripts/build.sh"},
		},
	}

	if def.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", def.Name, "test-skill")
	}
	if len(def.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(def.Resources))
	}
	if def.Resources[0].Type != ResourceTypeScript {
		t.Errorf("Resource Type = %q, want %q", def.Resources[0].Type, ResourceTypeScript)
	}
}

func TestResourceTypeConstants(t *testing.T) {
	if ResourceTypeScript != "script" {
		t.Errorf("ResourceTypeScript = %q", ResourceTypeScript)
	}
	if ResourceTypeReference != "reference" {
		t.Errorf("ResourceTypeReference = %q", ResourceTypeReference)
	}
	if ResourceTypeAsset != "asset" {
		t.Errorf("ResourceTypeAsset = %q", ResourceTypeAsset)
	}
}
