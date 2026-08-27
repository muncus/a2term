package ui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/joestump-agent/a2tea/event"
	tmca2ui "github.com/tmc/a2ui"

	"a2term/pkg/ui"
)

func TestModelInit(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil initial cmd")
	}

	view := m.View()
	if !strings.Contains(view.Content, "Initializing a2term") {
		t.Errorf("expected view to contain initializing before window size, got %q", view.Content)
	}
}

func TestModelWindowSize(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	view := updated.View()

	if !strings.Contains(view.Content, "a2term") {
		t.Errorf("expected view to render title 'a2term', got %q", view.Content)
	}
	if !strings.Contains(view.Content, "Welcome to a2term") {
		t.Errorf("expected view to render welcome message, got %q", view.Content)
	}
}

func TestModelSlashCommands(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Enter /help
	for _, ch := range "/help" {
		updated, _ = updated.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := updated.View()
	if !strings.Contains(view.Content, "a2term Commands & Help") {
		t.Errorf("expected /help output in view, got %q", view.Content)
	}

	// Enter /clear
	for _, ch := range "/clear" {
		updated, _ = updated.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view = updated.View()
	if !strings.Contains(view.Content, "Conversation history cleared") {
		t.Errorf("expected /clear confirmation in view, got %q", view.Content)
	}
}

func TestModelInteractionEvents(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Simulate a button click event
	btnEvent := event.ButtonClicked{
		Source: event.Source{ComponentID: "submitBtn"},
		Action: &tmca2ui.EventAction{Name: "submitForm"},
	}
	updated, _ = updated.Update(btnEvent)

	view := updated.View()
	if !strings.Contains(view.Content, "submitForm") {
		t.Errorf("expected button action to appear in view, got %q", view.Content)
	}

	// Simulate client message
	clientMsg := tmca2ui.ClientMessage{
		Action: &tmca2ui.ActionEvent{
			Name:              "confirmAction",
			SourceComponentID: "confirmBtn",
			Context:           map[string]any{"confirmed": true},
		},
	}
	updated, _ = updated.Update(clientMsg)

	view = updated.View()
	if !strings.Contains(view.Content, "confirmAction") {
		t.Errorf("expected client message action to appear in view, got %q", view.Content)
	}
}

func TestModelDiagnosticsOnStartup(t *testing.T) {
	initErr := errors.New("no compatible transports found: available transports - []")
	m := ui.NewModel(ui.Config{
		AgentURL:   "http://localhost:9001",
		InitialErr: initErr,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	view := updated.View()

	if !strings.Contains(view.Content, "DIAGNOSTIC: Agent Connection Error") {
		t.Errorf("expected diagnostic title in view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "http://localhost:9001") {
		t.Errorf("expected target URL in diagnostic view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "no compatible transports found") {
		t.Errorf("expected error message in diagnostic view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "Troubleshooting Tips") {
		t.Errorf("expected troubleshooting tips in diagnostic view, got %q", view.Content)
	}
}

func TestModelPlainTextResponses(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Send plain text message from agent (no A2UI tags)
	plainText := "I am a helpful assistant. Here is a list of features:\n- Fast\n- Reliable\n- Terminal-native"
	updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(plainText))

	view := updated.View()
	if !strings.Contains(view.Content, "I am a helpful assistant") {
		t.Errorf("expected plain text response in view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "Terminal-native") {
		t.Errorf("expected bullet list in view, got %q", view.Content)
	}
}

func TestModelStreamingPlainText(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Stream chunk 1
	updated, _ = updated.Update(ui.NewAgentStreamChunkMsgForTest("Thinking about your query...", false))
	view1 := updated.View()
	if !strings.Contains(view1.Content, "Thinking about your query...") {
		t.Errorf("expected intermediate stream chunk in view, got %q", view1.Content)
	}

	// Stream final chunk
	finalText := "The answer is 42."
	updated, _ = updated.Update(ui.NewAgentStreamChunkMsgForTest(finalText, true))
	viewFinal := updated.View()
	if !strings.Contains(viewFinal.Content, "The answer is 42.") {
		t.Errorf("expected final stream chunk in view, got %q", viewFinal.Content)
	}
}

func TestModelScrollbackKeyBindings(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	// Set small height so content exceeds viewport height
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})

	// Add multiple long messages to create scrollback
	for i := 1; i <= 20; i++ {
		longMsg := fmt.Sprintf("Message #%d:\nLine A\nLine B\nLine C\nLine D\nLine E", i)
		updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(longMsg))
	}

	model, ok := updated.(ui.Model)
	if !ok {
		t.Fatalf("expected ui.Model type")
	}

	// Initially at bottom after messages
	if !model.ViewportAtBottom() {
		t.Errorf("expected viewport to be at bottom initially")
	}
	initialOffset := model.ViewportYOffset()
	if initialOffset <= 0 {
		t.Fatalf("expected initial offset > 0, got %d", initialOffset)
	}

	// Test PgUp / PageUp (scrolls up half page)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	model = updated.(ui.Model)
	if model.ViewportYOffset() >= initialOffset {
		t.Errorf("expected YOffset to decrease after PgUp: got %d, want < %d", model.ViewportYOffset(), initialOffset)
	}
	pgUpOffset := model.ViewportYOffset()

	// Test Ctrl+U (scrolls up half page)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	model = updated.(ui.Model)
	if model.ViewportYOffset() >= pgUpOffset {
		t.Errorf("expected YOffset to decrease after Ctrl+U: got %d, want < %d", model.ViewportYOffset(), pgUpOffset)
	}

	// Test Shift+Up (scrolls up 3 lines)
	currOffset := model.ViewportYOffset()
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	model = updated.(ui.Model)
	if model.ViewportYOffset() != currOffset-3 && model.ViewportYOffset() >= currOffset {
		t.Errorf("expected YOffset to decrease after Shift+Up: got %d", model.ViewportYOffset())
	}

	// Test Ctrl+Home (jumps to top)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyHome, Mod: tea.ModCtrl})
	model = updated.(ui.Model)
	if model.ViewportYOffset() != 0 {
		t.Errorf("expected YOffset to be 0 after Ctrl+Home, got %d", model.ViewportYOffset())
	}
	if !model.ViewportAtTop() {
		t.Errorf("expected ViewportAtTop() to be true")
	}

	// Test View rendering while scrolled up shows scroll alert
	view := updated.View()
	if !strings.Contains(view.Content, "Top (0%)") && !strings.Contains(view.Content, "End: Bottom") {
		t.Errorf("expected scroll alert in footer when scrolled to top, got:\n%s", view.Content)
	}

	// Test Ctrl+End (jumps to bottom)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModCtrl})
	model = updated.(ui.Model)
	if !model.ViewportAtBottom() {
		t.Errorf("expected ViewportAtBottom() to be true after Ctrl+End")
	}

	// Test Ctrl+D (scrolls down half page after scrolling up)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyHome, Mod: tea.ModCtrl})
	updated, _ = updated.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	model = updated.(ui.Model)
	if model.ViewportYOffset() == 0 {
		t.Errorf("expected YOffset > 0 after Ctrl+D from top")
	}

	// Test PgDn (scrolls down half page)
	afterCtrlD := model.ViewportYOffset()
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	model = updated.(ui.Model)
	if model.ViewportYOffset() <= afterCtrlD {
		t.Errorf("expected YOffset > %d after PgDn, got %d", afterCtrlD, model.ViewportYOffset())
	}
}

func TestModelMouseWheelScrollback(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})

	for i := 1; i <= 20; i++ {
		longMsg := fmt.Sprintf("Message #%d: some content line 1\nsome content line 2", i)
		updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(longMsg))
	}

	model := updated.(ui.Model)
	bottomOffset := model.ViewportYOffset()

	// Scroll mouse wheel up
	updated, _ = updated.Update(tea.MouseWheelMsg{
		Button: tea.MouseWheelUp,
		X:      10,
		Y:      5,
	})
	model = updated.(ui.Model)
	if model.ViewportYOffset() >= bottomOffset {
		t.Errorf("expected YOffset to decrease on MouseWheelUp: got %d, initial %d", model.ViewportYOffset(), bottomOffset)
	}

	scrolledUpOffset := model.ViewportYOffset()

	// Scroll mouse wheel down
	updated, _ = updated.Update(tea.MouseWheelMsg{
		Button: tea.MouseWheelDown,
		X:      10,
		Y:      5,
	})
	model = updated.(ui.Model)
	if model.ViewportYOffset() <= scrolledUpOffset {
		t.Errorf("expected YOffset to increase on MouseWheelDown: got %d, previous %d", model.ViewportYOffset(), scrolledUpOffset)
	}
}

func TestModelStickyScrollbackDuringUpdates(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})

	for i := 1; i <= 15; i++ {
		updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(fmt.Sprintf("Old Message %d\nLine 1\nLine 2", i)))
	}

	// User scrolls to top to inspect history
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyHome, Mod: tea.ModCtrl})
	model := updated.(ui.Model)
	if model.ViewportYOffset() != 0 {
		t.Fatalf("expected offset 0 at top, got %d", model.ViewportYOffset())
	}

	// New streaming chunk arrives while user is reading top scrollback
	updated, _ = updated.Update(ui.NewAgentStreamChunkMsgForTest("New incoming message chunk...", false))
	model = updated.(ui.Model)

	// Scroll position MUST NOT be forcefully moved to bottom
	if model.ViewportYOffset() != 0 {
		t.Errorf("expected scroll position to remain sticky at 0 while reading history, got %d", model.ViewportYOffset())
	}

	// Final stream chunk arrives
	updated, _ = updated.Update(ui.NewAgentStreamChunkMsgForTest("New incoming message final.", true))
	model = updated.(ui.Model)
	if model.ViewportYOffset() != 0 {
		t.Errorf("expected scroll position to still remain at 0 after final chunk, got %d", model.ViewportYOffset())
	}

	// User jumps to bottom
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModCtrl})
	model = updated.(ui.Model)
	if !model.ViewportAtBottom() {
		t.Errorf("expected viewport to be at bottom after Ctrl+End")
	}

	// Now that user is at bottom, new message should follow at bottom
	updated, _ = updated.Update(ui.NewAgentResponseMsgForTest("Another message at bottom"))
	model = updated.(ui.Model)
	if !model.ViewportAtBottom() {
		t.Errorf("expected viewport to remain at bottom when user was at bottom")
	}
}
