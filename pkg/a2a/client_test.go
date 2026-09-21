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
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"

	a2aclient "github.com/muncus/a2term/pkg/a2a"
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

func TestFormatAuthHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"   ", ""},
		{"secret123", "Bearer secret123"},
		{"  secret123  ", "Bearer secret123"},
		{"my-token-xyz", "Bearer my-token-xyz"},
		{"ghp_abc123456789", "Bearer ghp_abc123456789"},
	}

	for _, tt := range tests {
		got := a2aclient.FormatAuthHeader(tt.input)
		if got != tt.expected {
			t.Errorf("FormatAuthHeader(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestClientAuthHeaderJSONRPC(t *testing.T) {
	var currentExpectedToken = "Bearer secret-token-xyz"
	var cardAuthHeader, callAuthHeader string

	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		cardAuthHeader = r.Header.Get("Authorization")
		if cardAuthHeader != currentExpectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}
		card := a2a.AgentCard{
			Name:        "Auth Agent",
			Description: "Mock agent requiring auth",
			SupportedInterfaces: []*a2a.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2a.TransportProtocolJSONRPC,
					ProtocolVersion: a2a.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		callAuthHeader = r.Header.Get("Authorization")
		if callAuthHeader != currentExpectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"message": map[string]any{
					"messageId": "resp-auth-1",
					"role":      "ROLE_AGENT",
					"parts": []map[string]any{
						{"text": "Authorized response!"},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()

	// 1. Test connecting with CardURL without auth token - should fail because card endpoint returns 401
	_, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		CardURL: serverURL + "/.well-known/agent-card.json",
	})
	if err == nil {
		t.Fatal("expected connection without auth token to fail on 401")
	}

	// 2. Test connecting with token value
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL:  serverURL,
		AuthToken: "secret-token-xyz",
	})
	if err != nil {
		t.Fatalf("failed to connect with auth token: %v", err)
	}

	if cardAuthHeader != "Bearer secret-token-xyz" {
		t.Errorf("card endpoint received header %q, want %q", cardAuthHeader, "Bearer secret-token-xyz")
	}

	if client.AuthToken() != "secret-token-xyz" {
		t.Errorf("client.AuthToken() = %q, want %q", client.AuthToken(), "secret-token-xyz")
	}

	resp, err := client.SendMessage(ctx, "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if resp != "Authorized response!" {
		t.Errorf("expected 'Authorized response!', got %q", resp)
	}
	if callAuthHeader != "Bearer secret-token-xyz" {
		t.Errorf("call endpoint received header %q, want %q", callAuthHeader, "Bearer secret-token-xyz")
	}

	// 3. Test dynamic update of AuthToken
	currentExpectedToken = "Bearer new-token-123"
	client.SetAuthToken("new-token-123")
	if client.AuthToken() != "new-token-123" {
		t.Errorf("expected AuthToken to be updated to 'new-token-123', got %q", client.AuthToken())
	}

	resp, err = client.SendMessage(ctx, "hello again")
	if err != nil {
		t.Fatalf("SendMessage with updated token failed: %v", err)
	}
	if callAuthHeader != "Bearer new-token-123" {
		t.Errorf("call endpoint received updated header %q, want %q", callAuthHeader, "Bearer new-token-123")
	}
}

