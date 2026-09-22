// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package a2ui

import (
	"strings"
	"testing"

	tmca2ui "github.com/tmc/a2ui"
)

func TestSurfaceManager_MultiSurfaceLifecycle(t *testing.T) {
	sm := NewSurfaceManager()

	// 1. Create surface A
	sm.CreateSurface(&tmca2ui.CreateSurface{
		SurfaceID:     "surface-a",
		SendDataModel: true,
	})

	// 2. Create surface B
	sm.CreateSurface(&tmca2ui.CreateSurface{
		SurfaceID:     "surface-b",
		SendDataModel: false,
	})

	// 3. Add components to surface A with data binding and formatString
	textCompA := tmca2ui.Component{
		ID: "text-a",
		Text: &tmca2ui.TextComponent{
			Text: tmca2ui.DynamicString{
				FunctionCall: &tmca2ui.FunctionCall{
					Call: "formatString",
					Args: map[string]any{
						"template": "Hello ${name}",
						"name":     map[string]any{"path": "/user/name"},
					},
				},
			},
		},
	}

	cardCompA := tmca2ui.Component{
		ID:   "root",
		Card: &tmca2ui.CardComponent{Child: "text-a"},
	}

	// Update data model for surface A
	_, err := sm.UpdateDataModel(&tmca2ui.UpdateDataModel{
		SurfaceID: "surface-a",
		Path:      "/user/name",
		Value:     "Alice",
	})
	if err != nil {
		t.Fatalf("UpdateDataModel failed: %v", err)
	}

	modelA, err := sm.UpdateComponents(&tmca2ui.UpdateComponents{
		SurfaceID:  "surface-a",
		Components: []tmca2ui.Component{cardCompA, textCompA},
	})
	if err != nil {
		t.Fatalf("UpdateComponents failed: %v", err)
	}
	if modelA == nil {
		t.Fatal("expected modelA to be non-nil")
	}

	viewA := modelA.View().Content
	if !strings.Contains(viewA, "Hello Alice") {
		t.Errorf("expected viewA to contain 'Hello Alice', got: %s", viewA)
	}

	// 4. Update data model for surface A on subsequent turn
	modelA2, err := sm.UpdateDataModel(&tmca2ui.UpdateDataModel{
		SurfaceID: "surface-a",
		Path:      "/user/name",
		Value:     "Bob",
	})
	if err != nil {
		t.Fatalf("UpdateDataModel turn 2 failed: %v", err)
	}
	viewA2 := modelA2.View().Content
	if !strings.Contains(viewA2, "Hello Bob") {
		t.Errorf("expected viewA2 to contain 'Hello Bob', got: %s", viewA2)
	}

	// 5. Test ExportClientDataModel
	dmA := sm.ExportClientDataModel("surface-a")
	if dmA == nil {
		t.Fatal("expected non-nil client data model for surface-a")
	}
	surfs := dmA["surfaces"].(map[string]any)
	surfAData := surfs["surface-a"].(map[string]any)
	userMap := surfAData["user"].(map[string]any)
	if userMap["name"] != "Bob" {
		t.Errorf("expected exported user.name = Bob, got %v", userMap["name"])
	}

	// Surface B has SendDataModel: false
	dmB := sm.ExportClientDataModel("surface-b")
	if dmB != nil {
		t.Errorf("expected nil data model for surface-b because SendDataModel is false, got %v", dmB)
	}

	// 6. Delete surface A
	deleted := sm.DeleteSurface(&tmca2ui.DeleteSurface{SurfaceID: "surface-a"})
	if !deleted {
		t.Errorf("expected DeleteSurface to return true")
	}
	_, exists := sm.GetSurface("surface-a")
	if exists {
		t.Errorf("expected surface-a to be marked deleted")
	}
}

func TestSurfaceManager_ValidationCheckRule(t *testing.T) {
	sm := NewSurfaceManager()
	sm.CreateSurface(&tmca2ui.CreateSurface{SurfaceID: "form-surf"})

	// Input field with CheckRule: username must equal 'alice'
	field := tmca2ui.Component{
		ID: "username",
		TextField: &tmca2ui.TextFieldComponent{
			Label: tmca2ui.StringLiteral("Username"),
		},
		Checks: []tmca2ui.CheckRule{
			{
				Condition: tmca2ui.DynamicBoolean{
					FunctionCall: &tmca2ui.FunctionCall{
						Call: "equal",
						Args: map[string]any{
							"left":  map[string]any{"path": "username"},
							"right": "alice",
						},
					},
				},
				Message: "Username must be alice",
			},
		},
	}

	_, err := sm.UpdateComponents(&tmca2ui.UpdateComponents{
		SurfaceID:  "form-surf",
		Components: []tmca2ui.Component{field},
	})
	if err != nil {
		t.Fatalf("UpdateComponents failed: %v", err)
	}

	// Test failing validation
	valErr := sm.ValidateInput("form-surf", "username", "")
	if valErr == nil {
		t.Fatal("expected validation error for empty username")
	}
	if valErr.Code != "ValidationFailed" {
		t.Errorf("expected code ValidationFailed, got %q", valErr.Code)
	}
	if valErr.Message != "Username must be alice" {
		t.Errorf("expected message 'Username must be alice', got %q", valErr.Message)
	}
}
