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
)

func TestReadJSONLStream_LineByLine(t *testing.T) {
	rawStream := `{"createSurface": {"surfaceId": "surf1", "catalogId": "basic.json"}}
Here is some conversational text
{"updateComponents": {"surfaceId": "surf1", "components": [{"id": "root", "component": "Text", "text": "Hello"}]}}
{"updateDataModel": {"surfaceId": "surf1", "path": "/user", "value": {"name": "Bob"}}}
{"deleteSurface": {"surfaceId": "surf1"}}
`

	events, err := ReadJSONLStream(strings.NewReader(rawStream))
	if err != nil {
		t.Fatalf("ReadJSONLStream failed: %v", err)
	}

	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// 1. CreateSurface
	if !events[0].IsA2UI || events[0].Message.CreateSurface == nil || events[0].Message.CreateSurface.SurfaceID != "surf1" {
		t.Errorf("expected CreateSurface for surf1, got %+v", events[0])
	}

	// 2. Text line
	if events[1].IsA2UI || events[1].Text != "Here is some conversational text" {
		t.Errorf("expected text line, got %+v", events[1])
	}

	// 3. UpdateComponents
	if !events[2].IsA2UI || events[2].Message.UpdateComponents == nil || events[2].Message.UpdateComponents.SurfaceID != "surf1" {
		t.Errorf("expected UpdateComponents for surf1, got %+v", events[2])
	}

	// 4. UpdateDataModel
	if !events[3].IsA2UI || events[3].Message.UpdateDataModel == nil || events[3].Message.UpdateDataModel.Path != "/user" {
		t.Errorf("expected UpdateDataModel, got %+v", events[3])
	}

	// 5. DeleteSurface
	if !events[4].IsA2UI || events[4].Message.DeleteSurface == nil || events[4].Message.DeleteSurface.SurfaceID != "surf1" {
		t.Errorf("expected DeleteSurface, got %+v", events[4])
	}
}

func TestExtractMessagesAndText_TaggedAndProse(t *testing.T) {
	content := `Here is your dashboard:
<a2ui-json>
[
  {"createSurface": {"surfaceId": "dash", "catalogId": "basic.json"}},
  {"updateComponents": {"surfaceId": "dash", "components": [{"id": "root", "component": "Text", "text": "Dashboard"}]}}
]
</a2ui-json>
Let me know if you need changes.`

	msgs, prose := ExtractMessagesAndText(content)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if !strings.Contains(prose, "Here is your dashboard:") || !strings.Contains(prose, "Let me know if you need changes.") {
		t.Errorf("expected prose to contain both intro and outro, got %q", prose)
	}
}
