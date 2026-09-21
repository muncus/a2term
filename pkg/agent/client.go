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

package agent

import "context"

// SessionInfo encapsulates active conversation context and task identifiers.
type SessionInfo struct {
	ContextID string
	TaskID    string
}

// Client is the interface implemented by conversational agent backends.
type Client interface {
	// AgentName returns the display name or endpoint of the connected agent.
	AgentName() string

	// CurrentSession returns the active session context and task identifiers.
	CurrentSession() SessionInfo

	// ResetSession clears active conversational and task identifiers.
	ResetSession()

	// SendMessage sends a user message and returns the response text.
	SendMessage(ctx context.Context, text string) (string, error)

	// StreamMessage sends a message and yields streaming chunks as they arrive.
	StreamMessage(ctx context.Context, text string, onChunk func(chunk string, isFinal bool, err error)) error

	// SendActionEvent dispatches an A2UI interaction event (e.g. button click or form submit) back to the agent.
	SendActionEvent(ctx context.Context, actionName string, sourceID string, contextValues map[string]any) (string, error)

	// SendA2UIAction dispatches a full A2UI ActionEvent with optional surfaceID and client data model back to the agent.
	SendA2UIAction(ctx context.Context, actionName string, surfaceID string, sourceID string, contextValues map[string]any, clientDataModel map[string]any) (string, error)
}

