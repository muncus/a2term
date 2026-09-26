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
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	a2aclient "github.com/muncus/a2term/pkg/a2a"
)

var openBrowserFunc = openBrowser

func openBrowser(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}

func (m *Model) handleUIAction(summary string, sendCmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.items = append(m.items, NewActionItem(uuid.NewString(), summary))
	m.toast = summary
	m.updateViewportContent()
	m.chatViewport.GotoBottom()
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()

	var cmds []tea.Cmd
	cmds = append(cmds, m.clearToastAfter(3*time.Second))

	if sendCmd != nil {
		if m.client != nil {
			m.isLoading = true
			m.status = "Sending action..."
			cmds = append(cmds, sendCmd, m.spinner.Tick)
		} else {
			m.items = append(m.items, NewErrorItem(uuid.NewString(), "Cannot send action: No A2A agent connected. Use /agent <url> or /card <url> to connect."))
			m.updateViewportContent()
			m.chatViewport.GotoBottom()
			m.logsViewport.GotoBottom()
			m.syncActiveViewportMirror()
		}
	}
	return *m, tea.Batch(cmds...)
}

func (m *Model) handleLocalUIAction(summary string, localCmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.items = append(m.items, NewActionItem(uuid.NewString(), summary))
	m.toast = summary
	// Local actions do not wait for agent response, so do not set "Sending action..." status
	m.isLoading = false
	if m.status == "Sending action..." {
		if m.client != nil {
			m.status = "Connected"
		} else {
			m.status = "Ready"
		}
	}
	m.updateViewportContent()
	m.chatViewport.GotoBottom()
	m.logsViewport.GotoBottom()
	m.syncActiveViewportMirror()

	var cmds []tea.Cmd
	cmds = append(cmds, m.clearToastAfter(3*time.Second))
	if localCmd != nil {
		cmds = append(cmds, localCmd)
	}
	return *m, tea.Batch(cmds...)
}

func (m *Model) sendMessageCmd(text string) tea.Cmd {
	client := m.client
	target := m.agentURL
	if target == "" {
		target = m.cardURL
	}
	return func() tea.Msg {
		if client == nil {
			return agentErrorMsg{
				err:    fmt.Errorf("agent client not initialized"),
				target: target,
				phase:  "Pre-flight",
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		parts, err := client.SendMessage(ctx, text)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: target,
				phase:  "SendMessage Execution",
			}
		}
		return agentResponseMsg{parts: parts}
	}
}

func (m *Model) sendActionCmd(actionName, surfaceID, sourceID string, contextValues map[string]any, clientDataModel map[string]any) tea.Cmd {
	client := m.client
	target := m.agentURL
	if target == "" {
		target = m.cardURL
	}
	return func() tea.Msg {
		if client == nil {
			return agentErrorMsg{
				err:    fmt.Errorf("agent client not initialized"),
				target: target,
				phase:  "Pre-flight",
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		parts, err := client.SendA2UIAction(ctx, actionName, surfaceID, sourceID, contextValues, clientDataModel)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: target,
				phase:  fmt.Sprintf("SendA2UIAction (%s)", actionName),
			}
		}
		return agentResponseMsg{parts: parts}
	}
}

func (m *Model) reconnectCmd(opts a2aclient.ClientOptions, targetURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cli, err := a2aclient.NewClient(ctx, opts)
		if err != nil {
			return agentErrorMsg{
				err:    err,
				target: targetURL,
				phase:  "Connection Establishment & Card Resolution",
			}
		}
		return reconnectSuccessMsg{
			client:    cli,
			agentName: cli.AgentName(),
			targetURL: targetURL,
		}
	}
}
