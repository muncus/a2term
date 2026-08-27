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

func TestModelArrowKeyNavigationInSurface(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Add an A2UI surface with 2 buttons
	a2uiMsg := `<a2ui-json>
{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "test-surface",
    "components": [
      { "component": "Card", "id": "root", "child": "col" },
      { "component": "Column", "id": "col", "children": ["btn1", "btn2"] },
      { "component": "Button", "id": "btn1", "action": { "event": { "name": "action1" } }, "child": "t1" },
      { "component": "Text", "id": "t1", "text": "First Button" },
      { "component": "Button", "id": "btn2", "action": { "event": { "name": "action2" } }, "child": "t2" },
      { "component": "Text", "id": "t2", "text": "Second Button" }
    ]
  }
}
</a2ui-json>`

	updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(a2uiMsg))

	// 1. Initially focus is on input. Pressing Up when input is empty enters the surface!
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	// 2. Pressing Down or Right navigates from btn1 to btn2
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	// 3. Pressing Enter activates btn2
	var cmd tea.Cmd
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected command on Enter activating btn2")
	}

	msg := cmd()
	var btnEvent event.ButtonClicked
	if ev, ok := msg.(event.ButtonClicked); ok {
		btnEvent = ev
	} else if batch, ok := msg.(tea.BatchMsg); ok {
		for _, bcmd := range batch {
			if bcmd != nil {
				if ev, ok := bcmd().(event.ButtonClicked); ok {
					btnEvent = ev
					break
				}
			}
		}
	}
	if btnEvent.ID != "btn2" {
		t.Errorf("expected activated button to be btn2, got %q", btnEvent.ID)
	}

	// 4. Pressing Up navigates back to btn1
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg2 := cmd()
		var btnEvent2 event.ButtonClicked
		if ev, ok := msg2.(event.ButtonClicked); ok {
			btnEvent2 = ev
		} else if batch, ok := msg2.(tea.BatchMsg); ok {
			for _, bcmd := range batch {
				if bcmd != nil {
					if ev, ok := bcmd().(event.ButtonClicked); ok {
						btnEvent2 = ev
						break
					}
				}
			}
		}
		if btnEvent2.ID != "btn1" {
			t.Errorf("expected activated button after Up to be btn1, got %q", btnEvent2.ID)
		}
	}

	// 5. Pressing Right navigates to btn2
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg3 := cmd()
		var btnEvent3 event.ButtonClicked
		if ev, ok := msg3.(event.ButtonClicked); ok {
			btnEvent3 = ev
		} else if batch, ok := msg3.(tea.BatchMsg); ok {
			for _, bcmd := range batch {
				if bcmd != nil {
					if ev, ok := bcmd().(event.ButtonClicked); ok {
						btnEvent3 = ev
						break
					}
				}
			}
		}
		if btnEvent3.ID != "btn2" {
			t.Errorf("expected activated button after Right to be btn2, got %q", btnEvent3.ID)
		}
	}

	// 6. Pressing Left navigates back to btn1
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg4 := cmd()
		var btnEvent4 event.ButtonClicked
		if ev, ok := msg4.(event.ButtonClicked); ok {
			btnEvent4 = ev
		} else if batch, ok := msg4.(tea.BatchMsg); ok {
			for _, bcmd := range batch {
				if bcmd != nil {
					if ev, ok := bcmd().(event.ButtonClicked); ok {
						btnEvent4 = ev
						break
					}
				}
			}
		}
		if btnEvent4.ID != "btn1" {
			t.Errorf("expected activated button after Left to be btn1, got %q", btnEvent4.ID)
		}
	}
}

