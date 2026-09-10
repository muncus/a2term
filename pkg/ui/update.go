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
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/joestump-agent/a2tea/event"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"

	"github.com/muncus/a2term/pkg/a2a"
	"github.com/muncus/a2term/pkg/a2ui"
	"github.com/muncus/a2term/pkg/agent"
)

// Internal message types for async A2A agent events
type (
	agentResponseMsg struct {
		text string
	}

	agentStreamChunkMsg struct {
		chunk   string
		isFinal bool
	}

	agentErrorMsg struct {
		err      error
		action   string
		target   string
		phase    string
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

		headerHeight := 2
		footerHeight := 2
		inputHeight := 3
		vpHeight := msg.Height - headerHeight - footerHeight - inputHeight
		if vpHeight < 4 {
			vpHeight = 4
		}

		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(vpHeight)
		m.input.SetWidth(msg.Width - 6)

		// Update sizes of all active surfaces
		surfWidth := surfaceInnerWidth(msg.Width)
		for _, item := range m.items {
			if item.Kind == KindAgentSurface && item.Surface != nil {
				item.Surface.SetSize(surfWidth, vpHeight)
			}
		}

		m.updateViewportContent()
		m.viewport.GotoBottom()
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
			summary := fmt.Sprintf("Action: %s (source: %s, values: %v)", actionName, srcID, ctxValues)
			return m.handleUIAction(fmt.Sprintf("⚡ %s", summary), m.sendActionCmd(actionName, srcID, ctxValues))
		}
		return m, nil

	case event.ButtonClicked:
		// a2tea emits a native tmca2ui.ClientMessage alongside ButtonClicked with full form context.
		// Action dispatch is handled in tmca2ui.ClientMessage to prevent duplicate dispatch.
		return m, nil

	case event.InputSubmitted:
		summary := fmt.Sprintf("Submitted text for %s: %q", msg.Source.ComponentID, msg.Value)
		return m.handleUIAction(summary, nil)

	case event.ChoiceSelected:
		summary := fmt.Sprintf("Choice selected for %s: %v", msg.Source.ComponentID, msg.Values)
		return m.handleUIAction(summary, nil)

	// Mouse events for click-to-focus and scrolling
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)

	case tea.MouseWheelMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	// A2A Agent communication messages
	case agentResponseMsg:
		m.isLoading = false
		m.isStreaming = false
		m.status = "Connected"

		m.appendAgentResponseContent(msg.text)

		wasAtBottom := m.viewport.AtBottom()
		m.updateViewportContent()
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
		return m, nil

	case agentStreamChunkMsg:
		m.isStreaming = true
		m.status = "Streaming..."

		if msg.isFinal {
			m.isLoading = false
			m.isStreaming = false
			m.status = "Connected"

			// Remove temporary streaming item if present
			if m.streamItemID != "" {
				m.removeFeedItem(m.streamItemID)
				m.streamItemID = ""
			}

			m.appendAgentResponseContent(msg.chunk)
		} else {
			// Update ongoing streaming text item
			if m.streamItemID == "" {
				m.streamItemID = uuid.NewString()
				m.items = append(m.items, NewAgentTextItem(m.streamItemID, msg.chunk))
			} else {
				m.updateFeedItemContent(m.streamItemID, msg.chunk)
			}
		}

		wasAtBottom := m.viewport.AtBottom()
		m.updateViewportContent()
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
		return m, nil

	case agentErrorMsg:
		m.isLoading = false
		m.isStreaming = false
		m.status = "Error"

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
		m.viewport.GotoBottom()
		return m, nil

	case reconnectSuccessMsg:
		m.isLoading = false
		m.client = msg.client
		m.status = "Connected"
		m.items = append(m.items, NewSystemItem(uuid.NewString(), fmt.Sprintf("✅ Connected to %s (%s)", msg.agentName, msg.targetURL)))
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil

	// Key Presses
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
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

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	keyStr := msg.String()

	// Global shortcuts
	if keyStr == "ctrl+c" {
		return m, tea.Quit
	}

	// Global scrollback navigation shortcuts
	switch keyStr {
	case "pgup", "pageup":
		m.viewport.HalfPageUp()
		return m, nil
	case "pgdown", "pagedown":
		m.viewport.HalfPageDown()
		return m, nil
	case "ctrl+u":
		m.viewport.HalfPageUp()
		return m, nil
	case "ctrl+d":
		m.viewport.HalfPageDown()
		return m, nil
	case "shift+up":
		m.viewport.ScrollUp(3)
		return m, nil
	case "shift+down":
		m.viewport.ScrollDown(3)
		return m, nil
	case "ctrl+home":
		m.viewport.GotoTop()
		return m, nil
	case "ctrl+end":
		m.viewport.GotoBottom()
		return m, nil
	case "alt+up", "alt+k":
		m.viewport.ScrollUp(1)
		return m, nil
	case "alt+down", "alt+j":
		m.viewport.ScrollDown(1)
		return m, nil
	}

	// Focus switching with Tab
	if keyStr == "tab" {
		surfaces := m.FindSurfaceIndices()
		if len(surfaces) > 0 {
			if m.focusMode == FocusInput {
				cmd := m.FocusNextSurface()
				m.updateViewportContent()
				return m, cmd
			}
			// If already on surface, forward tab to cycle inside surface first
			if m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
				surf := m.items[m.focusedSurfaceIndex].Surface
				if surf != nil {
					updated, cmd := surf.Update(msg)
					if rm, ok := updated.(render.Model); ok {
						m.items[m.focusedSurfaceIndex].Surface = rm
					}
					m.updateViewportContent()
					return m, cmd
				}
			}
		}
	}

	// Shift+Tab or Ctrl+F switches focus between Input and Surface
	if keyStr == "shift+tab" || keyStr == "ctrl+f" {
		if m.focusMode == FocusSurface {
			cmd := m.ReturnFocusToInput()
			m.updateViewportContent()
			return m, cmd
		} else {
			cmd := m.FocusNextSurface()
			m.updateViewportContent()
			return m, cmd
		}
	}

	// Escape handling
	if keyStr == "esc" {
		if m.focusMode == FocusSurface {
			// Forward Esc to surface (to close modal if open)
			if m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
				surf := m.items[m.focusedSurfaceIndex].Surface
				if surf != nil {
					updated, cmd := surf.Update(msg)
					if rm, ok := updated.(render.Model); ok {
						m.items[m.focusedSurfaceIndex].Surface = rm
					}
					// If no modal was open, return focus to input
					if cmd == nil {
						cmd = m.ReturnFocusToInput()
					}
					m.updateViewportContent()
					return m, cmd
				}
			}
			cmd := m.ReturnFocusToInput()
			m.updateViewportContent()
			return m, cmd
		}
	}

	// When an A2UI Surface has focus
	if m.focusMode == FocusSurface && m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) {
		if keyStr == "home" {
			m.viewport.GotoTop()
			return m, nil
		}
		if keyStr == "end" {
			m.viewport.GotoBottom()
			return m, nil
		}

		surf := m.items[m.focusedSurfaceIndex].Surface
		if surf != nil {
			updated, cmd := m.updateSurfaceWithArrowNav(surf, msg)
			if rm, ok := updated.(render.Model); ok {
				m.items[m.focusedSurfaceIndex].Surface = rm
			}
			m.updateViewportContent()
			return m, cmd
		}
	}

	// When Chat Input has focus
	if m.focusMode == FocusInput {
		switch keyStr {
		case "up", "ctrl+up":
			// When chat input is empty or ctrl+up is pressed, enter the most recent active A2UI surface
			if m.input.Value() == "" || keyStr == "ctrl+up" {
				surfaces := m.FindSurfaceIndices()
				if len(surfaces) > 0 {
					cmd := m.FocusNextSurface()
					m.updateViewportContent()
					return m, cmd
				}
				m.viewport.ScrollUp(2)
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		case "down":
			if m.input.Value() == "" {
				m.viewport.ScrollDown(2)
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}

			m.input.SetValue("")

			// Check slash commands
			if strings.HasPrefix(val, "/") {
				return m.handleCommand(val)
			}

			// Add user message
			m.items = append(m.items, NewUserItem(uuid.NewString(), val))
			m.updateViewportContent()
			m.viewport.GotoBottom()

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
				m.viewport.GotoBottom()
				return m, nil
			}

			m.isLoading = true
			m.status = "Thinking..."

			cmds = append(cmds, m.sendMessageCmd(val))
			cmds = append(cmds, m.spinner.Tick)
			return m, tea.Batch(cmds...)

		case "home":
			if m.input.Value() == "" {
				m.viewport.GotoTop()
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		case "end":
			if m.input.Value() == "" {
				m.viewport.GotoBottom()
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleCommand(cmdStr string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmdStr)
	cmd := parts[0]

	switch cmd {
	case "/help":
		var sb strings.Builder
		sb.WriteString("📖 github.com/muncus/a2term Commands & Help:\n")
		sb.WriteString("  /help            - Show this help message\n")
		sb.WriteString("  /clear           - Clear conversation history\n")
		sb.WriteString("  /reset           - Reset active A2A session & task context\n")
		sb.WriteString("  /agent <url>     - Connect to agent endpoint URL\n")
		sb.WriteString("  /card <url>      - Resolve and connect using Agent Card URL\n")
		sb.WriteString("  /quit, /exit     - Exit github.com/muncus/a2term\n\n")
		sb.WriteString("⌨️ Keybindings:\n")
		sb.WriteString("  [Tab]            - Focus interactive A2UI surface / cycle controls\n")
		sb.WriteString("  [Shift+Tab]      - Return focus to chat input\n")
		sb.WriteString("  [Ctrl+F]         - Toggle focus between input and surface\n")
		sb.WriteString("  [Esc]            - Close modal / return focus to chat input\n")
		sb.WriteString("  [Enter]          - Send message / activate focused button\n")
		sb.WriteString("  [PgUp / PgDn]    - Scroll viewport half page up / down\n")
		sb.WriteString("  [Ctrl+U / Ctrl+D]- Scroll viewport half page up / down\n")
		sb.WriteString("  [Shift+Up / Down]- Scroll viewport 3 lines up / down\n")
		sb.WriteString("  [Ctrl+Home / End]- Jump to top / bottom of scrollback\n")
		sb.WriteString("  [Mouse Wheel]    - Scroll viewport smoothly\n")
		sb.WriteString("  [Ctrl+C]         - Quit")
		m.items = append(m.items, NewSystemItem(uuid.NewString(), sb.String()))

	case "/clear":
		m.items = make([]FeedItem, 0)
		m.focusedSurfaceIndex = -1
		m.focusMode = FocusInput
		m.items = append(m.items, NewSystemItem(uuid.NewString(), "Conversation history cleared."))

	case "/reset":
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
			return m, m.reconnectCmd(a2a.ClientOptions{AgentURL: url}, url)
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
			return m, m.reconnectCmd(a2a.ClientOptions{CardURL: url}, url)
		}

	case "/quit", "/exit":
		return m, tea.Quit

	default:
		m.items = append(m.items, NewErrorItem(uuid.NewString(), fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd)))
	}

	m.updateViewportContent()
	m.viewport.GotoBottom()
	return m, nil
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

		resp, err := client.SendMessage(ctx, text)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: target,
				phase:  "SendMessage Execution",
			}
		}
		return agentResponseMsg{text: resp}
	}
}

