# a2term

`a2term` is a terminal client built in Go that communicates with AI agents over the [A2A (Agent-to-Agent) Protocol](https://github.com/a2aproject/a2a-go) and renders interactive [A2UI](https://github.com/tmc/a2ui) elements directly in the terminal using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [a2tea](https://github.com/joestump-agent/a2tea).

## Features

- **A2A Protocol Communication**: Connects to remote A2A agents via Agent Card resolution (`.well-known/agent-card.json`) or direct JSON-RPC endpoints.
- **Embedded A2UI Rendering**: Scans agent responses for `<a2ui-json>` blocks and renders interactive UI surfaces (cards, buttons, text fields, checkboxes, sliders, choice pickers, tabs, modals).
- **Full Keyboard Navigation & Focus**:
  - `Tab`: Switch focus to active A2UI surfaces and cycle through interactive controls.
  - `Shift+Tab` or `Ctrl+F`: Return focus to the chat input prompt.
  - `Esc`: Dismiss modals or return focus to chat input.
  - `Enter`: Send message or activate focused button / submit form.
  - `PgUp` / `PgDn`: Scroll chat message viewport.
- **Bidirectional Event Dispatch**: User interactions on A2UI components (e.g. clicking a button, choosing options, submitting inputs) are automatically captured and sent back to the agent as action events with context values over A2A.
- **Modern Terminal UI**: Built with Bubble Tea v2, Bubbles v2, and Lipgloss v2.

## Installation & Build

Ensure you have Go 1.24+ installed.

```bash
# Clone and build
git clone https://github.com/joestump-agent/a2term.git
cd a2term
go build -o a2term ./cmd/a2term
```

## Usage

### Connecting to an A2A Agent

Connect using the agent's base URL (a2term will resolve the Agent Card at `/.well-known/agent-card.json`):

```bash
./a2term --agent http://localhost:8080
```

Or connect directly with a specific Agent Card URL:

```bash
./a2term --card http://localhost:8080/.well-known/agent-card.json
```

Supply a bearer token for authentication:

```bash
./a2term --agent http://localhost:8080 --auth secret-token
```

You can also set environment variables:

```bash
export A2A_AGENT_URL=http://localhost:8080
export A2A_AUTH_TOKEN=secret-token
./a2term
```

### Slash Commands

While in the chat interface, you can run commands:

- `/help`: Display commands and keybinding shortcuts.
- `/clear`: Clear conversation history.
- `/reset`: Reset current task and context IDs.
- `/agent <url>`: Connect or switch to an agent endpoint.
- `/card <url>`: Resolve and connect using an Agent Card URL.
- `/auth <token>`: Set or update bearer authorization token.
- `/quit` or `/exit`: Exit the application.

## Keybindings

| Key | Description |
| --- | --- |
| `Tab` | Focus interactive A2UI surface / cycle controls |
| `Shift+Tab` | Return focus to chat input |
| `Ctrl+F` | Toggle focus between input and surface |
| `Esc` | Close open modal / return focus to input |
| `Enter` | Send message / activate focused button |
| `PgUp` / `PgDn` | Scroll viewport half page up / down |
| `Ctrl+U` / `Ctrl+D` | Scroll viewport half page up / down |
| `Shift+Up` / `Shift+Down` | Scroll viewport 3 lines up / down |
| `Ctrl+Home` / `Ctrl+End` | Jump to top / bottom of scrollback history |
| `Home` / `End` | Jump to top / bottom (when input is empty or surface focused) |
| `Mouse Wheel` | Smoothly scroll viewport up / down |
| `Ctrl+C` | Quit application |

## Architecture

- [`pkg/a2a`](file:///Users/muncus/projects/a2term/pkg/a2a): A2A client wrapper managing agent card discovery, sessions (`TaskID`, `ContextID`), message sending, streaming, and action event payloads.
- [`pkg/a2ui`](file:///Users/muncus/projects/a2term/pkg/a2ui): A2UI scanning, rendering bridge (`a2tea`), styling, and event extraction.
- [`pkg/ui`](file:///Users/muncus/projects/a2term/pkg/ui): Bubble Tea v2 application model, views, keyboard routing, and viewport management.
- [`cmd/a2term`](file:///Users/muncus/projects/a2term/cmd/a2term): Main entry point with CLI flag parsing.

## Testing

Run tests across all packages:

```bash
go test -v ./...
```
