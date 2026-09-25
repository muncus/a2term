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

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the complete UI layout.
func (m Model) View() tea.View {
	if !m.ready {
		v := tea.NewView("Initializing github.com/muncus/a2term...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	var sections []string

	// Header Bar
	headerView := m.renderHeader()
	sections = append(sections, headerView)

	// Tab Bar
	tabBarView := m.renderTabBar()
	sections = append(sections, tabBarView)

	// Main Chat / Surfaces Viewport
	sections = append(sections, m.viewport.View())

	// Action Toast Bar (if present)
	if m.toast != "" {
		sections = append(sections, m.styles.ToastBox.Render(m.toast))
	}

	// Input Prompt Box (only visible on Chat tab)
	if m.activeTab == TabChat {
		sections = append(sections, m.renderInput())
	}

	// Footer Shortcuts Bar
	sections = append(sections, m.renderFooter())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Safety guard: ensure total view height never exceeds m.height so the footer is never pushed off-screen
	if m.height > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > m.height {
			excess := len(lines) - m.height
			headerH := lipgloss.Height(headerView)
			if headerH < len(lines)-excess {
				lines = append(lines[:headerH], lines[headerH+excess:]...)
				content = strings.Join(lines, "\n")
			}
		}
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *Model) renderHeader() string {
	title := m.styles.HeaderTitle.Render("github.com/muncus/a2term")

	var circle string
	if m.IsConnected() {
		circle = m.styles.HeaderConnectedDot.Render("●")
	} else {
		circle = m.styles.HeaderDisconnectedDot.Render("●")
	}

	agentName := "None"
	if m.client != nil && m.client.AgentName() != "" {
		agentName = m.client.AgentName()
	} else if m.agentURL != "" {
		agentName = m.agentURL
	} else if m.cardURL != "" {
		agentName = m.cardURL
	}
	agent := m.styles.HeaderAgent.Render(agentName)

	var sessionBadge string
	if m.client != nil {
		sess := m.client.CurrentSession()
		if sess.TaskID != "" {
			sessionBadge = m.styles.HeaderTask.Render(fmt.Sprintf("Task: %s", sess.TaskID))
		} else if sess.ContextID != "" {
			sessionBadge = m.styles.HeaderTask.Render(fmt.Sprintf("Ctx: %s", sess.ContextID))
		}
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", circle, " ", agent)
	right := sessionBadge

	insideWidth := m.width - 2
	if insideWidth < 20 {
		insideWidth = 20
	}

	// Adapt header items so they never wrap to a second line
	if lipgloss.Width(left)+lipgloss.Width(right)+1 > insideWidth {
		right = ""
	}
	if lipgloss.Width(left)+lipgloss.Width(right)+1 > insideWidth {
		maxAgentLen := insideWidth - lipgloss.Width(title) - 6
		if maxAgentLen > 3 && len(agentName) > maxAgentLen {
			agent = m.styles.HeaderAgent.Render(agentName[:maxAgentLen-3] + "...")
			left = lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", circle, " ", agent)
		}
	}

	gap := insideWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	space := strings.Repeat(" ", gap)

	return m.styles.Header.Width(m.width).Render(left + space + right)
}

func (m *Model) renderTabBar() string {
	// Tab 1: Chat
	t1Label := " 1 Chat "
	var t1 string
	if m.activeTab == TabChat {
		t1 = m.styles.TabActive.Render(t1Label)
	} else {
		t1 = m.styles.TabInactive.Render(t1Label)
	}

	// Tab 2: Surfaces
	surfCount := len(m.FindSurfaceIndices())
	var t2Label string
	if surfCount > 0 {
		t2Label = fmt.Sprintf(" 2 Surfaces (%d) ", surfCount)
	} else {
		t2Label = " 2 Surfaces "
	}
	var t2 string
	if m.activeTab == TabSurfaces {
		t2 = m.styles.TabActive.Render(t2Label)
	} else {
		t2 = m.styles.TabInactive.Render(t2Label)
	}

	// Tab 3: Logs
	t3Label := " 3 Logs "
	var t3 string
	if m.activeTab == TabLogs {
		t3 = m.styles.TabActive.Render(t3Label)
	} else {
		t3 = m.styles.TabInactive.Render(t3Label)
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, "  ", t1, "  ", t2, "  ", t3)
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A")).Render(strings.Repeat("─", m.width))
	return m.styles.TabBar.Width(m.width).Render(bar) + "\n" + divider
}

// tabAtX determines which Tab was clicked given an X coordinate on the tab bar row.
func (m *Model) tabAtX(x int) (Tab, bool) {
	curX := 2

	// Tab 1: Chat
	w1 := lipgloss.Width(" 1 Chat ")
	if x >= curX && x < curX+w1 {
		return TabChat, true
	}
	curX += w1 + 2

	// Tab 2: Surfaces
	surfCount := len(m.FindSurfaceIndices())
	var t2Label string
	if surfCount > 0 {
		t2Label = fmt.Sprintf(" 2 Surfaces (%d) ", surfCount)
	} else {
		t2Label = " 2 Surfaces "
	}
	w2 := lipgloss.Width(t2Label)
	if x >= curX && x < curX+w2 {
		return TabSurfaces, true
	}
	curX += w2 + 2

	// Tab 3: Logs
	w3 := lipgloss.Width(" 3 Logs ")
	if x >= curX && x < curX+w3 {
		return TabLogs, true
	}

	return TabChat, false
}

func (m *Model) renderInput() string {
	inputView := m.input.View()
	if m.focusMode == FocusInput {
		return m.styles.InputFocusedContainer.Width(m.width - 2).Render(inputView)
	}
	return m.styles.InputContainer.Width(m.width - 2).Render(inputView)
}

func (m *Model) renderFooter() string {

	activeVP := m.activeViewport()
	var scrollBadge string
	if activeVP.AtTop() && activeVP.AtBottom() {
		scrollBadge = m.styles.ScrollBadge.Render("📜 All")
	} else if activeVP.AtBottom() {
		scrollBadge = m.styles.ScrollBadge.Render("📜 Bottom")
	} else if activeVP.AtTop() {
		scrollBadge = m.styles.ScrollAlert.Render("⬆ Top (0%) • [End: Bottom]")
	} else {
		pct := int(activeVP.ScrollPercent() * 100)
		scrollBadge = m.styles.ScrollAlert.Render(fmt.Sprintf("⬇ %d%% • [End: Bottom]", pct))
	}

	insideWidth := m.width - 2
	if insideWidth < 20 {
		insideWidth = 20
	}

	var left string
	if m.isLoading || m.isStreaming {
		statusText := fmt.Sprintf("%s %s", m.spinner.View(), m.status)
		left = m.styles.FooterStatus.Render(statusText)
	} else if m.status == "Connection Failed" || m.status == "Error" || m.status == "Disconnected" || m.status == "Auth Failed" {
		statusText := fmt.Sprintf("● %s", m.status)
		left = m.styles.HeaderStatusError.Render(statusText)
	} else {
		// Placeholder on the left where the spinner appears
		statusText := "Ready"
		if m.status != "" && m.status != "Thinking..." && m.status != "Streaming..." && m.status != "Sending action..." {
			statusText = m.status
		}
		left = m.styles.FooterDesc.Render(fmt.Sprintf("● %s", statusText))
	}

	gap := insideWidth - lipgloss.Width(left) - lipgloss.Width(scrollBadge)
	if gap < 1 {
		gap = 1
	}
	space := strings.Repeat(" ", gap)

	return m.styles.Footer.Width(m.width).Render(left + space + scrollBadge)
}

func (m *Model) updateViewportContent() {
	m.updateChatViewportContent()
	m.updateSurfacesViewportContent()
	m.updateLogsViewportContent()
	m.syncActiveViewportMirror()
}

func (m *Model) updateChatViewportContent() {
	var sb strings.Builder
	for i, item := range m.items {
		sb.WriteString(m.renderChatFeedItem(i, item))
	}
	m.chatViewport.SetContent(sb.String())
}

func (m *Model) renderChatFeedItem(idx int, item FeedItem) string {
	switch item.Kind {
	case KindUser:
		role := m.styles.UserRole.Render(fmt.Sprintf("👤 You (%s)", item.Timestamp.Format("15:04")))
		text := m.styles.UserText.Render(item.Content)
		msg := fmt.Sprintf("%s\n%s", role, text)
		return m.styles.UserMessageBox.Width(m.width-4).Render(msg) + "\n"

	case KindAgentText:
		role := m.styles.AgentRole.Render(fmt.Sprintf("🤖 Agent (%s)", item.Timestamp.Format("15:04")))
		text := m.styles.AgentText.Render(item.Content)
		msg := fmt.Sprintf("%s\n%s", role, text)
		return m.styles.AgentMessageBox.Width(m.width-4).Render(msg) + "\n"

	case KindAgentSurface:
		surfID := item.SurfaceID
		if surfID == "" {
			surfID = "A2UI Component"
		}
		badge := m.styles.SurfaceBadge.Render("📦 A2UI Surface: " + surfID)
		hint := m.styles.FooterDesc.Render("Switch to [2 Surfaces] tab (Press F2 or click Surfaces) to view and interact.")
		cardNotice := m.styles.SurfaceNoticeBox.Width(m.width - noticePaddingH).Render(fmt.Sprintf("%s\n%s", badge, hint))
		return cardNotice + "\n"

	case KindSystem:
		return m.styles.SystemMessage.Render("ℹ️ "+item.Content) + "\n\n"

	case KindDiagnostic:
		if item.Diagnostic != nil {
			d := item.Diagnostic
			var diagLines []string
			diagLines = append(diagLines, m.styles.DiagnosticTitle.Render(fmt.Sprintf("⚠️ DIAGNOSTIC: %s", d.Title)))
			if d.TargetURL != "" {
				diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Target URL:"), m.styles.DiagnosticValue.Render(d.TargetURL)))
			}
			if d.Phase != "" {
				diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Phase:"), m.styles.DiagnosticValue.Render(d.Phase)))
			}
			if d.ErrorMsg != "" {
				diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Error:"), m.styles.DiagnosticValue.Render(d.ErrorMsg)))
			}
			if d.Details != "" {
				diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Details:"), m.styles.DiagnosticValue.Render(d.Details)))
			}
			if len(d.Tips) > 0 {
				diagLines = append(diagLines, "")
				diagLines = append(diagLines, m.styles.DiagnosticLabel.Render("Troubleshooting Tips:"))
				for _, tip := range d.Tips {
					diagLines = append(diagLines, m.styles.DiagnosticTip.Render("  • "+tip))
				}
			}
			content := strings.Join(diagLines, "\n")
			return m.styles.DiagnosticBox.Width(m.width-surfaceContainerPad).Render(content) + "\n"
		}
		return ""

	case KindError:
		return m.styles.ErrorMessage.Render("❌ "+item.Content) + "\n"

	default:
		return ""
	}
}

func (m *Model) updateSurfacesViewportContent() {
	surfaces := m.FindSurfaceIndices()
	if len(surfaces) == 0 {
		emptyBox := m.styles.SurfaceEmptyState.Width(m.width - noticePaddingH).Render(
			"📦 No A2UI Surfaces Active\n\n" +
				"Interactive forms, cards, and components generated by the agent will appear here.\n\n" +
				"Type a message in the [1 Chat] tab to interact with the agent.")
		m.surfacesViewport.SetContent(emptyBox)
		return
	}

	var sb strings.Builder
	currentLine := 0
	targetStart := -1
	targetEnd := -1
	targetElemStart := -1
	targetElemEnd := -1

	for _, idx := range surfaces {
		item := m.items[idx]
		itemStr := m.renderSurfaceFeedItem(idx, item)

		if m.focusMode == FocusSurface && m.focusedSurfaceIndex == idx {
			boxLines := strings.Split(strings.TrimSuffix(itemStr, "\n"), "\n")
			targetStart = currentLine
			targetEnd = currentLine + len(boxLines) - 1

			for lineIdx, line := range boxLines {
				if strings.Contains(line, "ACTIVE FOCUS") {
					continue
				}
				if strings.Contains(line, "▎") ||
					strings.Contains(line, "48;2;56;189;248") ||
					strings.Contains(line, "\x1b[7m") ||
					strings.Contains(line, "[7m") ||
					strings.Contains(line, ";7m") {
					absLine := currentLine + lineIdx
					if targetElemStart == -1 {
						targetElemStart = absLine
					}
					targetElemEnd = absLine
				}
			}
		}

		sb.WriteString(itemStr)
		currentLine += strings.Count(itemStr, "\n")
	}

	m.surfacesViewport.SetContent(sb.String())

	if m.focusMode == FocusSurface {
		if targetElemStart != -1 {
			m.ensureLineRangeVisibleInViewport(&m.surfacesViewport, targetElemStart, targetElemEnd)
		} else if targetStart != -1 {
			m.ensureLineRangeVisibleInViewport(&m.surfacesViewport, targetStart, targetEnd)
		}
	}
}

func (m *Model) renderSurfaceFeedItem(idx int, item FeedItem) string {
	isFocused := (m.focusMode == FocusSurface && m.focusedSurfaceIndex == idx)

	surfID := item.SurfaceID
	if surfID == "" {
		surfID = fmt.Sprintf("Surface #%d", idx+1)
	}

	var badge string
	if isFocused {
		badge = m.styles.SurfaceFocusedBadge.Render(fmt.Sprintf("🎮 %s [ACTIVE FOCUS - Tab/Arrows to navigate, Enter to activate]", surfID))
	} else {
		badge = m.styles.SurfaceBadge.Render(fmt.Sprintf("📦 %s [Click or press Tab to focus]", surfID))
	}

	surfaceContent := ""
	if item.Surface != nil {
		surfaceContent = item.Surface.View().Content
	}
	combined := fmt.Sprintf("%s\n\n%s", badge, surfaceContent)

	var containerText string
	if isFocused {
		containerText = m.styles.SurfaceFocusedContainer.Width(m.width - surfaceContainerPad).Render(combined)
	} else {
		containerText = m.styles.SurfaceContainer.Width(m.width - surfaceContainerPad).Render(combined)
	}
	return containerText + "\n"
}

func (m *Model) updateLogsViewportContent() {
	var sb strings.Builder

	for _, item := range m.items {
		timestamp := m.styles.LogTimestamp.Render(item.Timestamp.Format("15:04:05"))

		switch item.Kind {
		case KindSystem:
			prefix := m.styles.LogPrefix.Foreground(lipgloss.Color("#00D7AF")).Render("[SYSTEM]")
			sb.WriteString(fmt.Sprintf("%s %s %s\n", timestamp, prefix, item.Content))

		case KindAction:
			prefix := m.styles.LogPrefix.Foreground(lipgloss.Color("#F9E2AF")).Render("[ACTION]")
			sb.WriteString(fmt.Sprintf("%s %s %s\n", timestamp, prefix, item.Content))

		case KindError:
			prefix := m.styles.LogPrefix.Foreground(lipgloss.Color("#F43F5E")).Render("[ERROR]")
			sb.WriteString(fmt.Sprintf("%s %s %s\n", timestamp, prefix, item.Content))

		case KindDiagnostic:
			if item.Diagnostic != nil {
				d := item.Diagnostic
				var diagLines []string
				diagLines = append(diagLines, m.styles.DiagnosticTitle.Render(fmt.Sprintf("⚠️ DIAGNOSTIC: %s", d.Title)))
				if d.TargetURL != "" {
					diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Target URL:"), m.styles.DiagnosticValue.Render(d.TargetURL)))
				}
				if d.Phase != "" {
					diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Phase:"), m.styles.DiagnosticValue.Render(d.Phase)))
				}
				if d.ErrorMsg != "" {
					diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Error:"), m.styles.DiagnosticValue.Render(d.ErrorMsg)))
				}
				if d.Details != "" {
					diagLines = append(diagLines, fmt.Sprintf("%s %s", m.styles.DiagnosticLabel.Render("Details:"), m.styles.DiagnosticValue.Render(d.Details)))
				}
				if len(d.Tips) > 0 {
					diagLines = append(diagLines, "")
					diagLines = append(diagLines, m.styles.DiagnosticLabel.Render("Troubleshooting Tips:"))
					for _, tip := range d.Tips {
						diagLines = append(diagLines, m.styles.DiagnosticTip.Render("  • "+tip))
					}
				}
				content := strings.Join(diagLines, "\n")
				sb.WriteString(m.styles.DiagnosticBox.Width(m.width-surfaceContainerPad).Render(content) + "\n")
			}
		}
	}

	m.logsViewport.SetContent(sb.String())
}

// ensureLineRangeVisibleInViewport ensures the given line range is visible in the specified viewport.
func (m *Model) ensureLineRangeVisibleInViewport(vp *viewport.Model, startLine, endLine int) {
	if !m.ready || vp.Height() <= 0 {
		return
	}

	vpHeight := vp.Height()
	currOffset := vp.YOffset()
	totalLines := vp.TotalLineCount()
	maxOffset := totalLines - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	if startLine < currOffset {
		newOffset := startLine
		if newOffset > 0 {
			newOffset--
		}
		if newOffset > maxOffset {
			newOffset = maxOffset
		}
		if newOffset < 0 {
			newOffset = 0
		}
		vp.SetYOffset(newOffset)
		return
	}

	if endLine >= currOffset+vpHeight {
		elemHeight := endLine - startLine + 1
		var newOffset int
		if elemHeight <= vpHeight {
			newOffset = endLine - vpHeight + 2
		} else {
			newOffset = startLine
		}
		if newOffset > maxOffset {
			newOffset = maxOffset
		}
		if newOffset < 0 {
			newOffset = 0
		}
		vp.SetYOffset(newOffset)
		return
	}
}

// ensureLineRangeVisible ensures the given vertical line range [startLine, endLine]
// within the active viewport content is visible on screen.
func (m *Model) ensureLineRangeVisible(startLine, endLine int) {
	m.ensureLineRangeVisibleInViewport(m.activeViewport(), startLine, endLine)
}

// findSurfaceAtContentLine returns the index in m.items of the surface rendered at targetLine in the surfaces viewport.
func (m *Model) findSurfaceAtContentLine(targetLine int) int {
	if targetLine < 0 {
		return -1
	}
	surfaces := m.FindSurfaceIndices()
	currentLine := 0
	for _, idx := range surfaces {
		item := m.items[idx]
		itemStr := m.renderSurfaceFeedItem(idx, item)
		lines := strings.Count(itemStr, "\n")
		if targetLine >= currentLine && targetLine < currentLine+lines {
			return idx
		}
		currentLine += lines
	}
	return -1
}

// findItemAtContentLine returns the index of the FeedItem that renders on the given
// 0-indexed line of the active tab's viewport content, or -1 if line is out of range.
func (m *Model) findItemAtContentLine(targetLine int) int {
	if m.activeTab == TabSurfaces {
		return m.findSurfaceAtContentLine(targetLine)
	}
	if targetLine < 0 {
		return -1
	}

	currentLine := 0
	for i, item := range m.items {
		itemStr := m.renderChatFeedItem(i, item)
		lineCount := strings.Count(itemStr, "\n")
		if lineCount > 0 && targetLine >= currentLine && targetLine < currentLine+lineCount {
			return i
		}
		currentLine += lineCount
	}
	return -1
}
