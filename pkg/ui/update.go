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
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/joestump-agent/a2tea/event"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"

	"github.com/a2aproject/a2a-go/v2/a2a"
	a2aclient "github.com/muncus/a2term/pkg/a2a"
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

		headerHeight := lipgloss.Height(m.renderHeader()) + lipgloss.Height(m.renderTabBar())
		footerHeight := 2
		inputHeight := 3

		chatVpHeight := msg.Height - headerHeight - footerHeight - inputHeight
		if chatVpHeight < 4 {
			chatVpHeight = 4
		}

		fullVpHeight := msg.Height - headerHeight - footerHeight
		if fullVpHeight < 4 {
			fullVpHeight = 4
		}

		m.chatViewport.SetWidth(msg.Width)
		m.chatViewport.SetHeight(chatVpHeight)

		m.surfacesViewport.SetWidth(msg.Width)
		m.surfacesViewport.SetHeight(fullVpHeight)

		m.logsViewport.SetWidth(msg.Width)
		m.logsViewport.SetHeight(fullVpHeight)

		m.viewport.SetWidth(msg.Width)
		if m.activeTab == TabChat {
			m.viewport.SetHeight(chatVpHeight)
		} else {
			m.viewport.SetHeight(fullVpHeight)
		}

		m.input.SetWidth(msg.Width - 6)

		// Update sizes of all active surfaces
		surfWidth := surfaceInnerWidth(msg.Width)
		for _, item := range m.items {
			if item.Kind == KindAgentSurface && item.Surface != nil {
				item.Surface.SetSize(surfWidth, fullVpHeight)
			}
		}

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
				return m.handleUIAction(fmt.Sprintf("🔗 %s", summary), openCmd)
			}
		}
		return m, nil

	case event.InputSubmitted:
		summary := fmt.Sprintf("Submitted text for %s: %q", msg.Source.ComponentID, msg.Value)
		return (&m).handleUIAction(summary, nil)

	case event.ChoiceSelected:
		summary := fmt.Sprintf("Choice selected for %s: %v", msg.Source.ComponentID, msg.Values)
		return (&m).handleUIAction(summary, nil)

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
		m.syncActiveViewportMirror()
		return m, nil

	case agentStreamChunkMsg:
		m.isStreaming = true
		m.status = "Streaming..."

		chunkText := msg.chunk
		if chunkText == "" && len(msg.parts) > 0 {
			chunkText = a2aclient.ExtractPartsText(msg.parts)
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
		m.syncActiveViewportMirror()
		return m, nil

	case agentErrorMsg:
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
		return m, nil

	case reconnectSuccessMsg:
		m.isLoading = false
		m.client = msg.client
		m.status = "Connected"
		m.authFailed = false
		m.items = append(m.items, NewSystemItem(uuid.NewString(), fmt.Sprintf("✅ Connected to %s (%s)", msg.agentName, msg.targetURL)))
		m.updateViewportContent()
		m.chatViewport.GotoBottom()
		m.logsViewport.GotoBottom()
		m.syncActiveViewportMirror()
		return m, nil

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

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	keyStr := msg.String()

	// Global shortcuts
	if keyStr == "ctrl+c" {
		return *m, tea.Quit
	}

	// 1. Tab switching shortcuts
	switch keyStr {
	case "f1", "alt+1":
		return *m, m.SetActiveTab(TabChat)
	case "f2", "alt+2":
		return *m, m.SetActiveTab(TabSurfaces)
	case "f3", "alt+3":
		return *m, m.SetActiveTab(TabLogs)
	case "shift+left":
		return *m, m.PrevTab()
	case "shift+right":
		return *m, m.NextTab()
	}

	// 2. Global scrollback navigation shortcuts (scrolls the currently active tab's viewport)
	vp := m.activeViewport()
	switch keyStr {
	case "pgup", "pageup":
		vp.HalfPageUp()
		m.syncActiveViewportMirror()
		return *m, nil
	case "pgdown", "pagedown":
		vp.HalfPageDown()
		m.syncActiveViewportMirror()
		return *m, nil
	case "ctrl+u":
		vp.HalfPageUp()
		m.syncActiveViewportMirror()
		return *m, nil
	case "ctrl+d":
		vp.HalfPageDown()
		m.syncActiveViewportMirror()
		return *m, nil
	case "shift+up":
		vp.ScrollUp(3)
		m.syncActiveViewportMirror()
		return *m, nil
	case "shift+down":
		vp.ScrollDown(3)
		m.syncActiveViewportMirror()
		return *m, nil
	case "ctrl+home":
		vp.GotoTop()
		m.syncActiveViewportMirror()
		return *m, nil
	case "ctrl+end":
		vp.GotoBottom()
		m.syncActiveViewportMirror()
		return *m, nil
	case "alt+up", "alt+k":
		vp.ScrollUp(1)
		m.syncActiveViewportMirror()
		return *m, nil
	case "alt+down", "alt+j":
		vp.ScrollDown(1)
		m.syncActiveViewportMirror()
		return *m, nil
	}

	// 3. Tab-specific key handling

	// --- TAB: SURFACES ---
	if m.activeTab == TabSurfaces {
		if keyStr == "esc" {
			// Forward Esc to surface (to close modal if open)
			if m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
				surf := m.items[m.focusedSurfaceIndex].Surface
				if surf != nil {
					updated, cmd := surf.Update(msg)
					if rm, ok := updated.(render.Model); ok {
						m.items[m.focusedSurfaceIndex].Surface = rm
					}
					if cmd == nil {
						cmd = m.SetActiveTab(TabChat)
					}
					m.updateViewportContent()
					return *m, cmd
				}
			}
			return *m, m.SetActiveTab(TabChat)
		}

		if keyStr == "home" {
			m.surfacesViewport.GotoTop()
			m.syncActiveViewportMirror()
			return *m, nil
		}
		if keyStr == "end" {
			m.surfacesViewport.GotoBottom()
			m.syncActiveViewportMirror()
			return *m, nil
		}

		if keyStr == "ctrl+f" {
			cmd := m.FocusNextSurface()
			m.updateViewportContent()
			return *m, cmd
		}

		if m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
			surf := m.items[m.focusedSurfaceIndex].Surface
			if surf != nil {
				if keyStr == "tab" || keyStr == "shift+tab" {
					updated, cmd := surf.Update(msg)
					if rm, ok := updated.(render.Model); ok {
						m.items[m.focusedSurfaceIndex].Surface = rm
					}
					m.updateViewportContent()
					return *m, cmd
				}

				updated, cmd := m.updateSurfaceWithArrowNav(surf, msg)
				if rm, ok := updated.(render.Model); ok {
					m.items[m.focusedSurfaceIndex].Surface = rm
				}
				m.updateViewportContent()
				return *m, cmd
			}
		}
		return *m, nil
	}

	// --- TAB: LOGS ---
	if m.activeTab == TabLogs {
		if keyStr == "esc" {
			return *m, m.SetActiveTab(TabChat)
		}
		if keyStr == "home" {
			m.logsViewport.GotoTop()
			m.syncActiveViewportMirror()
			return *m, nil
		}
		if keyStr == "end" {
			m.logsViewport.GotoBottom()
			m.syncActiveViewportMirror()
			return *m, nil
		}
		if keyStr == "up" {
			m.logsViewport.ScrollUp(1)
			m.syncActiveViewportMirror()
			return *m, nil
		}
		if keyStr == "down" {
			m.logsViewport.ScrollDown(1)
			m.syncActiveViewportMirror()
			return *m, nil
		}
		return *m, nil
	}

	// --- TAB: CHAT ---
	if m.activeTab == TabChat {
		if keyStr == "tab" || keyStr == "ctrl+f" {
			surfaces := m.FindSurfaceIndices()
			if len(surfaces) > 0 {
				return *m, m.SetActiveTab(TabSurfaces)
			}
		}

		if keyStr == "esc" {
			if m.input.Value() != "" {
				m.input.SetValue("")
				return *m, nil
			}
		}

		switch keyStr {
		case "up", "ctrl+up":
			if m.input.Value() == "" || keyStr == "ctrl+up" {
				surfaces := m.FindSurfaceIndices()
				if len(surfaces) > 0 {
					return *m, m.SetActiveTab(TabSurfaces)
				}
				m.chatViewport.ScrollUp(2)
				m.syncActiveViewportMirror()
				return *m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return *m, cmd

		case "down":
			if m.input.Value() == "" {
				m.chatViewport.ScrollDown(2)
				m.syncActiveViewportMirror()
				return *m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return *m, cmd

		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return *m, nil
			}

			m.input.SetValue("")

			// Check slash commands
			if strings.HasPrefix(val, "/") {
				return m.handleCommand(val)
			}

			// Add user message
			m.items = append(m.items, NewUserItem(uuid.NewString(), val))
			m.updateViewportContent()
			m.chatViewport.GotoBottom()
			m.syncActiveViewportMirror()

			if m.client == nil {
				diag := DiagnosticInfo{
					Title:     "No Agent Connected",
					TargetURL: m.agentURL,
					Phase:     "Pre-flight Check",
					ErrorMsg:  "Cannot send message because no A2A agent client is connected.",
					Tips: []string{
						"Use '/agent <url>' to connect to an agent endpoint (e.g. /agent http://localhost:9001).",
						"Use '/card <url>' to connect via Agent Card JSON URL.",
						"Use '/help' for more information.",
					},
				}
				m.items = append(m.items, NewDiagnosticItem(uuid.NewString(), diag))
				m.updateViewportContent()
				m.chatViewport.GotoBottom()
				m.logsViewport.GotoBottom()
				m.syncActiveViewportMirror()
				return *m, nil
			}

			m.isLoading = true
			m.status = "Thinking..."

			cmds = append(cmds, m.sendMessageCmd(val))
			cmds = append(cmds, m.spinner.Tick)
			return *m, tea.Batch(cmds...)

		case "home":
			if m.input.Value() == "" {
				m.chatViewport.GotoTop()
				m.syncActiveViewportMirror()
				return *m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return *m, cmd

		case "end":
			if m.input.Value() == "" {
				m.chatViewport.GotoBottom()
				m.syncActiveViewportMirror()
				return *m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return *m, cmd

		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return *m, cmd
		}
	}

	return *m, tea.Batch(cmds...)
}

func (m *Model) handleCommand(cmdStr string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmdStr)
	cmd := parts[0]

	switch cmd {
	case "/help":
		var sb strings.Builder
		sb.WriteString("📖 github.com/muncus/a2term Commands & Help:\n")
		sb.WriteString("  /help            - Show this help message\n")
		sb.WriteString("  /tab <name|num>  - Switch tab: chat (1), surfaces (2), logs (3)\n")
		sb.WriteString("  /clear           - Clear conversation history\n")
		sb.WriteString("  /reset           - Reset active A2A session & task context\n")
		sb.WriteString("  /agent <url>     - Connect to agent endpoint URL\n")
		sb.WriteString("  /card <url>      - Resolve and connect using Agent Card URL\n")
		sb.WriteString("  /auth <token>    - Set or update bearer authorization token\n")
		sb.WriteString("  /quit, /exit     - Exit github.com/muncus/a2term\n\n")
		sb.WriteString("⌨️ Keybindings:\n")
		sb.WriteString("  [F1] / [Alt+1]   - Switch to Chat Feed tab\n")
		sb.WriteString("  [F2] / [Alt+2]   - Switch to A2UI Surfaces tab\n")
		sb.WriteString("  [F3] / [Alt+3]   - Switch to Logs & Diagnostics tab\n")
		sb.WriteString("  [Shift+← / →]    - Cycle between tabs\n")
		sb.WriteString("  [Tab]            - Navigate controls inside A2UI surface\n")
		sb.WriteString("  [Esc]            - Close modal / return to chat tab\n")
		sb.WriteString("  [Enter]          - Send message / activate focused button\n")
		sb.WriteString("  [PgUp / PgDn]    - Scroll viewport half page up / down\n")
		sb.WriteString("  [Ctrl+U / Ctrl+D]- Scroll viewport half page up / down\n")
		sb.WriteString("  [Shift+Up / Down]- Scroll viewport 3 lines up / down\n")
		sb.WriteString("  [Ctrl+Home / End]- Jump to top / bottom of scrollback\n")
		sb.WriteString("  [Mouse Wheel]    - Scroll viewport smoothly\n")
		sb.WriteString("  [Ctrl+C]         - Quit")
		m.items = append(m.items, NewSystemItem(uuid.NewString(), sb.String()))

	case "/tab":
		if len(parts) < 2 {
			m.items = append(m.items, NewSystemItem(uuid.NewString(), "Usage: /tab <1|2|3|chat|surfaces|logs>"))
			m.updateViewportContent()
			return *m, nil
		}
		target := strings.ToLower(parts[1])
		switch target {
		case "1", "chat":
			return *m, m.SetActiveTab(TabChat)
		case "2", "surfaces", "surface", "a2ui":
			return *m, m.SetActiveTab(TabSurfaces)
		case "3", "logs", "log":
			return *m, m.SetActiveTab(TabLogs)
		default:
			m.items = append(m.items, NewErrorItem(uuid.NewString(), fmt.Sprintf("Unknown tab: %q. Use chat, surfaces, or logs.", parts[1])))
			m.updateViewportContent()
			return *m, nil
		}

	case "/clear":
		m.items = make([]FeedItem, 0)
		m.focusedSurfaceIndex = -1
		m.focusMode = FocusInput
		m.surfaceFeedMap = make(map[string]string)
		m.items = append(m.items, NewSystemItem(uuid.NewString(), "Conversation history cleared."))
		m.updateViewportContent()
		m.chatViewport.GotoBottom()
		m.surfacesViewport.GotoTop()
		m.logsViewport.GotoTop()
		m.syncActiveViewportMirror()
		return *m, nil

	case "/reset":
		m.surfaceFeedMap = make(map[string]string)
		m.surfaceManager = a2ui.NewSurfaceManager()
		m.dispatcher = a2ui.NewDispatcher(m.surfaceManager, render.WithStyles(a2ui.DefaultTerminalStyles()))
		if m.client != nil {
			m.client.ResetSession()
			m.items = append(m.items, NewSystemItem(uuid.NewString(), "Session task and context reset."))
		} else {
			m.items = append(m.items, NewSystemItem(uuid.NewString(), "Session reset (no active client)."))
		}

	case "/agent":
		if len(parts) < 2 {
			m.items = append(m.items, NewErrorItem(uuid.NewString(), "Usage: /agent <url>"))
		} else {
			url := parts[1]
			m.agentURL = url
			m.cardURL = ""
			m.isLoading = true
			m.status = "Connecting..."
			m.updateViewportContent()
			return *m, m.reconnectCmd(a2aclient.ClientOptions{AgentURL: url, AuthToken: m.authToken}, url)
		}

	case "/card":
		if len(parts) < 2 {
			m.items = append(m.items, NewErrorItem(uuid.NewString(), "Usage: /card <url>"))
		} else {
			url := parts[1]
			m.cardURL = url
			m.agentURL = ""
			m.isLoading = true
			m.status = "Resolving card..."
			m.updateViewportContent()
			return *m, m.reconnectCmd(a2aclient.ClientOptions{CardURL: url, AuthToken: m.authToken}, url)
		}

	case "/auth":
		if len(parts) < 2 {
			if m.authToken != "" {
				m.items = append(m.items, NewSystemItem(uuid.NewString(), "Bearer token is currently configured."))
			} else {
				m.items = append(m.items, NewSystemItem(uuid.NewString(), "No bearer token configured. Usage: /auth <token>"))
			}
		} else {
			token := strings.TrimSpace(parts[1])
			if token == "none" || token == "clear" || token == `""` {
				m.authToken = ""
				m.items = append(m.items, NewSystemItem(uuid.NewString(), "Bearer authorization token cleared."))
			} else {
				m.authToken = token
				m.items = append(m.items, NewSystemItem(uuid.NewString(), "Bearer authorization token updated."))
			}
			target := m.agentURL
			opts := a2aclient.ClientOptions{AgentURL: target, AuthToken: m.authToken}
			if target == "" {
				target = m.cardURL
				opts = a2aclient.ClientOptions{CardURL: target, AuthToken: m.authToken}
			}
			if target != "" {
				m.isLoading = true
				m.status = "Reconnecting with updated auth..."
				m.updateViewportContent()
				return *m, m.reconnectCmd(opts, target)
			}
		}

	case "/quit", "/exit":
		return *m, tea.Quit

	default:
		m.items = append(m.items, NewErrorItem(uuid.NewString(), fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd)))
	}

	m.updateViewportContent()
	m.chatViewport.GotoBottom()
	m.syncActiveViewportMirror()
	return *m, nil
}

