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

package ui_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/joestump-agent/a2tea/event"
	"github.com/muncus/a2term/pkg/agent"
	"github.com/muncus/a2term/pkg/ui"
	tmca2ui "github.com/tmc/a2ui"
)

type recordedAction struct {
	ActionName      string
	SurfaceID       string
	SourceID        string
	ContextValues   map[string]any
	ClientDataModel map[string]any
}

type a2uiTestClient struct {
	mu      sync.Mutex
	actions []recordedAction
}

var _ agent.Client = (*a2uiTestClient)(nil)

func (c *a2uiTestClient) SendMessage(ctx context.Context, text string) ([]*a2a.Part, error) {
	return []*a2a.Part{a2a.NewTextPart("ok")}, nil
}

func (c *a2uiTestClient) StreamMessage(ctx context.Context, text string, onChunk func(parts []*a2a.Part, isFinal bool, err error)) error {
	return nil
}

func (c *a2uiTestClient) SendActionEvent(ctx context.Context, actionName, sourceID string, contextValues map[string]any) ([]*a2a.Part, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actions = append(c.actions, recordedAction{
		ActionName:    actionName,
		SourceID:      sourceID,
		ContextValues: contextValues,
	})
	return []*a2a.Part{a2a.NewTextPart("action ok")}, nil
}

func (c *a2uiTestClient) SendA2UIAction(ctx context.Context, actionName, surfaceID, sourceID string, contextValues map[string]any, clientDataModel map[string]any) ([]*a2a.Part, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actions = append(c.actions, recordedAction{
		ActionName:      actionName,
		SurfaceID:       surfaceID,
		SourceID:        sourceID,
		ContextValues:   contextValues,
		ClientDataModel: clientDataModel,
	})
	return []*a2a.Part{a2a.NewTextPart("a2ui action ok")}, nil
}

func (c *a2uiTestClient) ResetSession()                          {}
func (c *a2uiTestClient) AgentName() string                      { return "test-agent" }
func (c *a2uiTestClient) CurrentSession() agent.SessionInfo      { return agent.SessionInfo{} }

func executeCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, bcmd := range batch {
			executeCmd(bcmd)
		}
	}
}

