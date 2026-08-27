package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the complete UI layout.
func (m Model) View() tea.View {
	if !m.ready {
		v := tea.NewView("Initializing a2term...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	var sections []string

	// 1. Header Bar
	sections = append(sections, m.renderHeader())

	// 2. Main Chat / Surfaces Viewport
	sections = append(sections, m.viewport.View())

	// 3. Action Toast Bar (if present)
	if m.toast != "" {
		sections = append(sections, m.styles.ToastBox.Render(m.toast))
	}

	// 4. Input Prompt Box
	sections = append(sections, m.renderInput())

	// 5. Footer Shortcuts Bar
	sections = append(sections, m.renderFooter())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *Model) renderHeader() string {
	title := m.styles.HeaderTitle.Render("a2term")

	var statusText string
	var statusStyled string
	if m.isLoading || m.isStreaming {
		statusText = fmt.Sprintf("%s %s", m.spinner.View(), m.status)
		statusStyled = m.styles.HeaderStatus.Render(statusText)
	} else if m.status == "Connection Failed" || m.status == "Error" || m.status == "Disconnected" {
		statusText = fmt.Sprintf("● %s", m.status)
		statusStyled = m.styles.HeaderStatusError.Render(statusText)
	} else {
		statusText = fmt.Sprintf("● %s", m.status)
		statusStyled = m.styles.HeaderStatus.Render(statusText)
	}

	agentName := "Agent: None"
	if m.client != nil {
		agentName = fmt.Sprintf("Agent: %s", m.client.AgentName())
	} else if m.agentURL != "" {
		agentName = fmt.Sprintf("Target: %s", m.agentURL)
	}
	agent := m.styles.HeaderAgent.Render(agentName)

	var sessionBadge string
	if m.client != nil {
		ctxID, taskID := m.client.CurrentSession()
		if taskID != "" {
			sessionBadge = m.styles.HeaderTask.Render(fmt.Sprintf("Task: %s", taskID))
		} else if ctxID != "" {
			sessionBadge = m.styles.HeaderTask.Render(fmt.Sprintf("Ctx: %s", ctxID))
		}
	}

	var focusBadge string
	if m.focusMode == FocusSurface {
		focusBadge = m.styles.SurfaceFocusedBadge.Render("🎮 Focus: A2UI Surface (Tab/Arrows/Enter)")
	} else {
		focusBadge = m.styles.SurfaceBadge.Render("💬 Focus: Chat Input")
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, " ", statusStyled, " ", agent)
	right := lipgloss.JoinHorizontal(lipgloss.Center, sessionBadge, " ", focusBadge)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	space := strings.Repeat(" ", gap)

	return m.styles.Header.Width(m.width).Render(left + space + right)
}

func (m *Model) renderInput() string {
	inputView := m.input.View()
	if m.focusMode == FocusInput {
		return m.styles.InputFocusedContainer.Width(m.width - 2).Render(inputView)
	}
	return m.styles.InputContainer.Width(m.width - 2).Render(inputView)
}

func (m *Model) renderFooter() string {
	keys := []struct {
		key  string
		desc string
	}{
		{"Tab", "Surface"},
		{"Enter", "Send"},
		{"PgUp/Dn", "Scroll"},
		{"Ctrl+U/D", "HalfPg"},
		{"/help", "Commands"},
		{"Ctrl+C", "Quit"},
	}

	var keyParts []string
	for _, k := range keys {
		keyParts = append(keyParts, fmt.Sprintf("%s %s", m.styles.FooterKey.Render(k.key), m.styles.FooterDesc.Render(k.desc)))
	}

	left := strings.Join(keyParts, "  ")

	var scrollBadge string
	if m.viewport.AtTop() && m.viewport.AtBottom() {
		scrollBadge = m.styles.ScrollBadge.Render("📜 All")
	} else if m.viewport.AtBottom() {
		scrollBadge = m.styles.ScrollBadge.Render("📜 Bottom")
	} else if m.viewport.AtTop() {
		scrollBadge = m.styles.ScrollAlert.Render("⬆ Top (0%) • [End: Bottom]")
	} else {
		pct := int(m.viewport.ScrollPercent() * 100)
		scrollBadge = m.styles.ScrollAlert.Render(fmt.Sprintf("⬇ %d%% • [End: Bottom]", pct))
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(scrollBadge)
	if gap < 1 {
		gap = 1
	}
	space := strings.Repeat(" ", gap)

	return m.styles.Footer.Width(m.width).Render(left + space + scrollBadge)
}

func (m *Model) updateViewportContent() {
	var sb strings.Builder
	currentLine := 0
	targetStart := -1
	targetEnd := -1
	targetElemStart := -1
	targetElemEnd := -1

	for i, item := range m.items {
		var itemStr string
		switch item.Kind {
		case KindUser:
			role := m.styles.UserRole.Render(fmt.Sprintf("👤 You (%s)", item.Timestamp.Format("15:04")))
			text := m.styles.UserText.Render(item.Content)
			msg := fmt.Sprintf("%s\n%s", role, text)
			itemStr = m.styles.UserMessageBox.Width(m.width - 4).Render(msg) + "\n"

		case KindAgentText:
			role := m.styles.AgentRole.Render(fmt.Sprintf("🤖 Agent (%s)", item.Timestamp.Format("15:04")))
			text := m.styles.AgentText.Render(item.Content)
			msg := fmt.Sprintf("%s\n%s", role, text)
			itemStr = m.styles.AgentMessageBox.Width(m.width - 4).Render(msg) + "\n"

		case KindAgentSurface:
			isFocused := (m.focusMode == FocusSurface && m.focusedSurfaceIndex == i)
			var badge string
			if isFocused {
				badge = m.styles.SurfaceFocusedBadge.Render("🎮 A2UI SURFACE [ACTIVE FOCUS - Tab moves focus, Enter activates]")
			} else {
				badge = m.styles.SurfaceBadge.Render("📦 A2UI SURFACE [Press Tab to interact]")
			}

			surfaceContent := item.Surface.View().Content
			combined := fmt.Sprintf("%s\n\n%s", badge, surfaceContent)

			var containerText string
			if isFocused {
				containerText = m.styles.SurfaceFocusedContainer.Width(m.width - 4).Render(combined)
			} else {
				containerText = m.styles.SurfaceContainer.Width(m.width - 4).Render(combined)
			}
			itemStr = containerText + "\n"

			if isFocused {
				boxLines := strings.Split(containerText, "\n")
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

		case KindSystem:
			itemStr = m.styles.SystemMessage.Render("ℹ️ " + item.Content) + "\n\n"

		case KindAction:
			itemStr = m.styles.ActionMessage.Render(item.Content) + "\n"

		case KindError:
			itemStr = m.styles.ErrorMessage.Render("❌ " + item.Content) + "\n"

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
				itemStr = m.styles.DiagnosticBox.Width(m.width - 4).Render(content) + "\n"
			}
		}

		sb.WriteString(itemStr)
		currentLine += strings.Count(itemStr, "\n")
	}

	m.viewport.SetContent(sb.String())

	if m.focusMode == FocusSurface {
		if targetElemStart != -1 {
			m.ensureLineRangeVisible(targetElemStart, targetElemEnd)
		} else if targetStart != -1 {
			m.ensureLineRangeVisible(targetStart, targetEnd)
		}
	}
}

// ensureLineRangeVisible ensures the given vertical line range [startLine, endLine]
// within viewport content is visible on screen, scrolling the viewport only when necessary.
func (m *Model) ensureLineRangeVisible(startLine, endLine int) {
	if !m.ready || m.viewport.Height() <= 0 {
		return
	}

	vpHeight := m.viewport.Height()
	currOffset := m.viewport.YOffset()
	totalLines := m.viewport.TotalLineCount()
	maxOffset := totalLines - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	// 1. If startLine is above current view, scroll UP so startLine is visible
	if startLine < currOffset {
		newOffset := startLine
		if newOffset > 0 {
			newOffset-- // 1 line of context margin above
		}
		if newOffset > maxOffset {
			newOffset = maxOffset
		}
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
		return
	}

	// 2. If endLine is below current view, scroll DOWN so endLine is visible
	if endLine >= currOffset+vpHeight {
		elemHeight := endLine - startLine + 1
		var newOffset int
		if elemHeight <= vpHeight {
			// If element fits on screen, position it near bottom with 1 line margin below
			newOffset = endLine - vpHeight + 2
		} else {
			// If element is taller than viewport, align top of element to top of viewport
			newOffset = startLine
		}
		if newOffset > maxOffset {
			newOffset = maxOffset
		}
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
		return
	}

	// 3. Otherwise, already visible on screen!
}

