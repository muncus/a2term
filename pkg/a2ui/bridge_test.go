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

package a2ui_test

import (
	"strings"
	"testing"

	"github.com/joestump-agent/a2tea/event"
	tmca2ui "github.com/tmc/a2ui"

	"a2term/pkg/a2ui"
)

const sampleAgentResponse = `Here is the requested information:

<a2ui-json>
{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "weather-card",
    "components": [
      { "component": "Card", "id": "root", "child": "col" },
      { "component": "Column", "id": "col", "children": ["title", "temp", "btn"] },
      { "component": "Text", "id": "title", "text": "San Francisco Weather", "variant": "h1" },
      { "component": "Text", "id": "temp", "text": "68°F Partly Cloudy" },
      { "component": "Button", "id": "btn", "action": { "event": { "name": "refreshWeather", "context": { "city": "SF" } } }, "child": "btnText" },
      { "component": "Text", "id": "btnText", "text": "Refresh" }
    ]
  }
}
</a2ui-json>

Let me know if you need another city!`

func TestParseAgentResponse(t *testing.T) {
	segments, err := a2ui.ParseAgentResponse(sampleAgentResponse)
	if err != nil {
		t.Fatalf("ParseAgentResponse error: %v", err)
	}

	if len(segments) == 0 {
		t.Fatal("expected parsed segments, got empty")
	}

	hasText := false
	hasSurface := false

	for _, seg := range segments {
		if seg.Type == a2ui.TypeText && strings.Contains(seg.Text, "Here is the requested information") {
			hasText = true
		}
		if seg.Type == a2ui.TypeSurface && seg.Surface != nil {
			hasSurface = true
			out := seg.Surface.View().Content
			if !strings.Contains(out, "San Francisco Weather") {
				t.Errorf("surface does not contain title text, got: %q", out)
			}
		}
	}

	if !hasText {
		t.Error("expected text segment in parsed response")
	}
	if !hasSurface {
		t.Error("expected surface segment in parsed response")
	}
}

func TestParsePlainText(t *testing.T) {
	plain := "Hello world, this is just a regular response without any UI."
	segments, err := a2ui.ParseAgentResponse(plain)
	if err != nil {
		t.Fatalf("ParseAgentResponse error: %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}

	if segments[0].Type != a2ui.TypeText || segments[0].Text != plain {
		t.Errorf("unexpected segment: %+v", segments[0])
	}
}

func TestActionSummary(t *testing.T) {
	btnEvent := event.ButtonClicked{
		Source: event.Source{ComponentID: "submitBtn"},
		Action: &tmca2ui.EventAction{Name: "submitForm"},
	}

	name, src, summary := a2ui.ActionSummary(btnEvent)
	if name != "submitForm" {
		t.Errorf("expected name 'submitForm', got %q", name)
	}
	if src != "submitBtn" {
		t.Errorf("expected src 'submitBtn', got %q", src)
	}
	if !strings.Contains(summary, "submitForm") {
		t.Errorf("expected summary to contain 'submitForm', got %q", summary)
	}

	clientMsg := tmca2ui.ClientMessage{
		Action: &tmca2ui.ActionEvent{
			Name:              "vote",
			SourceComponentID: "btnVote",
			Context:           map[string]any{"choice": "Go"},
		},
	}

	name, src, summary = a2ui.ActionSummary(clientMsg)
	if name != "vote" || src != "btnVote" {
		t.Errorf("unexpected client message summary: %s, %s, %s", name, src, summary)
	}
}