// Commands for async operations

func (m *Model) sendMessageCmd(text string) tea.Cmd {
	client := m.client
	target := m.agentURL
	if target == "" {
		target = m.cardURL
	}
	return func() tea.Msg {
		if client == nil {
			return agentErrorMsg{
				err:    fmt.Errorf("agent client not initialized"),
				target: target,
				phase:  "Pre-flight",
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		parts, err := client.SendMessage(ctx, text)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: target,
				phase:  "SendMessage Execution",
			}
		}
		return agentResponseMsg{parts: parts}
	}
}

func (m *Model) sendActionCmd(actionName, surfaceID, sourceID string, contextValues map[string]any, clientDataModel map[string]any) tea.Cmd {
	client := m.client
	target := m.agentURL
	if target == "" {
		target = m.cardURL
	}
	return func() tea.Msg {
		if client == nil {
			return agentErrorMsg{
				err:    fmt.Errorf("agent client not initialized"),
				target: target,
				phase:  "Pre-flight",
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		parts, err := client.SendA2UIAction(ctx, actionName, surfaceID, sourceID, contextValues, clientDataModel)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: target,
				phase:  fmt.Sprintf("SendA2UIAction (%s)", actionName),
			}
		}
		return agentResponseMsg{parts: parts}
	}
}