func TestClientAuthHeaderREST(t *testing.T) {
	const expectedToken = "Bearer rest-token-456"
	var receivedAuthHeader string

	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name: "REST Auth Agent",
			SupportedInterfaces: []*a2a.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: a2a.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/message:send", func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		if receivedAuthHeader != expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := map[string]any{
			"message": map[string]any{
				"messageId": "resp-rest-auth-1",
				"role":      "ROLE_AGENT",
				"parts": []map[string]any{
					{"text": "REST response!"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL:  serverURL,
		AuthToken: "rest-token-456",
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	resp, err := client.SendMessage(ctx, "hello REST")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if resp != "REST response!" {
		t.Errorf("expected 'REST response!', got %q", resp)
	}
	if receivedAuthHeader != expectedToken {
		t.Errorf("expected header %q, got %q", expectedToken, receivedAuthHeader)
	}
}

func TestIsA2UIPart(t *testing.T) {
	if a2aclient.IsA2UIPart(nil) {
		t.Error("expected nil part to return false")
	}

	plainPart := a2a.NewTextPart("hello")
	if a2aclient.IsA2UIPart(plainPart) {
		t.Error("expected text/plain part to return false")
	}

	partStandard := a2a.NewDataPart(map[string]any{"updateComponents": map[string]any{}})
	partStandard.MediaType = a2aclient.A2UIMIMEType
	if !a2aclient.IsA2UIPart(partStandard) {
		t.Errorf("expected MediaType %s to be recognized as A2UI part", a2aclient.A2UIMIMEType)
	}

	partLegacy := a2a.NewDataPart(map[string]any{"updateComponents": map[string]any{}})
	partLegacy.MediaType = a2aclient.A2UIMIMETypeLegacy
	if !a2aclient.IsA2UIPart(partLegacy) {
		t.Errorf("expected MediaType %s to be recognized as A2UI part", a2aclient.A2UIMIMETypeLegacy)
	}

	partMeta := a2a.NewDataPart(map[string]any{"updateComponents": map[string]any{}})
	partMeta.Metadata = map[string]any{"mimeType": a2aclient.A2UIMIMEType}
	if !a2aclient.IsA2UIPart(partMeta) {
		t.Errorf("expected metadata mimeType %s to be recognized as A2UI part", a2aclient.A2UIMIMEType)
	}
}

func TestExtractTextA2UIPart(t *testing.T) {
	uiPayload := map[string]any{
		"updateComponents": map[string]any{
			"surfaceId": "main",
			"components": []any{
				map[string]any{"id": "root", "component": "Text", "text": map[string]any{"literal": "Hello A2UI"}},
			},
		},
	}
	part := a2a.NewDataPart(uiPayload)
	part.MediaType = a2aclient.A2UIMIMEType

	msg := a2a.NewMessage(a2a.MessageRoleAgent,
		a2a.NewTextPart("Welcome"),
		part,
	)

	text := a2aclient.ExtractText(msg)
	if !strings.Contains(text, "Welcome") {
		t.Errorf("expected text to contain 'Welcome', got %q", text)
	}
	if !strings.Contains(text, "<a2ui-json>") || !strings.Contains(text, "</a2ui-json>") {
		t.Errorf("expected A2UI data part to be wrapped in <a2ui-json> tags, got %q", text)
	}
	if !strings.Contains(text, "Hello A2UI") {
		t.Errorf("expected payload content in extracted text, got %q", text)
	}
}

func TestA2UIClientCapabilitiesAndActionMetadata(t *testing.T) {
	mux := http.NewServeMux()
	var serverURL string
	var lastReceivedMessage *a2a.Message

	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name: "Capabilities Agent",
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

		// Parse the message sent by client
		if params, ok := req["params"].(map[string]any); ok {
			if msgMap, ok := params["message"].(map[string]any); ok {
				msgBytes, _ := json.Marshal(msgMap)
				var msg a2a.Message
				_ = json.Unmarshal(msgBytes, &msg)
				lastReceivedMessage = &msg
			}
		}

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"message": map[string]any{
					"messageId": "resp-1",
					"role":      "ROLE_AGENT",
					"parts": []map[string]any{
						{"text": "OK"},
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
		AgentURL: serverURL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 1. Test SendMessage attaches capabilities
	_, err = client.SendMessage(ctx, "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if lastReceivedMessage == nil {
		t.Fatal("expected message to be received by mock server")
	}
	caps, ok := lastReceivedMessage.Metadata[a2aclient.ClientCapabilitiesKey].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata %q to be present, got: %v", a2aclient.ClientCapabilitiesKey, lastReceivedMessage.Metadata)
	}
	v091, ok := caps["v0.9.1"].(map[string]any)
	if !ok {
		t.Fatalf("expected v0.9.1 capabilities map, got: %v", caps)
	}
	catalogs, ok := v091["supportedCatalogIds"].([]any)
	if !ok || len(catalogs) == 0 || catalogs[0] != a2aclient.A2UIBasicCatalogID {
		t.Errorf("expected basic catalog ID in capabilities, got: %v", catalogs)
	}

	// 2. Test SendA2UIAction with surfaceId and clientDataModel
	testDataModel := map[string]any{
		"version": "v0.9",
		"surfaces": map[string]any{
			"surface-123": map[string]any{"city": "Paris"},
		},
	}
	_, err = client.SendA2UIAction(ctx, "submit_form", "surface-123", "btn_submit", map[string]any{"input": "val"}, testDataModel)
	if err != nil {
		t.Fatalf("SendA2UIAction failed: %v", err)
	}
	if lastReceivedMessage == nil {
		t.Fatal("expected action message to be received")
	}

	// Verify client data model was attached to metadata
	dataModelMeta, ok := lastReceivedMessage.Metadata[a2aclient.ClientDataModelKey].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata %q to be present, got: %v", a2aclient.ClientDataModelKey, lastReceivedMessage.Metadata)
	}
	if dataModelMeta["version"] != "v0.9" {
		t.Errorf("expected version v0.9 in data model meta, got: %v", dataModelMeta["version"])
	}

	// Verify DataPart had application/a2ui+json MIME type and v0.9 ActionEvent structure
	var foundA2UIPart bool
	for _, part := range lastReceivedMessage.Parts {
		if part.MediaType == a2aclient.A2UIMIMEType {
			foundA2UIPart = true
			if dataMap, ok := part.Data().(map[string]any); ok {
				if actMap, ok := dataMap["action"].(map[string]any); ok {
					if actMap["name"] != "submit_form" {
						t.Errorf("expected action name 'submit_form', got %v", actMap["name"])
					}
					if actMap["surfaceId"] != "surface-123" {
						t.Errorf("expected surfaceId 'surface-123', got %v", actMap["surfaceId"])
					}
					if actMap["sourceComponentId"] != "btn_submit" {
						t.Errorf("expected sourceComponentId 'btn_submit', got %v", actMap["sourceComponentId"])
					}
					if actMap["timestamp"] == "" {
						t.Error("expected non-empty timestamp in action payload")
					}
				} else {
					t.Errorf("expected action map in data part, got %v", dataMap)
				}
			}
		}
	}
	if !foundA2UIPart {
		t.Errorf("expected part with MediaType %s in action message", a2aclient.A2UIMIMEType)
	}
}


