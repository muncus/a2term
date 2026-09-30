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

package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/google/uuid"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"

	"github.com/muncus/a2term/pkg/a2ui"
	"github.com/muncus/a2term/pkg/agent"
)

func (m *Model) handleAgentResponse(msg agentResponseMsg) (tea.Model, tea.Cmd) {
	m.isLoading = false
	m.isStreaming = false
	m.authFailed = false
	m.status = "Connected"

	m.appendAgentResponseParts(msg.parts)

	wasAtBottom := m.chatViewport.AtBottom()
	m.updateViewportContent()
	if wasAtBottom {
		m.chatViewport.GotoBottom()
	}
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()
	return *m, nil
}

func (m *Model) handleStreamChunk(msg agentStreamChunkMsg) (tea.Model, tea.Cmd) {
	m.isStreaming = true
	m.status = "Streaming..."

	chunkText := msg.chunk
	if chunkText == "" && len(msg.parts) > 0 {
		var texts []string
		for _, p := range msg.parts {
			if p != nil && p.Text() != "" {
				texts = append(texts, p.Text())
			}
		}
		chunkText = strings.Join(texts, "")
	}

	if len(msg.parts) > 0 {
		for _, part := range msg.parts {
			if a2ui.IsA2UIPart(part) {
				if msgs, err := a2ui.ExtractServerMessages(part); err == nil && len(msgs) > 0 {
					m.processServerMessages(msgs)
				}
			}
		}
	}

	if chunkText != "" {
		msgs, prose := a2ui.ExtractMessagesAndText(chunkText)
		if len(msgs) > 0 {
			m.processServerMessages(msgs)
		}
		chunkText = prose
	}

	if msg.isFinal {
		m.isLoading = false
		m.isStreaming = false
		m.authFailed = false
		m.status = "Connected"

		// Remove temporary streaming item if present
		if m.streamItemID != "" {
			m.removeFeedItem(m.streamItemID)
			m.streamItemID = ""
		}

		if len(msg.parts) > 0 {
			m.appendAgentResponseParts(msg.parts)
		} else if chunkText != "" {
			m.items = append(m.items, NewAgentTextItem(uuid.NewString(), chunkText))
		}
	} else {
		// Update ongoing streaming text item
		if chunkText != "" {
			if m.streamItemID == "" {
				m.streamItemID = uuid.NewString()
				m.items = append(m.items, NewAgentTextItem(m.streamItemID, chunkText))
			} else {
				m.updateFeedItemContent(m.streamItemID, chunkText)
			}
		}
	}

	wasAtBottom := m.chatViewport.AtBottom()
	m.updateViewportContent()
	if wasAtBottom {
		m.chatViewport.GotoBottom()
	}
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()
	return *m, nil
}

func (m *Model) handleAgentError(msg agentErrorMsg) (tea.Model, tea.Cmd) {
	m.isLoading = false
	m.isStreaming = false
	if errors.Is(msg.err, agent.ErrAuthFailed) {
		m.status = "Auth Failed"
		m.authFailed = true
	} else {
		m.status = "Error"
	}

	diag := DiagnosticInfo{
		Title:     "Agent Communication Error",
		TargetURL: msg.target,
		Phase:     msg.phase,
		ErrorMsg:  msg.err.Error(),
		Tips: []string{
			"Check if the agent server crashed or closed the connection.",
			"Verify your network connection to the endpoint.",
			"Use /reset to reset task state, or /agent <url> to re-establish connection.",
		},
	}
	m.items = append(m.items, NewDiagnosticItem(uuid.NewString(), diag))
	m.updateViewportContent()
	m.chatViewport.GotoBottom()
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()
	return *m, nil
}

func (m *Model) handleReconnectSuccess(msg reconnectSuccessMsg) (tea.Model, tea.Cmd) {
	m.isLoading = false
	m.client = msg.client
	m.status = "Connected"
	m.authFailed = false
	m.items = append(m.items, NewSystemItem(uuid.NewString(), fmt.Sprintf("✅ Connected to %s (%s)", msg.agentName, msg.targetURL)))
	m.updateViewportContent()
	m.chatViewport.GotoBottom()
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()
	return *m, nil
}

