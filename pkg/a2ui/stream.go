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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	tmca2ui "github.com/tmc/a2ui"
)

const (
	tagOpen  = "<a2ui-json>"
	tagClose = "</a2ui-json>"
)

// StreamEvent represents an item extracted from an A2UI stream chunk.
type StreamEvent struct {
	Text     string
	Message  *tmca2ui.ServerMessage
	IsA2UI   bool
}

// isServerMessage reports whether a decoded ServerMessage contains an active mutation payload.
func isServerMessage(m *tmca2ui.ServerMessage) bool {
	return m != nil && (m.CreateSurface != nil || m.UpdateComponents != nil || m.UpdateDataModel != nil || m.DeleteSurface != nil)
}

// ParseServerMessages attempts to decode raw JSON as either a single ServerMessage or a slice of ServerMessages.
func ParseServerMessages(raw []byte) ([]tmca2ui.ServerMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// 1. Array of messages
	if trimmed[0] == '[' {
		var list []tmca2ui.ServerMessage
		if err := json.Unmarshal(trimmed, &list); err == nil {
			var valid []tmca2ui.ServerMessage
			for _, m := range list {
				if isServerMessage(&m) {
					valid = append(valid, m)
				}
			}
			if len(valid) > 0 {
				return valid, nil
			}
		}
	}

	// 2. Single message
	if trimmed[0] == '{' {
		var single tmca2ui.ServerMessage
		if err := json.Unmarshal(trimmed, &single); err == nil && isServerMessage(&single) {
			return []tmca2ui.ServerMessage{single}, nil
		}
	}

	return nil, fmt.Errorf("could not decode A2UI server messages from payload")
}

// ExtractMessagesAndText extracts all A2UI ServerMessages and conversational prose text from content.
// It handles:
// 1. Tagged blocks: <a2ui-json>...</a2ui-json>
// 2. Pure line-delimited JSONL stream (where lines are individual JSON messages)
// 3. Conversational prose interleaved with JSON
func ExtractMessagesAndText(content string) (messages []tmca2ui.ServerMessage, proseText string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, ""
	}

	// 1. Check for <a2ui-json> tags
	if strings.Contains(content, tagOpen) {
		var proseSb strings.Builder
		remaining := content
		for {
			openIdx := strings.Index(remaining, tagOpen)
			if openIdx < 0 {
				proseSb.WriteString(remaining)
				break
			}
			proseSb.WriteString(remaining[:openIdx])
			remaining = remaining[openIdx+len(tagOpen):]

			closeIdx := strings.Index(remaining, tagClose)
			if closeIdx < 0 {
				// Unclosed tag, take the rest as JSON
				if msgs, err := ParseServerMessages([]byte(remaining)); err == nil && len(msgs) > 0 {
					messages = append(messages, msgs...)
				}
				break
			}

			jsonBlock := remaining[:closeIdx]
			remaining = remaining[closeIdx+len(tagClose):]

			if msgs, err := ParseServerMessages([]byte(jsonBlock)); err == nil && len(msgs) > 0 {
				messages = append(messages, msgs...)
			}
		}
		return messages, strings.TrimSpace(proseSb.String())
	}

	// 2. Check for line-by-line JSONL
	scanner := bufio.NewScanner(strings.NewReader(content))
	var nonJSONLines []string
	foundAnyMessage := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		if msgs, err := ParseServerMessages([]byte(trimmedLine)); err == nil && len(msgs) == 1 {
			messages = append(messages, msgs[0])
			foundAnyMessage = true
		} else {
			nonJSONLines = append(nonJSONLines, line)
		}
	}

	if foundAnyMessage {
		return messages, strings.TrimSpace(strings.Join(nonJSONLines, "\n"))
	}

	// 3. Fallback: check if entire block is a JSON message or array
	if msgs, err := ParseServerMessages([]byte(content)); err == nil && len(msgs) > 0 {
		return msgs, ""
	}

	// Plain text
	return nil, trimmed
}

// ReadJSONLStream reads an io.Reader line-by-line and emits StreamEvents.
func ReadJSONLStream(r io.Reader) ([]StreamEvent, error) {
	scanner := bufio.NewScanner(r)
	var events []StreamEvent

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if msgs, err := ParseServerMessages([]byte(trimmed)); err == nil && len(msgs) == 1 {
			events = append(events, StreamEvent{
				Message: &msgs[0],
				IsA2UI:  true,
			})
		} else {
			events = append(events, StreamEvent{
				Text:   line,
				IsA2UI: false,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return events, err
	}
	return events, nil
}
