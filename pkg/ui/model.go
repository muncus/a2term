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

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/muncus/a2term/pkg/agent"
)

// FocusMode indicates which UI component currently has keyboard focus.
type FocusMode int

const (
	FocusInput FocusMode = iota
	FocusSurface
)

// Model represents the top-level Bubble Tea state for github.com/muncus/a2term.
type Model struct {
	client    agent.Client
	agentURL  string
	cardURL   string
	authToken string
	items     []FeedItem
	viewport  viewport.Model
	input     textinput.Model
	spinner   spinner.Model
	styles    Styles

	width int
	height int
	ready bool

	focusMode           FocusMode
	focusedSurfaceIndex int // Index into items pointing to the focused KindAgentSurface, or -1

	status        string
	isLoading     bool
	isStreaming   bool
	streamingText string
	streamItemID  string

	toast      string
	toastTicks int
}

// Config holds initial settings for creating a new Model.
type Config struct {
	Client     agent.Client
	AgentURL   string
	CardURL    string
	AuthToken  string
	InitialErr error
}

// NewModel initializes the Bubble Tea application model.
func NewModel(cfg Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7AF"))

	ti := textinput.New()
	ti.Placeholder = "Type a message or /help..."
	ti.Focus()
	ti.Prompt = "❯ "
	tiStyles := textinput.DefaultDarkStyles()
	tiStyles.Focused.Prompt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7AF"))
	ti.SetStyles(tiStyles)

	vp := viewport.New()
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	vp.SoftWrap = true

	styles := DefaultStyles()

	target := cfg.AgentURL
	if target == "" {
		target = cfg.CardURL
	}

	initialStatus := "Ready"
	if cfg.InitialErr != nil {
		initialStatus = "Connection Failed"
	} else if cfg.Client != nil {
		initialStatus = "Connected"
	} else if target == "" {
		initialStatus = "Disconnected"
	}

	m := Model{
		client:              cfg.Client,
		agentURL:            cfg.AgentURL,
		cardURL:             cfg.CardURL,
		authToken:           cfg.AuthToken,
		items:               make([]FeedItem, 0),
		viewport:            vp,
		input:               ti,
		spinner:             s,
		styles:              styles,
		focusMode:           FocusInput,
		focusedSurfaceIndex: -1,
		status:              initialStatus,
	}

	var welcome strings.Builder
	welcome.WriteString("✨ Welcome to github.com/muncus/a2term! Terminal client for A2A agents with A2UI.\n")
	welcome.WriteString("Type a message to chat, or use commands like /help, /clear, /agent <url>, /card <url>.\n")
	welcome.WriteString("When an A2UI card appears, press [Tab] to focus and interact with controls.")

	m.items = append(m.items, NewSystemItem("welcome", welcome.String()))

	if cfg.InitialErr != nil {
		diag := DiagnosticInfo{
			Title:     "Agent Connection Error",
			TargetURL: target,
			Phase:     "Startup Connection / Agent Card Discovery",
			ErrorMsg:  cfg.InitialErr.Error(),
			Tips: []string{
				"Ensure the agent server is running and accessible at the specified URL.",
				"Check if /.well-known/agent-card.json returns valid JSON.",
				"Use '/agent <url>' or '/card <url>' to reconnect once the agent is ready.",
				"Use '/help' to inspect command usage and options.",
			},
		}
		m.items = append(m.items, NewDiagnosticItem("init-err", diag))
	} else if cfg.Client != nil {
		m.items = append(m.items, NewSystemItem("conn-ok", fmt.Sprintf("✅ Connected to %s (%s)", cfg.Client.AgentName(), target)))
	} else {
		m.items = append(m.items, NewSystemItem("no-agent", "⚠️ No agent connected. Use '/agent <url>' or '/card <url>' to connect to an A2A agent."))
	}

	return m
}

// Init starts the blinking cursor and initial commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

// FindSurfaceIndices returns all indices in items that contain an interactive surface.
func (m *Model) FindSurfaceIndices() []int {
	var indices []int
	for i, item := range m.items {
		if item.Kind == KindAgentSurface && item.Surface != nil {
			indices = append(indices, i)
		}
	}
	return indices
}

// setFocusedSurface sets focus to the surface at the given index in items,
// or returns focus to chat input if index is -1.
func (m *Model) setFocusedSurface(index int) tea.Cmd {
	if index < 0 || index >= len(m.items) || m.items[index].Kind != KindAgentSurface || m.items[index].Surface == nil {
		return m.ReturnFocusToInput()
	}

	var cmds []tea.Cmd
	// Blur previously focused surface if different
	if m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) && m.focusedSurfaceIndex != index && m.items[m.focusedSurfaceIndex].Surface != nil {
		m.items[m.focusedSurfaceIndex].Surface.Blur()
	}

	m.input.Blur()
	m.focusMode = FocusSurface
	m.focusedSurfaceIndex = index
	if cmd := m.items[index].Surface.Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// FocusNextSurface moves focus to the next available A2UI surface, or wraps to input.
func (m *Model) FocusNextSurface() tea.Cmd {
	surfaces := m.FindSurfaceIndices()
	if len(surfaces) == 0 {
		return m.ReturnFocusToInput()
	}

	if m.focusMode == FocusInput {
		// Switch to the most recent surface
		return m.setFocusedSurface(surfaces[len(surfaces)-1])
	}

	// Currently on a surface: find next surface or cycle back to input
	for pos, idx := range surfaces {
		if idx == m.focusedSurfaceIndex {
			if pos+1 < len(surfaces) {
				return m.setFocusedSurface(surfaces[pos+1])
			}
			break
		}
	}

	return m.ReturnFocusToInput()
}

// FocusPreviousSurface moves focus to the previous A2UI surface or back to input.
func (m *Model) FocusPreviousSurface() tea.Cmd {
	surfaces := m.FindSurfaceIndices()
	if len(surfaces) == 0 {
		return m.ReturnFocusToInput()
	}

	if m.focusMode == FocusInput {
		return m.setFocusedSurface(surfaces[0])
	}

	for pos, idx := range surfaces {
		if idx == m.focusedSurfaceIndex {
			if pos > 0 {
				return m.setFocusedSurface(surfaces[pos-1])
			}
			break
		}
	}

	return m.ReturnFocusToInput()
}

// ReturnFocusToInput returns keyboard focus back to the text input box.
func (m *Model) ReturnFocusToInput() tea.Cmd {
	if m.focusedSurfaceIndex >= 0 && m.focusedSurfaceIndex < len(m.items) && m.items[m.focusedSurfaceIndex].Surface != nil {
		m.items[m.focusedSurfaceIndex].Surface.Blur()
	}
	m.focusMode = FocusInput
	m.focusedSurfaceIndex = -1
	m.viewport.GotoBottom()
	return m.input.Focus()
}

// surfaceInnerWidth computes the available inner width for an A2UI surface inside the surface container box.
// Outer SurfaceContainer width is (totalWidth - 4). The container has 2 cells border + 2 cells padding = 4 cells chrome,
// so inner width available to the surface without wrapping is totalWidth - 8.
func surfaceInnerWidth(totalWidth int) int {
	w := totalWidth - 8
	if w < 4 {
		return 4
	}
	return w
}