func TestModelEnsureFocusedElementVisibleOnNavigation(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	// Set small height (12 rows total -> viewport height ~5-6 rows)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

	// Add a tall A2UI form with 5 controls spanning multiple lines
	tallForm := `<a2ui-json>
{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "tall-form",
    "components": [
      { "component": "Card", "id": "root", "child": "col" },
      { "component": "Column", "id": "col", "children": ["t1", "input1", "input2", "cb1", "slider1", "btn_submit"] },
      { "component": "Text", "id": "t1", "text": "Header Form Title" },
      { "component": "TextField", "id": "input1", "label": "First Name" },
      { "component": "TextField", "id": "input2", "label": "Last Name" },
      { "component": "CheckBox", "id": "cb1", "label": "Agree to Terms" },
      { "component": "Slider", "id": "slider1", "label": "Rating", "min": 1, "max": 10, "value": 5 },
      { "component": "Button", "id": "btn_submit", "action": { "event": { "name": "submit" } }, "child": "btn_lbl" },
      { "component": "Text", "id": "btn_lbl", "text": "Submit Form" }
    ]
  }
}
</a2ui-json>`

	updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(tallForm))

	// Focus the surface from input (Up key)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	model := updated.(ui.Model)

	if model.FocusMode() != ui.FocusSurface {
		t.Fatalf("expected FocusSurface, got %v", model.FocusMode())
	}

	initialOffset := model.ViewportYOffset()

	// Navigate down step by step to reach the bottom button (5 focusables = 4 steps)
	for i := 0; i < 4; i++ {
		updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	model = updated.(ui.Model)

	// Since the form is taller than the viewport, navigating to the bottom element MUST scroll the viewport down
	if model.ViewportYOffset() <= initialOffset {
		t.Errorf("expected ViewportYOffset to increase when navigating down to bottom element: got %d, initial %d",
			model.ViewportYOffset(), initialOffset)
	}

	bottomOffset := model.ViewportYOffset()

	// Now navigate back up to the top element (4 steps backwards)
	for i := 0; i < 4; i++ {
		updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	model = updated.(ui.Model)

	// Navigating back up to the top element MUST scroll the viewport back up
	if model.ViewportYOffset() >= bottomOffset {
		t.Errorf("expected ViewportYOffset to decrease when navigating back up: got %d, bottom was %d",
			model.ViewportYOffset(), bottomOffset)
	}
}

func TestModelEnsureFocusedSurfaceVisibleOnFocusChange(t *testing.T) {
	m := ui.NewModel(ui.Config{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})

	// Add Surface 1 (at the top)
	surface1 := `<a2ui-json>
{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "surf1",
    "components": [
      { "component": "Card", "id": "root", "child": "b1" },
      { "component": "Button", "id": "b1", "action": { "event": { "name": "act1" } }, "child": "t1" },
      { "component": "Text", "id": "t1", "text": "Surface 1 Top Button" }
    ]
  }
}
</a2ui-json>`
	updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(surface1))

	// Add long messages in between to push Surface 1 far above the viewport
	for i := 1; i <= 15; i++ {
		updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(fmt.Sprintf("Middle message #%d\nLine A\nLine B", i)))
	}

	// Add Surface 2 (at the bottom)
	surface2 := `<a2ui-json>
{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "surf2",
    "components": [
      { "component": "Card", "id": "root", "child": "b2" },
      { "component": "Button", "id": "b2", "action": { "event": { "name": "act2" } }, "child": "t2" },
      { "component": "Text", "id": "t2", "text": "Surface 2 Bottom Button" }
    ]
  }
}
</a2ui-json>`
	updated, _ = updated.Update(ui.NewAgentResponseMsgForTest(surface2))

	// User enters Surface 2 at the bottom
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	model := updated.(ui.Model)
	if model.FocusedSurfaceIndex() != 17 { // Surface 2 is at index 17
		t.Logf("focused surface index: %d", model.FocusedSurfaceIndex())
	}
	bottomYOffset := model.ViewportYOffset()
	if bottomYOffset == 0 {
		t.Fatalf("expected bottomYOffset > 0 with many messages, got %d", bottomYOffset)
	}

	// User switches focus to Surface 1 (earlier in history)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	model = updated.(ui.Model)

	// If focus switched to Surface 1 (or cycled), ensure the viewport scrolled to make it visible
	if model.FocusMode() == ui.FocusSurface && model.FocusedSurfaceIndex() == 0 {
		if model.ViewportYOffset() >= bottomYOffset {
			t.Errorf("expected viewport to scroll up when Surface 1 is focused: got %d, bottom was %d",
				model.ViewportYOffset(), bottomYOffset)
		}
	}

	// Escape returns focus to input and viewport to bottom
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	model = updated.(ui.Model)
	if model.FocusMode() != ui.FocusInput {
		t.Errorf("expected FocusInput on Esc, got %v", model.FocusMode())
	}
	if !model.ViewportAtBottom() {
		t.Errorf("expected viewport at bottom when returning to input")
	}
}


