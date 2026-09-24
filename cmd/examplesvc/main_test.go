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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	a2aclient "github.com/muncus/a2term/pkg/a2a"
	"github.com/muncus/a2term/pkg/a2ui"
)

func TestCardsParseWithA2UI(t *testing.T) {
	cards := map[string]string{
		"showcase":     showcaseCard(),
		"buttons":      buttonsCard(),
		"form":         formCard(),
		"weather":      weatherCard("New York, NY", 75, "Clear"),
		"actionResult": actionResultCard("submit_form", "btn_submit", map[string]any{"user": "alice"}),
	}

	for name, cardText := range cards {
		t.Run(name, func(t *testing.T) {
			segments, err := a2ui.ParseAgentResponse(cardText)
			if err != nil {
				t.Fatalf("card %q failed to parse: %v", name, err)
			}

			if len(segments) == 0 {
				t.Fatalf("card %q produced 0 segments", name)
			}

			foundSurface := false
			for _, seg := range segments {
				if seg.Type == a2ui.TypeSurface && seg.Surface != nil {
					foundSurface = true
					viewContent := seg.Surface.View().Content
					if len(strings.TrimSpace(viewContent)) == 0 {
						t.Errorf("card %q rendered empty surface content", name)
					}
				}
			}

			if !foundSurface {
				t.Errorf("card %q did not produce a rendered A2UI surface", name)
			}
		})
	}
}

func TestExampleServiceEndToEndJSONRPC(t *testing.T) {
	executor := &ExampleExecutor{}
	reqHandler := a2asrv.NewHandler(executor)

	var serverURL string
	cardProducer := a2asrv.AgentCardProducerFn(func(_ context.Context) (*a2a.AgentCard, error) {
		return &a2a.AgentCard{
			Name:        "Test A2UI Agent",
			Description: "A2UI test agent",
			SupportedInterfaces: []*a2a.AgentInterface{
				a2a.NewAgentInterface(serverURL, a2a.TransportProtocolJSONRPC),
			},
		}, nil
	})

	cardHandler := a2asrv.NewAgentCardHandler(cardProducer)
	jsonrpcHandler := a2asrv.NewJSONRPCHandler(reqHandler)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/agent-card.json" && r.Method == http.MethodGet:
			cardHandler.ServeHTTP(w, r)
		case r.Method == http.MethodPost:
			jsonrpcHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()
	serverURL = server.URL

	ctx := context.Background()
	client, err := a2aclient.NewClient(ctx, a2aclient.ClientOptions{
		AgentURL: serverURL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 1. Send text prompt for buttons
	resp, err := client.SendMessage(ctx, "buttons")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if !strings.Contains(resp, "Button Gallery") {
		t.Errorf("expected response to contain 'Button Gallery', got: %q", resp)
	}

	// 2. Send action event
	actionResp, err := client.SendActionEvent(ctx, "approve_request", "btn_approve", map[string]any{"item_id": "123"})
	if err != nil {
		t.Fatalf("SendActionEvent failed: %v", err)
	}
	if !strings.Contains(actionResp, "approve_request") {
		t.Errorf("expected action response to contain 'approve_request', got: %q", actionResp)
	}

	// 3. Send prompt for showcase
	showcaseResp, err := client.SendMessage(ctx, "showcase")
	if err != nil {
		t.Fatalf("SendMessage for showcase failed: %v", err)
	}
	if !strings.Contains(showcaseResp, "A2UI Interactive Component Showcase") {
		t.Errorf("expected showcase response, got: %q", showcaseResp)
	}
}

func TestMultipartResponse(t *testing.T) {
	msg := multipartMessage()
	if msg == nil {
		t.Fatal("expected non-nil multipart message")
	}
	if len(msg.Parts) != 2 {
		t.Fatalf("expected 2 parts in multipart response, got %d", len(msg.Parts))
	}

	// Part 0: Text part
	textPart := msg.Parts[0]
	if textPart.Text() == "" {
		t.Error("expected non-empty text part")
	}

	// Part 1: A2UI DataPart with application/a2ui+json
	uiPart := msg.Parts[1]
	if uiPart.MediaType != A2UIMIMEType {
		t.Errorf("expected MediaType %q, got %q", A2UIMIMEType, uiPart.MediaType)
	}
	if mime, ok := uiPart.Metadata["mimeType"].(string); !ok || mime != A2UIMIMEType {
		t.Errorf("expected metadata mimeType %q, got %v", A2UIMIMEType, uiPart.Metadata["mimeType"])
	}
	if uiPart.Data() == nil {
		t.Fatal("expected non-nil Data in uiPart")
	}

	// Verify also via multipartCard alias
	cardMsg := multipartCard()
	if cardMsg == nil || len(cardMsg.Parts) != 2 {
		t.Errorf("expected multipartCard to return valid 2-part message")
	}
}

