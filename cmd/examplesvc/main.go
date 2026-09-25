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
	"flag"
	"fmt"
	"iter"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// ExampleExecutor implements a2asrv.AgentExecutor to serve interactive A2UI components over JSON-RPC.
type ExampleExecutor struct{}

func (e *ExampleExecutor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		msg := execCtx.Message
		userText := extractUserText(msg)
		actionName, sourceID, ctxValues, isAction := extractUserAction(msg)

		var agentMsg *a2a.Message

		if isAction {
			log.Printf("[examplesvc] Received action event: action=%s, source=%s, context=%v", actionName, sourceID, ctxValues)
			switch actionName {
			case "return_to_showcase", "refresh_data":
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(showcaseCard()))
			case "refresh_weather":
				city := "San Francisco, CA"
				if c, ok := ctxValues["city"].(string); ok && c != "" {
					city = c
				}
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(weatherCard(city, 72, "Sunny")))
			default:
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(actionResultCard(actionName, sourceID, ctxValues)))
			}
		} else {
			cleanText := strings.ToLower(strings.TrimSpace(userText))
			log.Printf("[examplesvc] Received user prompt: %q", cleanText)

			switch {
			case cleanText == "buttons" || cleanText == "button":
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(buttonsCard()))
			case cleanText == "form" || cleanText == "inputs" || cleanText == "input":
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(formCard()))
			case cleanText == "weather":
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(weatherCard("San Francisco, CA", 68, "Partly Cloudy")))
			case cleanText == "multipart" || cleanText == "multi" || cleanText == "multipart-card":
				agentMsg = multipartMessage()
			case cleanText == "help":
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("Available A2UI demo commands:\n- `showcase` or `all` : View the full interactive A2UI showcase\n- `buttons` : Test action buttons and events\n- `form` : Test editable text fields, check boxes, sliders\n- `weather` : Test dynamic card widgets\n- `multipart` : Test multi-part A2A response (text + A2UI DataPart)\n- `ping` : Test server responsiveness"))
			case cleanText == "ping":
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("Pong! 🏓 The A2UI test agent is alive and ready.\n\n"+buttonsCard()))
			default:
				agentMsg = a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(showcaseCard()))
			}
		}

		// Yield agent response message with ContextID preserved and empty TaskID
		agentMsg.ContextID = execCtx.ContextID
		yield(agentMsg, nil)
	}
}

func (e *ExampleExecutor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

func extractUserText(msg *a2a.Message) string {
	if msg == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range msg.Parts {
		if part == nil {
			continue
		}
		if t := part.Text(); t != "" {
			sb.WriteString(t)
		}
	}
	return strings.TrimSpace(sb.String())
}

func extractUserAction(msg *a2a.Message) (actionName string, sourceID string, ctxValues map[string]any, found bool) {
	if msg == nil {
		return "", "", nil, false
	}
	for _, part := range msg.Parts {
		if part == nil {
			continue
		}
		if d := part.Data(); d != nil {
			if m, ok := d.(map[string]any); ok {
				if actMap, ok := m["action"].(map[string]any); ok {
					name, _ := actMap["name"].(string)
					src, _ := actMap["sourceComponentId"].(string)
					ctx, _ := actMap["context"].(map[string]any)
					if name != "" {
						return name, src, ctx, true
					}
				}
				if act, ok := m["action"].(string); ok && act != "" {
					src, _ := m["sourceId"].(string)
					ctx, _ := m["context"].(map[string]any)
					return act, src, ctx, true
				}
			}
		}
	}
	return "", "", nil, false
}

// NewServerHandler creates the HTTP handler for the JSON-RPC A2A service.
func NewServerHandler(card *a2a.AgentCard, reqHandler a2asrv.RequestHandler, baseURL string) http.Handler {
	cardHandler := a2asrv.NewStaticAgentCardHandler(card)
	jsonrpcHandler := a2asrv.NewJSONRPCHandler(reqHandler)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/agent-card.json" && r.Method == http.MethodGet:
			cardHandler.ServeHTTP(w, r)
		case r.Method == http.MethodPost:
			jsonrpcHandler.ServeHTTP(w, r)
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "🤖 A2UI Example Service (JSON-RPC) is running at %s\n\nConnect with github.com/muncus/a2term:\n  ./github.com/muncus/a2term --agent=%s\n", baseURL, baseURL)
		default:
			http.NotFound(w, r)
		}
	})
}

func main() {
	port := flag.Int("port", 9002, "Port to listen on")
	host := flag.String("host", "127.0.0.1", "Host address to bind to")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	baseURL := fmt.Sprintf("http://%s:%d", *host, *port)

	card := &a2a.AgentCard{
		Name:        "A2UI Example Agent",
		Description: "Interactive A2UI test agent providing controls and interactive cards for github.com/muncus/a2term",
		Version:     "1.0.0",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(baseURL, a2a.TransportProtocolJSONRPC),
		},
	}

	executor := &ExampleExecutor{}
	reqHandler := a2asrv.NewHandler(executor)
	handler := NewServerHandler(card, reqHandler, baseURL)

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		log.Printf("==================================================")
		log.Printf("🚀 A2UI Example Service (JSON-RPC) on %s", baseURL)
		log.Printf("   Agent Card: %s/.well-known/agent-card.json", baseURL)
		log.Printf("   Test with:  ./github.com/muncus/a2term --agent=%s", baseURL)
		log.Printf("==================================================")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down A2UI Example Service...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Server stopped.")
}