func (m *Model) reconnectCmd(opts a2aclient.ClientOptions, targetURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cli, err := a2aclient.NewClient(ctx, opts)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: targetURL,
				phase:  "Connection Establishment & Card Resolution",
			}
		}
		return reconnectSuccessMsg{
			client:    cli,
			agentName: cli.AgentName(),
			targetURL: targetURL,
		}
	}
}

func (m *Model) handleUIAction(summary string, sendCmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.items = append(m.items, NewActionItem(uuid.NewString(), summary))
	m.toast = summary
	m.updateViewportContent()
	m.chatViewport.GotoBottom()
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()

	var cmds []tea.Cmd
	cmds = append(cmds, m.clearToastAfter(3*time.Second))

	if sendCmd != nil {
		if m.client != nil {
			m.isLoading = true
			m.status = "Sending action..."
			cmds = append(cmds, sendCmd, m.spinner.Tick)
		} else {
			m.items = append(m.items, NewErrorItem(uuid.NewString(), "Cannot send action: No A2A agent connected. Use /agent <url> or /card <url> to connect."))
			m.updateViewportContent()
			m.chatViewport.GotoBottom()
			m.logsViewport.GotoBottom()
			m.syncActiveViewportMirror()
		}
	}
	return *m, tea.Batch(cmds...)
}

