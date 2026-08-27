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

import tea "charm.land/bubbletea/v2"

// Export internal messages and inspectors for testing in ui_test package

// NewAgentResponseMsgForTest creates an agentResponseMsg for tests.
func NewAgentResponseMsgForTest(text string) tea.Msg {
	return agentResponseMsg{text: text}
}

// NewAgentStreamChunkMsgForTest creates an agentStreamChunkMsg for tests.
func NewAgentStreamChunkMsgForTest(chunk string, isFinal bool) tea.Msg {
	return agentStreamChunkMsg{chunk: chunk, isFinal: isFinal}
}

// ViewportYOffset returns the vertical scroll offset of the viewport.
func (m Model) ViewportYOffset() int {
	return m.viewport.YOffset()
}

// ViewportScrollPercent returns the scroll percentage of the viewport.
func (m Model) ViewportScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}

// ViewportAtBottom returns true if the viewport is scrolled to the bottom.
func (m Model) ViewportAtBottom() bool {
	return m.viewport.AtBottom()
}

// ViewportAtTop returns true if the viewport is scrolled to the top.
func (m Model) ViewportAtTop() bool {
	return m.viewport.AtTop()
}

// FocusedSurfaceIndex returns the index of the focused surface or -1.
func (m Model) FocusedSurfaceIndex() int {
	return m.focusedSurfaceIndex
}

// FocusMode returns the current focus mode.
func (m Model) FocusMode() FocusMode {
	return m.focusMode
}

// EnsureLineRangeVisibleForTest runs ensureLineRangeVisible for testing.
func (m *Model) EnsureLineRangeVisibleForTest(startLine, endLine int) {
	m.ensureLineRangeVisible(startLine, endLine)
}

