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
