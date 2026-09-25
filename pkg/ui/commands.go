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

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/joestump-agent/a2tea/render"
	a2aclient "github.com/muncus/a2term/pkg/a2a"
	"github.com/muncus/a2term/pkg/a2ui"
)

// handleCommand parses and executes user slash commands.
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