func (m *Model) appendAgentResponseParts(parts []*a2a.Part) {
	if len(parts) == 0 {
		return
	}

	segments, err := a2ui.ParseAgentParts(parts, render.WithStyles(a2ui.DefaultTerminalStyles()))
	if err != nil || len(segments) == 0 {
		text := a2aclient.ExtractPartsText(parts)
		if strings.TrimSpace(text) != "" {
			m.items = append(m.items, NewAgentTextItem(uuid.NewString(), text))
		}
		return
	}

	for _, seg := range segments {
		switch seg.Type {
		case a2ui.TypeSurface:
			if m.dispatcher != nil && len(seg.Messages) > 0 {
				m.processServerMessages(seg.Messages)
			} else {
				if m.ready && seg.Surface != nil {
					seg.Surface.SetSize(surfaceInnerWidth(m.width), m.surfacesViewport.Height())
				}
				surfID := seg.SurfaceID
				if surfID == "" {
					surfID = "surface"
				}
				m.items = append(m.items, NewAgentSurfaceItem(uuid.NewString(), surfID, seg.Surface, seg.Messages))
			}
		case a2ui.TypeText:
			msgs, prose := a2ui.ExtractMessagesAndText(seg.Text)
			if len(msgs) > 0 {
				m.processServerMessages(msgs)
			}
			if prose != "" {
				m.items = append(m.items, NewAgentTextItem(uuid.NewString(), prose))
			} else if len(msgs) == 0 {
				m.items = append(m.items, NewAgentTextItem(uuid.NewString(), seg.Text))
			}
		}
	}
}

