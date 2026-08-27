package ui

import (
	"charm.land/lipgloss/v2"
)

// Styles holds all lipgloss styles used across the TUI.
type Styles struct {
	// App container
	App lipgloss.Style

	// Header styles
	Header            lipgloss.Style
	HeaderTitle       lipgloss.Style
	HeaderStatus      lipgloss.Style
	HeaderStatusError lipgloss.Style
	HeaderAgent       lipgloss.Style
	HeaderTask        lipgloss.Style

	// Chat message styles
	UserMessageBox  lipgloss.Style
	UserRole        lipgloss.Style
	UserText        lipgloss.Style
	AgentMessageBox lipgloss.Style
	AgentRole       lipgloss.Style
	AgentText       lipgloss.Style
	SystemMessage   lipgloss.Style
	ErrorMessage    lipgloss.Style
	ActionMessage   lipgloss.Style

	// Diagnostics & Error box styles
	DiagnosticBox   lipgloss.Style
	DiagnosticTitle lipgloss.Style
	DiagnosticLabel lipgloss.Style
	DiagnosticValue lipgloss.Style
	DiagnosticTip   lipgloss.Style

	// Surface box
	SurfaceContainer        lipgloss.Style
	SurfaceFocusedContainer lipgloss.Style
	SurfaceBadge            lipgloss.Style
	SurfaceFocusedBadge     lipgloss.Style

	// Input styles
	InputContainer        lipgloss.Style
	InputFocusedContainer lipgloss.Style
	InputPrompt           lipgloss.Style

	// Footer and toast
	Footer          lipgloss.Style
	FooterKey       lipgloss.Style
	FooterDesc      lipgloss.Style
	ToastBox        lipgloss.Style
	ScrollBadge     lipgloss.Style
	ScrollAlert     lipgloss.Style
	ScrollIndicator lipgloss.Style
}

// DefaultStyles creates the default modern color scheme and styling.
func DefaultStyles() Styles {
	primary := lipgloss.Color("#7D56F4")      // Purple
	secondary := lipgloss.Color("#00D7AF")    // Teal
	accent := lipgloss.Color("#F43F5E")       // Coral/Rose
	bgDark := lipgloss.Color("#181825")       // Dark background
	textMuted := lipgloss.Color("#A6ADC8")    // Muted text
	textBright := lipgloss.Color("#CDD6F4")   // Bright text
	userBubbleBg := lipgloss.Color("#313244") // Dark slate for user
	agentBubbleBg := lipgloss.Color("#1E1E2E")
	borderNormal := lipgloss.Color("#45475A")
	borderFocused := lipgloss.Color("#38BDF8")

	return Styles{
		App: lipgloss.NewStyle().
			Padding(0, 0),

		Header: lipgloss.NewStyle().
			Background(bgDark).
			Foreground(textBright).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(borderNormal),

		HeaderTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primary).
			Padding(0, 1),

		HeaderStatus: lipgloss.NewStyle().
			Foreground(secondary).
			Bold(true),

		HeaderStatusError: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),

		HeaderAgent: lipgloss.NewStyle().
			Foreground(textMuted),

		HeaderTask: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9E2AF")).
			Italic(true),

		UserMessageBox: lipgloss.NewStyle().
			Background(userBubbleBg).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary),

		UserRole: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary),

		UserText: lipgloss.NewStyle().
			Foreground(textBright),

		AgentMessageBox: lipgloss.NewStyle().
			Background(agentBubbleBg).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondary),

		AgentRole: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary),

		AgentText: lipgloss.NewStyle().
			Foreground(textBright),

		SystemMessage: lipgloss.NewStyle().
			Foreground(textMuted).
			Italic(true).
			Padding(0, 1),

		ErrorMessage: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true).
			Padding(0, 1),

		ActionMessage: lipgloss.NewStyle().
			Foreground(secondary).
			Italic(true).
			Padding(0, 1),

		DiagnosticBox: lipgloss.NewStyle().
			Background(lipgloss.Color("#2A1E24")).
			Border(lipgloss.ThickBorder()).
			BorderForeground(accent).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1),

		DiagnosticTitle: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),

		DiagnosticLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9E2AF")).
			Bold(true),

		DiagnosticValue: lipgloss.NewStyle().
			Foreground(textBright),

		DiagnosticTip: lipgloss.NewStyle().
			Foreground(textMuted),

		SurfaceContainer: lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderNormal),

		SurfaceFocusedContainer: lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1).
			Padding(0, 1).
			Border(lipgloss.ThickBorder()).
			BorderForeground(borderFocused),

		SurfaceBadge: lipgloss.NewStyle().
			Foreground(textMuted).
			Background(lipgloss.Color("#313244")).
			Padding(0, 1),

		SurfaceFocusedBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(borderFocused).
			Bold(true).
			Padding(0, 1),

		InputContainer: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderNormal).
			Padding(0, 1),

		InputFocusedContainer: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondary).
			Padding(0, 1),

		InputPrompt: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary),

		Footer: lipgloss.NewStyle().
			Background(bgDark).
			Foreground(textMuted).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(borderNormal),

		FooterKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#45475A")).
			Padding(0, 1).
			Bold(true),

		FooterDesc: lipgloss.NewStyle().
			Foreground(textMuted),

		ToastBox: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#059669")).
			Bold(true).
			Padding(0, 2),

		ScrollBadge: lipgloss.NewStyle().
			Foreground(textMuted).
			Italic(true),

		ScrollAlert: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#181825")).
			Background(lipgloss.Color("#F9E2AF")).
			Bold(true).
			Padding(0, 1),

		ScrollIndicator: lipgloss.NewStyle().
			Foreground(secondary).
			Bold(true),
	}
}