func TestA2UI_Checklist_MultiSurfaceAndProgressiveStreaming(t *testing.T) {
	cli := &a2uiTestClient{}
	m := ui.NewModel(ui.Config{
		Client:   cli,
		AgentURL: "http://localhost:9999",
	})
	resM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = resM.(ui.Model)

	// 1. Stream line 1: createSurface for "surf-main" with sendDataModel: true
	chunk1 := `{"createSurface": {"surfaceId": "surf-main", "sendDataModel": true}}`
	resM, _ = m.Update(ui.NewAgentStreamChunkMsgForTest(chunk1, false))
	m = resM.(ui.Model)

	// 2. Stream line 2: updateDataModel with user name
	chunk2 := `{"updateDataModel": {"surfaceId": "surf-main", "path": "/user/name", "value": "Ada Lovelace"}}`
	resM, _ = m.Update(ui.NewAgentStreamChunkMsgForTest(chunk2, false))
	m = resM.(ui.Model)

	// 3. Stream line 3: updateComponents with formatString interpolation
	chunk3 := `{"updateComponents": {"surfaceId": "surf-main", "components": [
		{"id": "root", "component": "Card", "child": "col"},
		{"id": "col", "component": "Column", "children": ["title", "btn"]},
		{"id": "title", "component": "Text", "text": {"call": "formatString", "args": {"template": "Welcome, ${name}!", "name": {"path": "/user/name"}}}},
		{"id": "btn", "component": "Button", "child": "btn-lbl"},
		{"id": "btn-lbl", "component": "Text", "text": "Proceed"}
	]}}`
	resM, _ = m.Update(ui.NewAgentStreamChunkMsgForTest(chunk3, false))
	m = resM.(ui.Model)

	// Progressive rendering check: surface should already be rendered before isFinal!
	indices := m.FindSurfaceIndices()
	if len(indices) == 0 {
		t.Fatal("expected surface to be rendered progressively before isFinal")
	}

	// Switch to TabSurfaces to check rendered surface content
	resM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	m = resM.(ui.Model)

	vpView := m.View().Content
	if !strings.Contains(vpView, "Welcome, Ada Lovelace!") {
		t.Errorf("expected view to contain resolved 'Welcome, Ada Lovelace!', got:\n%s", vpView)
	}

	// Switch back to TabChat for conversational outro
	resM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	m = resM.(ui.Model)

	// 4. Stream final chunk with conversational outro
	chunkFinal := `Here is your card.`
	resM, _ = m.Update(ui.NewAgentStreamChunkMsgForTest(chunkFinal, true))
	m = resM.(ui.Model)

	vpViewFinal := m.View().Content
	if !strings.Contains(vpViewFinal, "Here is your card.") {
		t.Errorf("expected final view to contain conversational text, got:\n%s", vpViewFinal)
	}

	// 5. Test multi-surface: agent sends update for a second surface "surf-aux"
	auxMsg := `<a2ui-json>
[
  {"createSurface": {"surfaceId": "surf-aux"}},
  {"updateComponents": {"surfaceId": "surf-aux", "components": [
    {"id": "root", "component": "Text", "text": "Auxiliary Surface"}
  ]}}
]
</a2ui-json>`
	resM, _ = m.Update(ui.NewAgentResponseMsgForTest(auxMsg))
	m = resM.(ui.Model)

	indicesAfterAux := m.FindSurfaceIndices()
	if len(indicesAfterAux) != 2 {
		t.Fatalf("expected 2 surfaces in feed, got %d", len(indicesAfterAux))
	}

	// 6. Test deleteSurface for "surf-aux"
	delMsg := `{"deleteSurface": {"surfaceId": "surf-aux"}}`
	resM, _ = m.Update(ui.NewAgentResponseMsgForTest(delMsg))
	m = resM.(ui.Model)

	indicesAfterDel := m.FindSurfaceIndices()
	if len(indicesAfterDel) != 1 {
		t.Fatalf("expected 1 surface remaining after deleting aux, got %d", len(indicesAfterDel))
	}

	// 7. Test client action dispatch with clientDataModel
	// Trigger button action from surf-main
	actionMsg := tmca2ui.ClientMessage{
		Version: "v0.9",
		Action: &tmca2ui.ActionEvent{
			Name:              "submit_proceed",
			SurfaceID:         "surf-main",
			SourceComponentID: "btn",
			Context:           map[string]any{"choice": "yes"},
		},
	}
	resM, cmd := m.Update(actionMsg)
	m = resM.(ui.Model)
	if cmd == nil {
		t.Fatal("expected tea.Cmd from action dispatch")
	}

	// Execute cmd
	executeCmd(cmd)

	cli.mu.Lock()
	defer cli.mu.Unlock()
	if len(cli.actions) != 1 {
		t.Fatalf("expected 1 recorded action, got %d", len(cli.actions))
	}
	act := cli.actions[0]
	if act.ActionName != "submit_proceed" || act.SurfaceID != "surf-main" {
		t.Errorf("unexpected action parameters: %+v", act)
	}
	if act.ClientDataModel == nil {
		t.Fatal("expected non-nil clientDataModel because sendDataModel was true")
	}
	surfs := act.ClientDataModel["surfaces"].(map[string]any)
	mainData := surfs["surf-main"].(map[string]any)
	userData := mainData["user"].(map[string]any)
	if userData["name"] != "Ada Lovelace" {
		t.Errorf("expected exported user name 'Ada Lovelace', got %v", userData["name"])
	}
}

