package a2a_test

import (
	"context"
	"encoding/json"
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
