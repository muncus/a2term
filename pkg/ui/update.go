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
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/google/uuid"
	"github.com/joestump-agent/a2tea/event"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"

	"github.com/muncus/a2term/pkg/a2ui"
	"github.com/muncus/a2term/pkg/agent"
)

// Internal message types for async A2A agent events
type (
	agentResponseMsg struct {
		parts []*a2a.Part
	}

	agentStreamChunkMsg struct {
		chunk   string
		parts   []*a2a.Part
		isFinal bool
	}

	agentErrorMsg struct {
		err    error
		action string
		target string
		phase  string
	}

	reconnectSuccessMsg struct {
		client    agent.Client
		agentName string
		targetURL string
	}

	toastClearMsg struct{}
)

// Update is the main event dispatcher for Bubble Tea.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recalculateLayout()
		m.updateViewportContent()
		m.chatViewport.GotoBottom()
		m.logsViewport.GotoBottom()
		m.syncActiveViewportMirror()
		return m, nil

	case spinner.TickMsg:
		if m.isLoading || m.isStreaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case toastClearMsg:
		m.toast = ""
		return m, nil

	// A2UI Interaction Events emitted by a2tea
	case tmca2ui.ClientMessage:
		if msg.Action != nil {
			actionName := msg.Action.Name
			srcID := msg.Action.SourceComponentID
			ctxValues := msg.Action.Context

			surfaceID := msg.Action.SurfaceID
			if surfaceID == "" && m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
				surfaceID = m.items[m.focusedSurfaceIndex].SurfaceID
			}
			if surfaceID == "" {
				surfaceID = "default"
			}

			// Validate input against check rules before action dispatch
			if m.surfaceManager != nil {
				var inputVal any
				if ctxValues != nil {
					inputVal = ctxValues[srcID]
				}
				if valErr := m.surfaceManager.ValidateInput(surfaceID, srcID, inputVal); valErr != nil {
					m.toast = fmt.Sprintf("❌ %s", valErr.Message)
					m.items = append(m.items, NewErrorItem(uuid.NewString(), fmt.Sprintf("Validation Error (%s): %s", srcID, valErr.Message)))
					m.updateViewportContent()
					m.viewport.GotoBottom()
					return m, m.clearToastAfter(3 * time.Second)
				}
			}

			var clientDataModel map[string]any
			if m.surfaceManager != nil {
				clientDataModel = m.surfaceManager.ExportClientDataModel(surfaceID)
			}

			summary := fmt.Sprintf("Action: %s (source: %s, values: %v)", actionName, srcID, ctxValues)
			return (&m).handleUIAction(fmt.Sprintf("⚡ %s", summary),
				m.sendActionCmd(actionName, surfaceID, srcID, ctxValues, clientDataModel))
		}
		return m, nil

	case event.ButtonClicked:
		// Client-side actions (such as openUrl function calls) are handled here,
		// but responses to the model/server use ClientMessage to prevent duplicate dispatch.
		if m.surfaceManager == nil {
			return m, nil
		}

		surfaceID := msg.Source.SurfaceID
		if surfaceID == "" && m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
			surfaceID = m.items[m.focusedSurfaceIndex].SurfaceID
		}
		if surfaceID == "" {
			surfaceID = "default"
		}

		st, ok := m.surfaceManager.GetSurface(surfaceID)
		if !ok {
			return m, nil
		}

		comp, ok := st.Components[msg.ID]
		if !ok || comp.Button == nil || comp.Button.Action.FunctionCall == nil {
			return m, nil
		}

		fn := comp.Button.Action.FunctionCall
		if strings.EqualFold(fn.Call, "openUrl") {
			rawURL, _ := fn.Args["url"]
			resolved := a2ui.ResolveValue(rawURL, st.DataStore, nil)
			targetURL := fmt.Sprintf("%v", resolved)
			if targetURL != "" && targetURL != "<nil>" {
				summary := fmt.Sprintf("Opened URL: %s", targetURL)
				openCmd := func() tea.Msg {
					_ = openBrowserFunc(targetURL)
					return nil
				}
				return m.handleLocalUIAction(fmt.Sprintf("🔗 %s", summary), openCmd)
			}
		}
		return m, nil

	case event.InputSubmitted:
		summary := fmt.Sprintf("Submitted text for %s: %q", msg.Source.ComponentID, msg.Value)
		return (&m).handleLocalUIAction(summary, nil)

	case event.ChoiceSelected:
		summary := fmt.Sprintf("Choice selected for %s: %v", msg.Source.ComponentID, msg.Values)
		return (&m).handleLocalUIAction(summary, nil)

	// Mouse events for click-to-focus and scrolling
	case tea.MouseClickMsg:
		return (&m).handleMouseClick(msg)

	case tea.MouseWheelMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg:
		var cmd tea.Cmd
		switch m.activeTab {
		case TabSurfaces:
			m.surfacesViewport, cmd = m.surfacesViewport.Update(msg)
		case TabLogs:
			m.logsViewport, cmd = m.logsViewport.Update(msg)
		default:
			m.chatViewport, cmd = m.chatViewport.Update(msg)
		}
		m.syncActiveViewportMirror()
		return m, cmd

	// A2A Agent communication messages
	case agentResponseMsg:
		return (&m).handleAgentResponse(msg)

	case agentStreamChunkMsg:
		return (&m).handleStreamChunk(msg)

	case agentErrorMsg:
		return (&m).handleAgentError(msg)

	case reconnectSuccessMsg:
		return (&m).handleReconnectSuccess(msg)

	// Key Presses
	case tea.KeyPressMsg:
		return (&m).handleKeyPress(msg)
	}

	// Forward other messages to active component
	if m.focusMode == FocusSurface && m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
		if surf := m.items[m.focusedSurfaceIndex].Surface; surf != nil {
			var cmd tea.Cmd
			var updated tea.Model
			updated, cmd = surf.Update(msg)
			if rm, ok := updated.(render.Model); ok {
				m.items[m.focusedSurfaceIndex].Surface = rm
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	} else if m.focusMode == FocusInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	headerHeight := lipgloss.Height(m.renderHeader())
	tabBarHeight := lipgloss.Height(m.renderTabBar())
	topOffset := headerHeight + tabBarHeight

	// Only respond to left clicks for focus switching
	if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone || msg.Button == 0 {
		// 1. Click on Tab Bar line (headerHeight is the tab buttons line)
		if msg.Y >= headerHeight && msg.Y < headerHeight+1 {
			if tab, ok := m.tabAtX(msg.X); ok {
				return *m, m.SetActiveTab(tab)
			}
		}

		vpHeight := m.activeViewport().Height()

		// 2. Click below the viewport (Input box, Toast, or Footer)
		if msg.Y >= topOffset+vpHeight {
			var cmd tea.Cmd
			if m.activeTab != TabChat {
				cmd = m.SetActiveTab(TabChat)
			} else {
				cmd = m.ReturnFocusToInput()
				m.updateViewportContent()
			}
			return *m, cmd
		}

		// 3. Click inside viewport
		if msg.Y >= topOffset && msg.Y < topOffset+vpHeight {
			vpRow := msg.Y - topOffset

			if m.activeTab == TabChat {
				contentLine := m.chatViewport.YOffset() + vpRow
				itemIdx := m.findItemAtContentLine(contentLine)
				if itemIdx >= 0 && itemIdx < len(m.items) && m.items[itemIdx].Kind == KindAgentSurface && m.items[itemIdx].Surface != nil {
					cmd1 := m.SetActiveTab(TabSurfaces)
					cmd2 := m.setFocusedSurface(itemIdx)
					return *m, tea.Batch(cmd1, cmd2)
				}
			} else if m.activeTab == TabSurfaces {
				contentLine := m.surfacesViewport.YOffset() + vpRow
				surfIdx := m.findSurfaceAtContentLine(contentLine)
				if surfIdx >= 0 && surfIdx < len(m.items) && m.items[surfIdx].Kind == KindAgentSurface && m.items[surfIdx].Surface != nil {
					cmd := m.setFocusedSurface(surfIdx)
					m.updateViewportContent()
					return *m, cmd
				}
			}
		}
	}

	// Also forward mouse click to active viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}
