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
	"strings"
)

var defaultIconGlyphs = map[string]string{
	"search":         "🔍",
	"settings":       "⚙",
	"user":           "👤",
	"person":         "👤",
	"account":        "👤",
	"star":           "★",
	"arrow_forward":  "→",
	"arrow_right":    "→",
	"arrow_back":     "←",
	"arrow_left":     "←",
	"arrow_up":       "↑",
	"arrow_down":     "↓",
	"warning":        "⚠",
	"info":           "ℹ",
	"error":          "✖",
	"check":          "✓",
	"done":           "✓",
	"close":          "✕",
	"cancel":         "✕",
	"home":           "🏠",
	"menu":           "☰",
	"refresh":        "↻",
	"delete":         "🗑",
	"trash":          "🗑",
	"edit":           "✏",
	"mail":           "✉",
	"email":          "✉",
	"lock":           "🔒",
	"unlock":         "🔓",
	"heart":          "♥",
	"share":          "↗",
	"bell":           "🔔",
	"notification":   "🔔",
	"calendar":       "📅",
	"clock":          "🕒",
	"time":           "🕒",
	"folder":         "📁",
	"file":           "📄",
	"download":       "📥",
	"upload":         "📤",
	"play":           "▶",
	"pause":          "⏸",
	"stop":           "⏹",
}

// IconGlyph returns a Unicode glyph representation for a given well-known icon name.
// If the icon is not in the dictionary, it returns a formatted bracketed fallback: ⟨name⟩.
func IconGlyph(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.ReplaceAll(clean, "-", "_")
	if glyph, ok := defaultIconGlyphs[clean]; ok {
		return glyph
	}
	if clean == "" {
		return "⟨icon⟩"
	}
	return "⟨" + clean + "⟩"
}