func (m *Model) appendAgentResponseParts(parts []*a2a.Part) {
	if len(parts) == 0 {
		return
	}

	for _, part := range parts {
		if part == nil {
			continue
		}

		// 1. A2UI DataPart (standard transport application/a2ui+json)
		if a2ui.IsA2UIPart(part) {
			if msgs, err := a2ui.ExtractServerMessages(part); err == nil && len(msgs) > 0 {
				m.processServerMessages(msgs)
			}
			continue
		}

		// 2. Structured Data part (tool calls, data payloads): send to logs
		if d := part.Data(); d != nil {
			logContent := formatDataPartLog(d)
			m.items = append(m.items, NewToolCallItem(uuid.NewString(), logContent))
			continue
		}

		// 3. Multimodal raw data
		if r := part.Raw(); len(r) > 0 && part.Text() == "" {
			if strings.HasPrefix(part.MediaType, "image/") {
				name := part.Filename
				if name == "" {
					name = "image"
				}
				label := fmt.Sprintf("[🖼️ Image: %s (%s, %d bytes)]", name, part.MediaType, len(r))
				m.items = append(m.items, NewAgentTextItem(uuid.NewString(), label))
			} else {
				// Non-image raw data: send to logs
				label := fmt.Sprintf("Raw data: %s (%d bytes)", part.MediaType, len(r))
				m.items = append(m.items, NewToolCallItem(uuid.NewString(), label))
			}
			continue
		}

		// 4. Text part: check for embedded A2UI server messages or conversational prose
		text := part.Text()
		if text == "" {
			continue
		}

		msgs, prose := a2ui.ExtractMessagesAndText(text)
		if len(msgs) > 0 {
			m.processServerMessages(msgs)
		}
		if prose != "" {
			m.items = append(m.items, NewAgentTextItem(uuid.NewString(), prose))
		}
	}
}

func formatDataPartLog(d any) string {
	if s, ok := d.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			if b, err := json.MarshalIndent(parsed, "", "  "); err == nil {
				return string(b)
			}
		}
		return s
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", d)
}


func (m *Model) processServerMessages(msgs []tmca2ui.ServerMessage) {
	if m.dispatcher == nil {
		if m.surfaceManager == nil {
			m.surfaceManager = a2ui.NewSurfaceManager()
		}
		m.dispatcher = a2ui.NewDispatcher(m.surfaceManager, render.WithStyles(a2ui.DefaultTerminalStyles()))
	}
	results, err := m.dispatcher.DispatchBatch(msgs)
	if err != nil {
		m.items = append(m.items, NewErrorItem(uuid.NewString(), fmt.Sprintf("A2UI Dispatch Error: %v", err)))
		return
	}

	for _, res := range results {
		switch res.Type {
		case a2ui.EventSurfaceCreated:
			// Surface initialized in surface manager

		case a2ui.EventSurfaceUpdated:
			if res.Model == nil {
				continue
			}
			if m.ready {
				res.Model.SetSize(surfaceInnerWidth(m.width), m.surfacesViewport.Height())
			}

			if feedID, ok := m.surfaceFeedMap[res.SurfaceID]; ok {
				// Update existing surface item in place
				for i := range m.items {
					if m.items[i].ID == feedID {
						m.items[i].Surface = res.Model
						m.items[i].Messages = append(m.items[i].Messages, res.Message)
						break
					}
				}
			} else {
				// Append new surface item
				itemID := uuid.NewString()
				item := NewAgentSurfaceItem(itemID, res.SurfaceID, res.Model, []tmca2ui.ServerMessage{res.Message})
				m.items = append(m.items, item)
				if m.surfaceFeedMap == nil {
					m.surfaceFeedMap = make(map[string]string)
				}
				m.surfaceFeedMap[res.SurfaceID] = itemID
			}

		case a2ui.EventSurfaceDeleted:
			if feedID, ok := m.surfaceFeedMap[res.SurfaceID]; ok {
				m.removeFeedItem(feedID)
				delete(m.surfaceFeedMap, res.SurfaceID)
				if m.focusMode == FocusSurface {
					m.focusMode = FocusInput
					m.focusedSurfaceIndex = -1
				}
			}
		}
	}
}

func (m *Model) appendAgentResponseContent(content string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return
	}
	m.appendAgentResponseParts([]*a2a.Part{a2a.NewTextPart(content)})
}

func (m *Model) clearToastAfter(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return toastClearMsg{}
	}
}

func (m *Model) removeFeedItem(id string) {
	for i, it := range m.items {
		if it.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return
		}
	}
}

func (m *Model) updateFeedItemContent(id string, content string) {
	for i, it := range m.items {
		if it.ID == id {
			m.items[i].Content = content
			return
		}
	}
}
