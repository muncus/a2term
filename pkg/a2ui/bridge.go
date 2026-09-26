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
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/event"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"
)

// Standard A2UI MIME types and constants for A2A transport.
const (
	A2UIMIMEType       = "application/a2ui+json"
	A2UIMIMETypeLegacy = "application/json+a2ui"
	A2UIBasicCatalogID = "https://a2ui.org/catalogs/v0.9/basic.json"
)

// ContentType identifies whether a parsed part is plain text or an interactive A2UI surface.
type ContentType int

const (
	TypeText ContentType = iota
	TypeSurface
)

// ParsedSegment is a unit of an agent response: either prose text or a rendered A2UI surface model.
type ParsedSegment struct {
	Type      ContentType
	Text      string
	SurfaceID string
	Messages  []tmca2ui.ServerMessage
	Surface   render.Model
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
				surfID := ""
				for _, msg := range part.Messages {
					if msg.CreateSurface != nil && msg.CreateSurface.SurfaceID != "" {
						surfID = msg.CreateSurface.SurfaceID
						break
					}
					if msg.UpdateComponents != nil && msg.UpdateComponents.SurfaceID != "" {
						surfID = msg.UpdateComponents.SurfaceID
						break
					}
				}
				segments = append(segments, ParsedSegment{
					Type:      TypeSurface,
					SurfaceID: surfID,
					Messages:  part.Messages,
					Surface:   rm,
				})
			}
		}
	}

	return segments, nil
}

// IsA2UIPart reports whether an A2A Part carries an A2UI payload based on its
// MediaType or metadata mimeType.
func IsA2UIPart(part *a2a.Part) bool {
	if part == nil {
		return false
	}
	if part.MediaType == A2UIMIMEType || part.MediaType == A2UIMIMETypeLegacy {
		return true
	}
	if part.Metadata != nil {
		if mime, ok := part.Metadata["mimeType"].(string); ok {
			if mime == A2UIMIMEType || mime == A2UIMIMETypeLegacy {
				return true
			}
		}
	}
	return false
}

// ExtractServerMessages decodes A2UI server messages from a Part's Data, Raw, or Text.
func ExtractServerMessages(part *a2a.Part) ([]tmca2ui.ServerMessage, error) {
	if part == nil {
		return nil, fmt.Errorf("nil part")
	}

	var rawJSON []byte

	if d := part.Data(); d != nil {
		bytes, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("marshal part data: %w", err)
		}
		rawJSON = bytes
	} else if r := part.Raw(); len(r) > 0 {
		rawJSON = r
	} else if t := part.Text(); t != "" {
		rawJSON = []byte(t)
	} else {
		return nil, fmt.Errorf("part carries no data, raw bytes, or text")
	}

	return ParseServerMessages(rawJSON)
}

// surfaceIDFromMessages finds the surface ID targeted by the server messages.
func surfaceIDFromMessages(msgs []tmca2ui.ServerMessage) string {
	for _, msg := range msgs {
		if msg.CreateSurface != nil && msg.CreateSurface.SurfaceID != "" {
			return msg.CreateSurface.SurfaceID
		}
		if msg.UpdateComponents != nil && msg.UpdateComponents.SurfaceID != "" {
			return msg.UpdateComponents.SurfaceID
		}
		if msg.UpdateDataModel != nil && msg.UpdateDataModel.SurfaceID != "" {
			return msg.UpdateDataModel.SurfaceID
		}
		if msg.DeleteSurface != nil && msg.DeleteSurface.SurfaceID != "" {
			return msg.DeleteSurface.SurfaceID
		}
	}
	return "surface"
}

// ParseAgentParts parses a slice of A2A Parts, preserving metadata and decoding
// interactive A2UI surfaces or plain text segments.
func ParseAgentParts(parts []*a2a.Part, customStyles ...render.Option) ([]ParsedSegment, error) {
	var segments []ParsedSegment

	for _, part := range parts {
		if part == nil {
			continue
		}

		// 1. A2UI Data/Raw part with application/a2ui+json MIME type
		if IsA2UIPart(part) {
			msgs, err := ExtractServerMessages(part)
			if err == nil && len(msgs) > 0 {
				surfaceModel, err := a2tea.Render(msgs, customStyles...)
				if err == nil && surfaceModel != nil {
					var rm render.Model
					if rModel, ok := surfaceModel.(render.Model); ok {
						rm = rModel
					}
					surfID := surfaceIDFromMessages(msgs)
					segments = append(segments, ParsedSegment{
						Type:      TypeSurface,
						SurfaceID: surfID,
						Messages:  msgs,
						Surface:   rm,
					})
					continue
				}
			}
		}

		// 2. Prose / Text Part
		if text := part.Text(); text != "" {
			// Check if text embeds legacy <a2ui-json> tags
			if a2tea.Contains(text) {
				subSegs, err := ParseAgentResponse(text, customStyles...)
				if err == nil && len(subSegs) > 0 {
					segments = append(segments, subSegs...)
					continue
				}
			}
			trimmed := strings.TrimSpace(text)
			if trimmed != "" {
				segments = append(segments, ParsedSegment{
					Type: TypeText,
					Text: trimmed,
				})
			}
			continue
		}

		// 3. Multimodal / Raw bytes fallback
		if r := part.Raw(); len(r) > 0 {
			label := fmt.Sprintf("[Binary data: %s (%d bytes)]", part.MediaType, len(r))
			if strings.HasPrefix(part.MediaType, "image/") {
				name := part.Filename
				if name == "" {
					name = "image"
				}
				label = fmt.Sprintf("[🖼️ Image: %s (%s, %d bytes)]", name, part.MediaType, len(r))
			}
			segments = append(segments, ParsedSegment{
				Type: TypeText,
				Text: label,
			})
			continue
		}

		// 4. URL / Attachment fallback
		if u := part.URL(); u != "" {
			segments = append(segments, ParsedSegment{
				Type: TypeText,
				Text: fmt.Sprintf("[Attachment: %s]", u),
			})
			continue
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