func TestA2UI_Checklist_ClientValidationBeforeDispatch(t *testing.T) {
	cli := &a2uiTestClient{}
	m := ui.NewModel(ui.Config{
		Client:   cli,
		AgentURL: "http://localhost:9999",
	})
	resM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = resM.(ui.Model)

	// Surface with a TextField with CheckRule requiring value to equal 'correct_pass'
	surfaceSetup := `<a2ui-json>
[
  {"createSurface": {"surfaceId": "auth-surf"}},
  {"updateComponents": {"surfaceId": "auth-surf", "components": [
    {
      "id": "password_input",
      "component": "TextField",
      "label": "Password",
      "checks": [
        {
          "condition": {
            "call": "equal",
            "args": {
              "left": {"path": "password_input"},
              "right": "correct_pass"
            }
          },
          "message": "Invalid password"
        }
      ]
    }
  ]}}
]
</a2ui-json>`

	resM, _ = m.Update(ui.NewAgentResponseMsgForTest(surfaceSetup))
	m = resM.(ui.Model)

	// Attempt action dispatch with bad password
	badAction := tmca2ui.ClientMessage{
		Version: "v0.9",
		Action: &tmca2ui.ActionEvent{
			Name:              "login",
			SurfaceID:         "auth-surf",
			SourceComponentID: "password_input",
			Context:           map[string]any{"password_input": "wrong_pass"},
		},
	}

	resM, cmd := m.Update(badAction)
	m = resM.(ui.Model)

	// Action should NOT have dispatched any command to server
	if cmd != nil {
		// Run batch cmd if any, ensuring no SendA2UIAction was emitted
		_ = cmd()
	}

	cli.mu.Lock()
	actionCount := len(cli.actions)
	cli.mu.Unlock()

	if actionCount > 0 {
		t.Errorf("expected 0 actions sent to server on validation failure, got %d", actionCount)
	}

	// Verify validation error is reported in viewport/toast
	vpContent := m.View().Content
	if !strings.Contains(vpContent, "Invalid password") {
		t.Errorf("expected viewport to contain 'Invalid password', got:\n%s", vpContent)
	}
}

func TestA2UI_Checklist_OpenURLClientAction(t *testing.T) {
	var openedURLs []string
	cleanup := ui.SetOpenBrowserFuncForTest(func(targetURL string) error {
		openedURLs = append(openedURLs, targetURL)
		return nil
	})
	defer cleanup()

	cli := &a2uiTestClient{}
	m := ui.NewModel(ui.Config{
		Client:   cli,
		AgentURL: "http://localhost:9999",
	})
	resM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = resM.(ui.Model)

	// Surface with an OpenURL button
	surfaceSetup := `<a2ui-json>
[
  {"createSurface": {"surfaceId": "link-surf"}},
  {"updateComponents": {"surfaceId": "link-surf", "components": [
    {"id": "root", "component": "Card", "child": "btn"},
    {
      "id": "btn",
      "component": "Button",
      "child": "lbl",
      "action": {
        "functionCall": {
          "call": "openUrl",
          "args": {"url": "https://a2ui.org/docs"}
        }
      }
    },
    {"id": "lbl", "component": "Text", "text": "Documentation"}
  ]}}
]
</a2ui-json>`

	resM, _ = m.Update(ui.NewAgentResponseMsgForTest(surfaceSetup))
	m = resM.(ui.Model)

	// Focus the surface (Pressing Up when input is empty enters the surface)
	resM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = resM.(ui.Model)

	// Press Enter on the focused button
	resM, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = resM.(ui.Model)

	if cmd != nil {
		msg := cmd()
		if ev, ok := msg.(event.ButtonClicked); ok {
			resM, cmd = m.Update(ev)
			m = resM.(ui.Model)
			if cmd != nil {
				executeCmd(cmd)
			}
		}
	}

	// 1. Verify openBrowserFunc was called with the target URL
	if len(openedURLs) != 1 || openedURLs[0] != "https://a2ui.org/docs" {
		t.Fatalf("expected openedURLs to be ['https://a2ui.org/docs'], got %v", openedURLs)
	}

	// 2. Verify no ClientMessage was sent to the server (client-side only action)
	cli.mu.Lock()
	actionCount := len(cli.actions)
	cli.mu.Unlock()
	if actionCount != 0 {
		t.Errorf("expected 0 server actions for client-side openUrl, got %d", actionCount)
	}

	// 3. Verify UI view displays feedback
	viewContent := m.View().Content
	if !strings.Contains(viewContent, "Opened URL: https://a2ui.org/docs") {
		t.Errorf("expected view to contain 'Opened URL: https://a2ui.org/docs', got:\n%s", viewContent)
	}
}

