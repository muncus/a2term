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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	a2aspec "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/muncus/a2term/pkg/a2a"
)

func TestAuthFlagParsing(t *testing.T) {
	fs := flag.NewFlagSet("a2term", flag.ContinueOnError)
	agentFlag := fs.String("agent", "", "URL of the A2A agent endpoint")
	cardFlag := fs.String("card", "", "URL of the A2A Agent Card")
	authFlag := fs.String("auth", "", "Bearer token for authorization header")

	args := []string{"--agent", "http://localhost:8080", "--auth", "secret-test-token"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if *agentFlag != "http://localhost:8080" {
		t.Errorf("expected agentFlag %q, got %q", "http://localhost:8080", *agentFlag)
	}
	if *cardFlag != "" {
		t.Errorf("expected empty cardFlag, got %q", *cardFlag)
	}
	if *authFlag != "secret-test-token" {
		t.Errorf("expected authFlag %q, got %q", "secret-test-token", *authFlag)
	}
}

func TestAuthEnvFallback(t *testing.T) {
	const testEnvVar = "A2A_AUTH_TOKEN"
	origVal := os.Getenv(testEnvVar)
	defer os.Setenv(testEnvVar, origVal)

	os.Setenv(testEnvVar, "env-token-999")

	fs := flag.NewFlagSet("a2term", flag.ContinueOnError)
	authFlag := fs.String("auth", "", "Bearer token for authorization header")

	// No --auth flag passed
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	authToken := *authFlag
	if authToken == "" {
		authToken = os.Getenv(testEnvVar)
	}

	if authToken != "env-token-999" {
		t.Errorf("expected authToken from env %q, got %q", "env-token-999", authToken)
	}

	// Flag takes precedence over env
	if err := fs.Parse([]string{"--auth", "flag-token-111"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	authToken = *authFlag
	if authToken == "" {
		authToken = os.Getenv(testEnvVar)
	}
	if authToken != "flag-token-111" {
		t.Errorf("expected authToken from flag %q, got %q", "flag-token-111", authToken)
	}
}

func TestAuthFlagSendsAuthorizationHeader(t *testing.T) {
	const expectedToken = "secret-val-abc"
	const expectedHeader = "Bearer secret-val-abc"

	var receivedCardAuth, receivedCallAuth string

	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		receivedCardAuth = r.Header.Get("Authorization")
		card := a2aspec.AgentCard{
			Name: "Flag Auth Agent",
			SupportedInterfaces: []*a2aspec.AgentInterface{
				{
					URL:             serverURL,
					ProtocolBinding: a2aspec.TransportProtocolJSONRPC,
					ProtocolVersion: a2aspec.Version,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		receivedCallAuth = r.Header.Get("Authorization")
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
						{"text": "Response from authorized server"},
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := a2a.NewClient(ctx, a2a.ClientOptions{
		AgentURL:  serverURL,
		AuthToken: expectedToken,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if receivedCardAuth != expectedHeader {
		t.Errorf("agent card resolution received header %q, want %q", receivedCardAuth, expectedHeader)
	}

	resp, err := cli.SendMessage(ctx, "test query")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if text := a2a.ExtractPartsText(resp); text != "Response from authorized server" {
		t.Errorf("expected 'Response from authorized server', got %q", text)
	}
	if receivedCallAuth != expectedHeader {
		t.Errorf("agent call received header %q, want %q", receivedCallAuth, expectedHeader)
	}
}
