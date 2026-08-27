package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	a2aclient "a2term/pkg/a2a"
	"a2term/pkg/a2ui"
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