func (m *Model) sendActionCmd(actionName, sourceID string, contextValues map[string]any) tea.Cmd {
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

		resp, err := client.SendActionEvent(ctx, actionName, sourceID, contextValues)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: target,
				phase:  fmt.Sprintf("SendActionEvent (%s)", actionName),
			}
		}
		return agentResponseMsg{text: resp}
	}
}

func (m *Model) reconnectCmd(opts a2a.ClientOptions, targetURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cli, err := a2a.NewClient(ctx, opts)
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
	m.viewport.GotoBottom()

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
			m.viewport.GotoBottom()
		}
	}
	return *m, tea.Batch(cmds...)
}

func (m *Model) appendAgentResponseContent(content string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return
	}

	segments, err := a2ui.ParseAgentResponse(content, render.WithStyles(a2ui.DefaultTerminalStyles()))
	if err != nil || len(segments) == 0 {
		m.items = append(m.items, NewAgentTextItem(uuid.NewString(), content))
		return
	}

	for _, seg := range segments {
		if seg.Type == a2ui.TypeSurface && seg.Surface != nil {
			if m.ready {
				seg.Surface.SetSize(surfaceInnerWidth(m.width), m.viewport.Height())
			}
			m.items = append(m.items, NewAgentSurfaceItem(uuid.NewString(), "surface", seg.Surface, seg.Messages))
		} else if seg.Text != "" {
			m.items = append(m.items, NewAgentTextItem(uuid.NewString(), seg.Text))
		}
	}
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

func (m Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	headerHeight := lipgloss.Height(m.renderHeader())
	vpHeight := m.viewport.Height()

	// Only respond to left clicks for focus switching
	if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone || msg.Button == 0 {
		// 1. Click below the viewport (Input box, Toast, or Footer) -> focus chat input
		if msg.Y >= headerHeight+vpHeight {
			cmd := m.ReturnFocusToInput()
			m.updateViewportContent()
			return m, cmd
		}

		// 2. Click inside viewport -> check if an A2UI surface was clicked
		if msg.Y >= headerHeight && msg.Y < headerHeight+vpHeight {
			vpRow := msg.Y - headerHeight
			contentLine := m.viewport.YOffset() + vpRow
			itemIdx := m.findItemAtContentLine(contentLine)

			if itemIdx >= 0 && itemIdx < len(m.items) && m.items[itemIdx].Kind == KindAgentSurface && m.items[itemIdx].Surface != nil {
				cmd := m.setFocusedSurface(itemIdx)
				m.updateViewportContent()
				return m, cmd
			}
		}
	}

	// Also forward mouse click to viewport for standard selection / handling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}




