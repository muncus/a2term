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
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/joestump-agent/a2tea/render"
)

// handleKeyPress processes keyboard navigation, shortcuts, and input submissions.
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
