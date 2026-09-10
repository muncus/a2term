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

package agent_test

import (
	"context"
	"testing"

	"github.com/muncus/a2term/pkg/agent"
)

// mockClient verifies interface compliance at compile-time.
type mockClient struct {
	name      string
	session   agent.SessionInfo
	resetHits int
}

var _ agent.Client = (*mockClient)(nil)

func (m *mockClient) AgentName() string {
	return m.name
}

func (m *mockClient) CurrentSession() agent.SessionInfo {
	return m.session
}

func (m *mockClient) ResetSession() {
	m.resetHits++
	m.session = agent.SessionInfo{}
}

func (m *mockClient) SendMessage(ctx context.Context, text string) (string, error) {
	return "response to: " + text, nil
}

func (m *mockClient) StreamMessage(ctx context.Context, text string, onChunk func(chunk string, isFinal bool, err error)) error {
	onChunk("chunk1", false, nil)
	onChunk("chunk2", true, nil)
	return nil
}

func (m *mockClient) SendActionEvent(ctx context.Context, actionName string, sourceID string, contextValues map[string]any) (string, error) {
	return "action response: " + actionName, nil
}

func TestClientInterface(t *testing.T) {
	var cli agent.Client = &mockClient{
		name: "TestAgent",
		session: agent.SessionInfo{
			ContextID: "ctx-123",
			TaskID:    "task-456",
		},
	}

	if cli.AgentName() != "TestAgent" {
		t.Errorf("AgentName() = %q, want %q", cli.AgentName(), "TestAgent")
	}

	sess := cli.CurrentSession()
	if sess.ContextID != "ctx-123" || sess.TaskID != "task-456" {
		t.Errorf("CurrentSession() = %+v, want ctx-123 / task-456", sess)
	}

	ctx := context.Background()
	resp, err := cli.SendMessage(ctx, "hello")
	if err != nil || resp != "response to: hello" {
		t.Errorf("SendMessage() = %q, %v; want %q, nil", resp, err, "response to: hello")
	}

	actResp, err := cli.SendActionEvent(ctx, "click", "btn1", nil)
	if err != nil || actResp != "action response: click" {
		t.Errorf("SendActionEvent() = %q, %v; want %q, nil", actResp, err, "action response: click")
	}

	cli.ResetSession()
	if sess := cli.CurrentSession(); sess.ContextID != "" || sess.TaskID != "" {
		t.Errorf("ResetSession() did not clear session: %+v", sess)
	}
}
