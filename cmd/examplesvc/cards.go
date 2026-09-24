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

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// wrapA2UI wraps raw JSON components in an A2UI v0.9 updateComponents message and <a2ui-json> tags.
func wrapA2UI(surfaceID string, components []map[string]any) string {
	msg := map[string]any{
		"version": "v0.9",
		"updateComponents": map[string]any{
			"surfaceId":  surfaceID,
			"components": components,
		},
	}
	data, _ := json.MarshalIndent(msg, "", "  ")
	return fmt.Sprintf("<a2ui-json>\n%s\n</a2ui-json>", string(data))
}

// showcaseCard returns a complete A2UI showcase containing all controls supported by github.com/muncus/a2term.
func showcaseCard() string {
	components := []map[string]any{
		{
			"component": "Card",
			"id":        "root",
			"child":     "main_col",
		},
		{
			"component": "Column",
			"id":        "main_col",
			"children": []string{
				"header_title",
				"header_desc",
				"div_1",
				"section_text",
				"row_badges",
				"div_2",
				"section_inputs",
				"cb_notifications",
				"cp_theme",
				"sl_volume",
				"tf_notes",
				"div_3",
				"section_actions",
				"row_buttons",
			},
		},
		{
			"component": "Text",
			"id":        "header_title",
			"text":      "✨ A2UI Interactive Component Showcase",
			"variant":   "h1",
		},
		{
			"component": "Text",
			"id":        "header_desc",
			"text":      "Tab / Shift+Tab to cycle focus • Enter/Space to interact • Esc to exit surface",
			"variant":   "caption",
		},
		{
			"component": "Divider",
			"id":        "div_1",
		},
		{
			"component": "Text",
			"id":        "section_text",
			"text":      "1. Layout & Status",
			"variant":   "h2",
		},
		{
			"component": "Row",
			"id":        "row_badges",
			"children":  []string{"badge_online", "badge_version", "badge_transport"},
		},
		{
			"component": "Text",
			"id":        "badge_online",
			"text":      "[🟢 Online]",
			"variant":   "body",
		},
		{
			"component": "Text",
			"id":        "badge_version",
			"text":      "[📦 A2UI v0.9]",
			"variant":   "body",
		},
		{
			"component": "Text",
			"id":        "badge_transport",
			"text":      "[⚡ REST + JSON-RPC]",
			"variant":   "body",
		},
		{
			"component": "Divider",
			"id":        "div_2",
		},
		{
			"component": "Text",
			"id":        "section_inputs",
			"text":      "2. Interactive Input Controls",
			"variant":   "h2",
		},
		{
			"component": "CheckBox",
			"id":        "cb_notifications",
			"label":     "Enable real-time event notifications",
			"value":     true,
		},
		{
			"component": "ChoicePicker",
			"id":        "cp_theme",
			"variant":   "radio",
			"options": []map[string]any{
				{"label": "Dark Modern", "value": "dark"},
				{"label": "Solarized", "value": "solarized"},
				{"label": "Cyberpunk", "value": "cyberpunk"},
			},
			"value": []string{"dark"},
		},
		{
			"component": "Slider",
			"id":        "sl_volume",
			"min":       0,
			"max":       100,
			"value":     75,
		},
		{
			"component": "TextField",
			"id":        "tf_notes",
			"label":     "User Notes / Search query",
			"value":     "github.com/muncus/a2term interactive test",
		},
		{
			"component": "Divider",
			"id":        "div_3",
		},
		{
			"component": "Text",
			"id":        "section_actions",
			"text":      "3. Action Triggers",
			"variant":   "h2",
		},
		{
			"component": "Row",
			"id":        "row_buttons",
			"children":  []string{"btn_submit", "btn_refresh", "btn_ping"},
		},
		{
			"component": "Button",
			"id":        "btn_submit",
			"child":     "btn_submit_text",
			"action": map[string]any{
				"event": map[string]any{
					"name": "submit_form",
					"context": map[string]any{
						"source": "showcase",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "btn_submit_text",
			"text":      "🚀 Submit Form",
		},
		{
			"component": "Button",
			"id":        "btn_refresh",
			"child":     "btn_refresh_text",
			"action": map[string]any{
				"event": map[string]any{
					"name": "refresh_data",
					"context": map[string]any{
						"action": "refresh",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "btn_refresh_text",
			"text":      "🔄 Refresh Data",
		},
		{
			"component": "Button",
			"id":        "btn_ping",
			"child":     "btn_ping_text",
			"action": map[string]any{
				"event": map[string]any{
					"name": "ping",
					"context": map[string]any{
						"type": "quick_ping",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "btn_ping_text",
			"text":      "⚡ Ping",
		},
	}

	return "Here is the interactive A2UI showcase surface:\n\n" + wrapA2UI("showcase-surface", components) + "\n\nTry navigating using Tab/Shift+Tab and triggering buttons with Enter!"
}

// buttonsCard returns a card focused on button interactions and events.
func buttonsCard() string {
	components := []map[string]any{
		{
			"component": "Card",
			"id":        "root",
			"child":     "btn_col",
		},
		{
			"component": "Column",
			"id":        "btn_col",
			"children": []string{
				"btn_title",
				"btn_desc",
				"div_btn",
				"row_actions",
			},
		},
		{
			"component": "Text",
			"id":        "btn_title",
			"text":      "🔘 Button Gallery",
			"variant":   "h1",
		},
		{
			"component": "Text",
			"id":        "btn_desc",
			"text":      "Select any button to trigger an A2UI action event back to the agent.",
			"variant":   "body",
		},
		{
			"component": "Divider",
			"id":        "div_btn",
		},
		{
			"component": "Row",
			"id":        "row_actions",
			"children":  []string{"btn_approve", "btn_reject", "btn_cancel"},
		},
		{
			"component": "Button",
			"id":        "btn_approve",
			"child":     "txt_approve",
			"action": map[string]any{
				"event": map[string]any{
					"name": "approve_request",
					"context": map[string]any{
						"decision": "approved",
						"item_id":  "item-994",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "txt_approve",
			"text":      "✅ Approve",
		},
		{
			"component": "Button",
			"id":        "btn_reject",
			"child":     "txt_reject",
			"action": map[string]any{
				"event": map[string]any{
					"name": "reject_request",
					"context": map[string]any{
						"decision": "rejected",
						"item_id":  "item-994",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "txt_reject",
			"text":      "❌ Reject",
		},
		{
			"component": "Button",
			"id":        "btn_cancel",
			"child":     "txt_cancel",
			"action": map[string]any{
				"event": map[string]any{
					"name": "cancel_action",
					"context": map[string]any{
						"decision": "cancel",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "txt_cancel",
			"text":      "⏹ Cancel",
		},
	}

	return "Here is the Button Gallery:\n\n" + wrapA2UI("button-surface", components)
}

// formCard returns a focused form with inputs and validation.
func formCard() string {
	components := []map[string]any{
		{
			"component": "Card",
			"id":        "root",
			"child":     "form_col",
		},
		{
			"component": "Column",
			"id":        "form_col",
			"children": []string{
				"form_title",
				"form_caption",
				"div_form_1",
				"tf_name",
				"tf_email",
				"cb_terms",
				"sl_rating",
				"div_form_2",
				"btn_save",
			},
		},
		{
			"component": "Text",
			"id":        "form_title",
			"text":      "📝 Feedback Form",
			"variant":   "h1",
		},
		{
			"component": "Text",
			"id":        "form_caption",
			"text":      "Edit the inputs below and click 'Submit Feedback' to send.",
			"variant":   "caption",
		},
		{
			"component": "Divider",
			"id":        "div_form_1",
		},
		{
			"component": "TextField",
			"id":        "tf_name",
			"label":     "Full Name",
			"value":     "Terminal Explorer",
		},
		{
			"component": "TextField",
			"id":        "tf_email",
			"label":     "Email Address",
			"value":     "explorer@example.com",
		},
		{
			"component": "CheckBox",
			"id":        "cb_terms",
			"label":     "I agree to the feedback guidelines",
			"value":     true,
		},
		{
			"component": "Slider",
			"id":        "sl_rating",
			"min":       1,
			"max":       5,
			"value":     5,
		},
		{
			"component": "Divider",
			"id":        "div_form_2",
		},
		{
			"component": "Button",
			"id":        "btn_save",
			"child":     "txt_save",
			"action": map[string]any{
				"event": map[string]any{
					"name": "submit_feedback",
					"context": map[string]any{
						"form": "feedback",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "txt_save",
			"text":      "📨 Submit Feedback",
		},
	}

	return "Feedback form ready:\n\n" + wrapA2UI("form-surface", components)
}

// weatherCard returns a weather information widget.
func weatherCard(city string, temp int, condition string) string {
	components := []map[string]any{
		{
			"component": "Card",
			"id":        "root",
			"child":     "weather_col",
		},
		{
			"component": "Column",
			"id":        "weather_col",
			"children": []string{
				"w_title",
				"w_temp",
				"w_details",
				"w_div",
				"w_refresh_btn",
			},
		},
		{
			"component": "Text",
			"id":        "w_title",
			"text":      fmt.Sprintf("🌤️ Weather for %s", city),
			"variant":   "h1",
		},
		{
			"component": "Text",
			"id":        "w_temp",
			"text":      fmt.Sprintf("%d°F • %s", temp, condition),
			"variant":   "h2",
		},
		{
			"component": "Text",
			"id":        "w_details",
			"text":      "Humidity: 58% | Wind: 7 mph WNW | UV Index: Moderate",
			"variant":   "caption",
		},
		{
			"component": "Divider",
			"id":        "w_div",
		},
		{
			"component": "Button",
			"id":        "w_refresh_btn",
			"child":     "w_refresh_text",
			"action": map[string]any{
				"event": map[string]any{
					"name": "refresh_weather",
					"context": map[string]any{
						"city": city,
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "w_refresh_text",
			"text":      "🔄 Refresh Forecast",
		},
	}

	return fmt.Sprintf("Current forecast for %s:\n\n", city) + wrapA2UI("weather-surface", components)
}

// actionResultCard generates an updated card when an action event is triggered.
func actionResultCard(actionName string, sourceID string, ctxValues map[string]any) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎯 **Action Received:** `%s` (source: `%s`)\n\n", actionName, sourceID))

	if len(ctxValues) > 0 {
		ctxJSON, _ := json.MarshalIndent(ctxValues, "", "  ")
		sb.WriteString("Context Data:\n```json\n" + string(ctxJSON) + "\n```\n\n")
	}

	components := []map[string]any{
		{
			"component": "Card",
			"id":        "root",
			"child":     "result_col",
		},
		{
			"component": "Column",
			"id":        "result_col",
			"children": []string{
				"res_title",
				"res_msg",
				"res_div",
				"res_btn",
			},
		},
		{
			"component": "Text",
			"id":        "res_title",
			"text":      fmt.Sprintf("Action Processed: %s", actionName),
			"variant":   "h1",
		},
		{
			"component": "Text",
			"id":        "res_msg",
			"text":      fmt.Sprintf("Successfully handled event from '%s'. The server state has been updated.", sourceID),
			"variant":   "body",
		},
		{
			"component": "Divider",
			"id":        "res_div",
		},
		{
			"component": "Button",
			"id":        "res_btn",
			"child":     "res_btn_text",
			"action": map[string]any{
				"event": map[string]any{
					"name": "return_to_showcase",
					"context": map[string]any{
						"previous_action": actionName,
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "res_btn_text",
			"text":      "🔙 Return to Showcase",
		},
	}

	sb.WriteString(wrapA2UI("result-surface", components))
	return sb.String()
}

// A2UIMIMEType is the standard MIME type for A2UI payloads.
const A2UIMIMEType = "application/a2ui+json"

// multipartCardComponents returns the A2UI components for the multi-part demo surface.
func multipartCardComponents() []map[string]any {
	return []map[string]any{
		{
			"component": "Card",
			"id":        "root",
			"child":     "multipart_col",
		},
		{
			"component": "Column",
			"id":        "multipart_col",
			"children": []string{
				"mp_title",
				"mp_desc",
				"mp_div_1",
				"mp_row_badges",
				"mp_div_2",
				"mp_body",
				"mp_div_3",
				"mp_row_actions",
			},
		},
		{
			"component": "Text",
			"id":        "mp_title",
			"text":      "🔀 Multi-Part A2A Message",
			"variant":   "h1",
		},
		{
			"component": "Text",
			"id":        "mp_desc",
			"text":      "Demonstrating an A2A message with both text/plain and application/a2ui+json parts.",
			"variant":   "caption",
		},
		{
			"component": "Divider",
			"id":        "mp_div_1",
		},
		{
			"component": "Row",
			"id":        "mp_row_badges",
			"children":  []string{"mp_badge_parts", "mp_badge_mime", "mp_badge_status"},
		},
		{
			"component": "Text",
			"id":        "mp_badge_parts",
			"text":      "[📦 Multi-Part A2A]",
			"variant":   "body",
		},
		{
			"component": "Text",
			"id":        "mp_badge_mime",
			"text":      "[🏷️ application/a2ui+json]",
			"variant":   "body",
		},
		{
			"component": "Text",
			"id":        "mp_badge_status",
			"text":      "[🟢 Connected]",
			"variant":   "body",
		},
		{
			"component": "Divider",
			"id":        "mp_div_2",
		},
		{
			"component": "Text",
			"id":        "mp_body",
			"text":      "This surface was delivered via an A2A DataPart carrying declarative A2UI components alongside a separate text part in the same response.",
			"variant":   "body",
		},
		{
			"component": "Divider",
			"id":        "mp_div_3",
		},
		{
			"component": "Row",
			"id":        "mp_row_actions",
			"children":  []string{"btn_mp_action", "btn_mp_return"},
		},
		{
			"component": "Button",
			"id":        "btn_mp_action",
			"child":     "txt_mp_action",
			"action": map[string]any{
				"event": map[string]any{
					"name": "multipart_test_action",
					"context": map[string]any{
						"source": "multipart_card",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "txt_mp_action",
			"text":      "⚡ Test Action",
		},
		{
			"component": "Button",
			"id":        "btn_mp_return",
			"child":     "txt_mp_return",
			"action": map[string]any{
				"event": map[string]any{
					"name": "return_to_showcase",
					"context": map[string]any{
						"previous": "multipart",
					},
				},
			},
		},
		{
			"component": "Text",
			"id":        "txt_mp_return",
			"text":      "🔙 Return to Showcase",
		},
	}
}

// newA2UIDataPart creates an A2A DataPart containing A2UI messages with the
// standard "application/a2ui+json" MIME type set in both MediaType and metadata.
func newA2UIDataPart(surfaceID string, components []map[string]any) *a2a.Part {
	uiPayload := []map[string]any{
		{
			"version": "v0.9",
			"updateComponents": map[string]any{
				"surfaceId":  surfaceID,
				"components": components,
			},
		},
	}
	part := a2a.NewDataPart(uiPayload)
	part.MediaType = A2UIMIMEType
	part.SetMeta("mimeType", A2UIMIMEType)
	return part
}

// multipartCard returns an A2A Message containing both a text part and an A2UI DataPart
// with MIME type "application/a2ui+json".
func multipartCard() *a2a.Message {
	return multipartMessage()
}

// multipartMessage returns an A2A Message containing both a text part and an A2UI DataPart
// with MIME type "application/a2ui+json".
func multipartMessage() *a2a.Message {
	textPart := a2a.NewTextPart("Here is a multi-part response with text and interactive A2UI:")
	textPart.MediaType = "text/plain"

	uiPart := newA2UIDataPart("multipart-surface", multipartCardComponents())

	return a2a.NewMessage(a2a.MessageRoleAgent, textPart, uiPart)
}

