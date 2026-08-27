package a2ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/event"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"
)

// ContentType identifies whether a parsed part is plain text or an interactive A2UI surface.
type ContentType int

const (
	TypeText ContentType = iota
	TypeSurface
)

// ParsedSegment is a unit of an agent response: either prose text or a rendered A2UI surface model.
type ParsedSegment struct {
	Type     ContentType
	Text     string
	Messages []tmca2ui.ServerMessage
	Surface  render.Model
}

// ParseAgentResponse parses an agent response string, extracting text segments and rendering any A2UI components.
func ParseAgentResponse(content string, customStyles ...render.Option) ([]ParsedSegment, error) {
	if !a2tea.Contains(content) {
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			return nil, nil
		}
		return []ParsedSegment{
			{
				Type: TypeText,
				Text: trimmed,
			},
		}, nil
	}

	parts, err := a2tea.Scan(content)
	if err != nil {
		// Fallback to plain text on scan error
		return []ParsedSegment{
			{
				Type: TypeText,
				Text: content,
			},
		}, nil
	}

	var segments []ParsedSegment
	for _, part := range parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			segments = append(segments, ParsedSegment{
				Type: TypeText,
				Text: text,
			})
		}

		if len(part.Messages) > 0 {
			surfaceModel, err := a2tea.Render(part.Messages, customStyles...)
			if err == nil && surfaceModel != nil {
				var rm render.Model
				if rModel, ok := surfaceModel.(render.Model); ok {
					rm = rModel
				}
				segments = append(segments, ParsedSegment{
					Type:     TypeSurface,
					Messages: part.Messages,
					Surface:  rm,
				})
			}
		}
	}

	return segments, nil
}

// RenderMessages creates an interactive A2UI surface from a list of A2UI server messages.
func RenderMessages(msgs []tmca2ui.ServerMessage, opts ...render.Option) (render.Model, error) {
	m, err := a2tea.Render(msgs, opts...)
	if err != nil {
		return nil, err
	}
	if rm, ok := m.(render.Model); ok {
		return rm, nil
	}
	return nil, nil
}

// ActionSummary formats an interaction event for display and logging.
func ActionSummary(msg tea.Msg) (name string, source string, summary string) {
	switch ev := msg.(type) {
	case event.ButtonClicked:
		actionName := "click"
		if ev.Action != nil && ev.Action.Name != "" {
			actionName = ev.Action.Name
		}
		return actionName, ev.Source.ComponentID, fmt.Sprintf("Button clicked: %s (id: %s)", actionName, ev.Source.ComponentID)
	case tmca2ui.ClientMessage:
		if ev.Action != nil {
			return ev.Action.Name, ev.Action.SourceComponentID, fmt.Sprintf("Action triggered: %s (source: %s, ctx: %v)", ev.Action.Name, ev.Action.SourceComponentID, ev.Action.Context)
		}
		return "client_message", "", "A2UI Client Message"
	case event.InputSubmitted:
		return "input_submitted", ev.Source.ComponentID, fmt.Sprintf("Input submitted: %q (id: %s)", ev.Value, ev.Source.ComponentID)
	case event.ChoiceSelected:
		return "choice_selected", ev.Source.ComponentID, fmt.Sprintf("Choice selected: %v (id: %s)", ev.Values, ev.Source.ComponentID)
	default:
		return "", "", ""
	}
}

// DefaultTerminalStyles returns a clean styling theme for A2UI surfaces.
func DefaultTerminalStyles() render.Styles {
	st := render.DefaultStyles()
	st.CardBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)
	st.Button = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4338CA")).
		Padding(0, 2).
		Bold(true)
	st.ButtonFocused = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#38BDF8")).
		Padding(0, 2).
		Bold(true)
	st.Heading = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F43F5E")).
		MarginBottom(1)
	st.Subheading = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FB923C"))
	st.Caption = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6ADC8")).
		Italic(true)
	return st
}
