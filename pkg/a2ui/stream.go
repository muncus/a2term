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
	"encoding/json"
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

// ParseLineAsServerMessage attempts to decode a single line as an A2UI ServerMessage.
func ParseLineAsServerMessage(line string) (*tmca2ui.ServerMessage, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return nil, false
	}

	var msg tmca2ui.ServerMessage
	if err := json.Unmarshal([]byte(trimmed), &msg); err == nil {
		if msg.CreateSurface != nil || msg.UpdateComponents != nil || msg.UpdateDataModel != nil || msg.DeleteSurface != nil {
			return &msg, true
		}
	}
	return nil, false
}

// ParseServerMessagesFromJSON attempts to decode a JSON string as either a single ServerMessage or a slice of ServerMessages.
func ParseServerMessagesFromJSON(rawJSON string) ([]tmca2ui.ServerMessage, error) {
	trimmed := strings.TrimSpace(rawJSON)
	if trimmed == "" {
		return nil, nil
	}

	// Try single message
	if strings.HasPrefix(trimmed, "{") {
		var single tmca2ui.ServerMessage
		if err := json.Unmarshal([]byte(trimmed), &single); err == nil {
			if single.CreateSurface != nil || single.UpdateComponents != nil || single.UpdateDataModel != nil || single.DeleteSurface != nil {
				return []tmca2ui.ServerMessage{single}, nil
			}
		}
	}

	// Try array of messages
	if strings.HasPrefix(trimmed, "[") {
		var list []tmca2ui.ServerMessage
		if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
			var valid []tmca2ui.ServerMessage
			for _, m := range list {
				if m.CreateSurface != nil || m.UpdateComponents != nil || m.UpdateDataModel != nil || m.DeleteSurface != nil {
					valid = append(valid, m)
				}
			}
			if len(valid) > 0 {
				return valid, nil
			}
		}
	}

	return nil, nil
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
				if msgs, err := ParseServerMessagesFromJSON(remaining); err == nil && len(msgs) > 0 {
					messages = append(messages, msgs...)
				}
				break
			}

			jsonBlock := remaining[:closeIdx]
			remaining = remaining[closeIdx+len(tagClose):]

			if msgs, err := ParseServerMessagesFromJSON(jsonBlock); err == nil && len(msgs) > 0 {
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

		if msg, ok := ParseLineAsServerMessage(trimmedLine); ok {
			messages = append(messages, *msg)
			foundAnyMessage = true
		} else {
			nonJSONLines = append(nonJSONLines, line)
		}
	}

	if foundAnyMessage {
		return messages, strings.TrimSpace(strings.Join(nonJSONLines, "\n"))
	}

	// 3. Fallback: check if entire block is a JSON message or array
	if msgs, err := ParseServerMessagesFromJSON(content); err == nil && len(msgs) > 0 {
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

		if msg, ok := ParseLineAsServerMessage(trimmed); ok {
			events = append(events, StreamEvent{
				Message: msg,
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