func (m *Model) processServerMessages(msgs []tmca2ui.ServerMessage) {
	if m.dispatcher == nil {
		return
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

// updateSurfaceWithArrowNav updates the focused A2UI surface with arrow key navigation support.
// If the surface or its active control (e.g. ChoicePicker cursor, Slider adjustment, Tab switch)
// does not consume the arrow key, it advances or retreats focus between components.
func (m Model) updateSurfaceWithArrowNav(surf render.Model, msg tea.KeyPressMsg) (render.Model, tea.Cmd) {
	keyStr := msg.String()
	beforeView := surf.View().Content
	updated, cmd := surf.Update(msg)
	resSurf := surf
	if rm, ok := updated.(render.Model); ok {
		resSurf = rm
	}
	afterView := resSurf.View().Content

	if beforeView == afterView {
		switch keyStr {
		case "down", "right":
			// Forward Tab to navigate to next control in surface
			tabMsg := tea.KeyPressMsg{Code: tea.KeyTab}
			u2, c2 := resSurf.Update(tabMsg)
			if rm, ok := u2.(render.Model); ok {
				resSurf = rm
			}
			if c2 != nil {
				cmd = tea.Batch(cmd, c2)
			}
		case "up", "left":
			// Forward Shift+Tab to navigate to previous control in surface
			shiftTabMsg := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
			u2, c2 := resSurf.Update(shiftTabMsg)
			if rm, ok := u2.(render.Model); ok {
				resSurf = rm
			}
			if c2 != nil {
				cmd = tea.Batch(cmd, c2)
			}
		}
	}

	return resSurf, cmd
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

var openBrowserFunc = openBrowser

func openBrowser(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}
