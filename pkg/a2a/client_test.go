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

package a2a_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"

	a2aclient "a2term/pkg/a2a"
)

func TestExtractText(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleAgent,
		a2a.NewTextPart("Hello "),
		a2a.NewTextPart("world!"),
	)

	text := a2aclient.ExtractText(msg)
	if text != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %q", text)
	}

	nilText := a2aclient.ExtractText(nil)
	if nilText != "" {
		t.Errorf("expected empty string for nil message, got %q", nilText)
	}
}

func TestClientOptionsValidation(t *testing.T) {
	ctx := context.Background()
	_, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{})
	if err == nil {
		t.Error("expected error when neither AgentURL nor CardURL is provided")
	}
}

func TestClientWithMockServerJSONRPC(t *testing.T) {
	mux := http.NewServeMux()

	var serverURL string
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name:        "Test JSON-RPC Agent",
			Description: "Mock test agent",
			SupportedInterfaces: []*a2a.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2a.TransportProtocolJSONRPC,
					ProtocolVersion: a2a.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"message": map[string]any{
					"messageId": "resp-1",
					"role":      "ROLE_AGENT",
					"parts": []map[string]any{
						{"text": "Hello from JSON-RPC mock agent!"},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.AgentName() != "Test JSON-RPC Agent" {
		t.Errorf("expected agent name 'Test JSON-RPC Agent', got %q", client.AgentName())
	}

	reply, err := client.SendMessage(ctx, "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if reply != "Hello from JSON-RPC mock agent!" {
		t.Errorf("expected 'Hello from JSON-RPC mock agent!', got %q", reply)
	}
}

func TestClientWithMockServerREST(t *testing.T) {
	mux := http.NewServeMux()

	var serverURL string
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name:        "Test REST Agent",
			Description: "Mock test REST agent",
			SupportedInterfaces: []*a2a.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: a2a.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/message:send", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"message": map[string]any{
				"messageId": "resp-rest-1",
				"role":      "ROLE_AGENT",
				"parts": []map[string]any{
					{"text": "Hello from REST mock agent!"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create REST client: %v", err)
	}

	if client.AgentName() != "Test REST Agent" {
		t.Errorf("expected agent name 'Test REST Agent', got %q", client.AgentName())
	}

	reply, err := client.SendMessage(ctx, "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if reply != "Hello from REST mock agent!" {
		t.Errorf("expected 'Hello from REST mock agent!', got %q", reply)
	}
}

func TestClientWithTaskArtifacts(t *testing.T) {
	mux := http.NewServeMux()

	var serverURL string
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name:        "ADK Artifact Agent",
			Description: "Mock ADK agent with artifact output",
			SupportedInterfaces: []*a2a.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2a.TransportProtocolJSONRPC,
					ProtocolVersion: a2a.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"task": map[string]any{
					"id":        "task-123",
					"contextId": "ctx-456",
					"status": map[string]any{
						"state": "TASK_STATE_COMPLETED",
					},
					"artifacts": []map[string]any{
						{
							"artifactId": "art-1",
							"parts": []map[string]any{
								{"text": "ADK Agent Output via Artifacts"},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	reply, err := client.SendMessage(ctx, "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if reply != "ADK Agent Output via Artifacts" {
		t.Errorf("expected 'ADK Agent Output via Artifacts', got %q", reply)
	}
}

func TestClientOmitTaskIDOnTerminalState(t *testing.T) {
	mux := http.NewServeMux()

	var serverURL string
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name:        "Terminal State Test Agent",
			Description: "Mock agent testing terminal state transitions",
			SupportedInterfaces: []*a2a.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2a.TransportProtocolJSONRPC,
					ProtocolVersion: a2a.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	var step int
	var receivedTaskIDs []string
	var receivedContextIDs []string

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		params, _ := req["params"].(map[string]any)
		msg, _ := params["message"].(map[string]any)
		taskID, _ := msg["taskId"].(string)
		ctxID, _ := msg["contextId"].(string)

		receivedTaskIDs = append(receivedTaskIDs, taskID)
		receivedContextIDs = append(receivedContextIDs, ctxID)

		step++
		var taskState string
		if step == 1 {
			taskState = "TASK_STATE_WORKING"
		} else if step == 2 {
			taskState = "TASK_STATE_COMPLETED"
		} else {
			taskState = "TASK_STATE_COMPLETED"
		}

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"task": map[string]any{
					"id":        fmt.Sprintf("task-%d", step),
					"contextId": "thread-123",
					"status": map[string]any{
						"state": taskState,
					},
					"artifacts": []map[string]any{
						{
							"artifactId": "art-1",
							"parts": []map[string]any{
								{"text": fmt.Sprintf("Response for step %d", step)},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Message 1: Starts task-1 (working)
	_, err = client.SendMessage(ctx, "msg 1")
	if err != nil {
		t.Fatalf("msg 1 failed: %v", err)
	}
	sess1 := client.CurrentSession()
	if sess1.TaskID != "task-1" {
		t.Errorf("expected active taskID 'task-1' after working response, got %q", sess1.TaskID)
	}
	if sess1.ContextID != "thread-123" {
		t.Errorf("expected contextID 'thread-123', got %q", sess1.ContextID)
	}

	// Message 2: Continues task-1, which returns COMPLETED
	_, err = client.SendMessage(ctx, "msg 2")
	if err != nil {
		t.Fatalf("msg 2 failed: %v", err)
	}
	sess2 := client.CurrentSession()
	if sess2.TaskID != "" {
		t.Errorf("expected empty taskID after terminal COMPLETED state, got %q", sess2.TaskID)
	}
	if sess2.ContextID != "thread-123" {
		t.Errorf("expected contextID 'thread-123' to be retained, got %q", sess2.ContextID)
	}

	// Message 3: Next message should omit taskId so server starts fresh task
	_, err = client.SendMessage(ctx, "msg 3")
	if err != nil {
		t.Fatalf("msg 3 failed: %v", err)
	}

	// Verify what server received on message 3:
	if len(receivedTaskIDs) != 3 {
		t.Fatalf("expected 3 requests received, got %d", len(receivedTaskIDs))
	}
	if receivedTaskIDs[0] != "" {
		t.Errorf("step 1: expected empty initial taskId, got %q", receivedTaskIDs[0])
	}
	if receivedTaskIDs[1] != "task-1" {
		t.Errorf("step 2: expected taskId 'task-1', got %q", receivedTaskIDs[1])
	}
	if receivedTaskIDs[2] != "" {
		t.Errorf("step 3: expected omitted/empty taskId for new task, got %q", receivedTaskIDs[2])
	}
	if receivedContextIDs[2] != "thread-123" {
		t.Errorf("step 3: expected contextId 'thread-123' preserved, got %q", receivedContextIDs[2])
	}
}
