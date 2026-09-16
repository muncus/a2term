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

package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"

	"github.com/muncus/a2term/pkg/agent"
)

var _ agent.Client = (*Client)(nil)

// Client wraps an A2A client instance and manages conversation state.
type Client struct {
	mu        sync.RWMutex
	baseURL   string
	cardURL   string
	authToken string
	card      *a2a.AgentCard
	client    *a2aclient.Client
	contextID string
	taskID    a2a.TaskID
}

// ClientOptions holds options for constructing a Client.
type ClientOptions struct {
	AgentURL  string
	CardURL   string
	AuthToken string
}

// FormatAuthHeader formats a token into a Bearer Authorization header value.
// If a token is provided, it formats it as "Bearer <token>".
// If the token is empty, it returns an empty string.
func FormatAuthHeader(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

type authInterceptor struct {
	getAuthHeader func() string
}

func (a *authInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	if a.getAuthHeader != nil {
		if header := a.getAuthHeader(); header != "" {
			if req.ServiceParams == nil {
				req.ServiceParams = make(a2aclient.ServiceParams)
			}
			req.ServiceParams["Authorization"] = []string{header}
		}
	}
	return ctx, nil, nil
}

func (a *authInterceptor) After(ctx context.Context, resp *a2aclient.Response) error {
	return nil
}

// NewClient initializes a new A2A client given an Agent URL or Card URL.
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	c := &Client{
		baseURL:   strings.TrimRight(opts.AgentURL, "/"),
		cardURL:   opts.CardURL,
		authToken: opts.AuthToken,
	}

	if c.baseURL == "" && c.cardURL == "" {
		return nil, errors.New("either AgentURL or CardURL must be provided")
	}

	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// Connect establishes or re-establishes connection to the A2A agent.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var card *a2a.AgentCard
	var err error

	targetURL := c.cardURL
	if targetURL == "" {
		targetURL = c.baseURL
	}

	authHeader := FormatAuthHeader(c.authToken)

	// Try resolving agent card first
	resolver := agentcard.DefaultResolver
	var cardOpts []agentcard.ResolveOption
	if authHeader != "" {
		cardOpts = append(cardOpts, agentcard.WithRequestHeader("Authorization", authHeader))
	}

	if c.cardURL != "" {
		// If explicit card URL provided
		u, parseErr := url.Parse(c.cardURL)
		if parseErr == nil {
			path := u.Path
			u.Path = ""
			base := u.String()
			opts := append([]agentcard.ResolveOption{agentcard.WithPath(path)}, cardOpts...)
			card, err = resolver.Resolve(ctx, base, opts...)
		} else {
			card, err = resolver.Resolve(ctx, c.cardURL, cardOpts...)
		}
	} else if c.baseURL != "" {
		card, err = resolver.Resolve(ctx, c.baseURL, cardOpts...)
	}

	factoryOpts := []a2aclient.FactoryOption{
		a2aclient.WithCallInterceptors(&authInterceptor{
			getAuthHeader: func() string {
				c.mu.RLock()
				defer c.mu.RUnlock()
				return FormatAuthHeader(c.authToken)
			},
		}),
	}

	var createErr error
	if err == nil && card != nil {
		c.card = card
		var cli *a2aclient.Client
		cli, createErr = a2aclient.NewFromCard(ctx, card, factoryOpts...)
		if createErr == nil {
			c.client = cli
			return nil
		}
	}

	// Fallback to direct endpoints if agent card was not resolvable or card transport failed
	if c.baseURL != "" {
		endpoints := []*a2a.AgentInterface{
			a2a.NewAgentInterface(c.baseURL, a2a.TransportProtocolJSONRPC),
			a2a.NewAgentInterface(c.baseURL, a2a.TransportProtocolHTTPJSON),
		}
		cli, epErr := a2aclient.NewFromEndpoints(ctx, endpoints, factoryOpts...)
		if epErr == nil {
			c.client = cli
			return nil
		}
		if createErr != nil {
			return fmt.Errorf("agent card resolved (%s) but transport negotiation failed: %v; direct endpoint failed: %w", card.Name, createErr, epErr)
		}
		if err != nil {
			return fmt.Errorf("agent card resolution failed (%w); direct endpoint failed: %v", err, epErr)
		}
		return fmt.Errorf("failed to connect to agent endpoint: %w", epErr)
	}

	if createErr != nil {
		return fmt.Errorf("agent card resolved (%s) but transport negotiation failed: %w", card.Name, createErr)
	}
	if err != nil {
		return fmt.Errorf("failed to resolve agent card at %s: %w", targetURL, err)
	}
	return errors.New("unable to establish agent connection")
}

// AgentCard returns the discovered AgentCard if available.
func (c *Client) AgentCard() *a2a.AgentCard {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.card
}

// AgentName returns the agent name or a fallback display string.
func (c *Client) AgentName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.card != nil && c.card.Name != "" {
		return c.card.Name
	}
	if c.baseURL != "" {
		return c.baseURL
	}
	return "A2A Agent"
}

// AuthToken returns the configured authentication token.
func (c *Client) AuthToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authToken
}

// SetAuthToken updates the authentication token used by the client for future requests.
func (c *Client) SetAuthToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authToken = token
}

// CurrentSession returns the current ContextID and TaskID encapsulated in agent.SessionInfo.
func (c *Client) CurrentSession() agent.SessionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return agent.SessionInfo{
		ContextID: c.contextID,
		TaskID:    string(c.taskID),
	}
}

// ResetSession resets the active TaskID and ContextID to start a fresh conversation.
func (c *Client) ResetSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskID = ""
	c.contextID = ""
}

// SendMessage sends a user message and returns the response text (including any embedded A2UI tags).
func (c *Client) SendMessage(ctx context.Context, text string) (string, error) {
	c.mu.Lock()
	if c.client == nil {
		c.mu.Unlock()
		return "", errors.New("client not connected")
	}

	taskInfo := a2a.TaskInfo{
		TaskID:    c.taskID,
		ContextID: c.contextID,
	}

	userMsg := a2a.NewMessageForTask(a2a.MessageRoleUser, taskInfo, a2a.NewTextPart(text))
	req := &a2a.SendMessageRequest{
		Message: userMsg,
	}
	client := c.client
	c.mu.Unlock()

	result, err := client.SendMessage(ctx, req)
	if err != nil {
		return "", err
	}

	return c.processResult(result)
}

// SendActionEvent sends an interaction event (such as a button click or form submission) back to the agent.
func (c *Client) SendActionEvent(ctx context.Context, actionName string, sourceID string, contextValues map[string]any) (string, error) {
	c.mu.Lock()
	if c.client == nil {
		c.mu.Unlock()
		return "", errors.New("client not connected")
	}

	taskInfo := a2a.TaskInfo{
		TaskID:    c.taskID,
		ContextID: c.contextID,
	}

	payload := map[string]any{
		"action":   actionName,
		"sourceId": sourceID,
		"context":  contextValues,
	}

	// Create user message with action data and text representation
	dataPart := a2a.NewDataPart(payload)
	jsonBytes, _ := json.Marshal(payload)
	textPart := a2a.NewTextPart(string(jsonBytes))

	userMsg := a2a.NewMessageForTask(a2a.MessageRoleUser, taskInfo, dataPart, textPart)
	req := &a2a.SendMessageRequest{
		Message: userMsg,
	}
	client := c.client
	c.mu.Unlock()

	result, err := client.SendMessage(ctx, req)
	if err != nil {
		return "", err
	}

	return c.processResult(result)
}

// StreamMessage sends a message and yields streaming events as they arrive.
func (c *Client) StreamMessage(ctx context.Context, text string, onChunk func(chunk string, isFinal bool, err error)) error {
	c.mu.Lock()
	if c.client == nil {
		c.mu.Unlock()
		return errors.New("client not connected")
	}

	taskInfo := a2a.TaskInfo{
		TaskID:    c.taskID,
		ContextID: c.contextID,
	}

	userMsg := a2a.NewMessageForTask(a2a.MessageRoleUser, taskInfo, a2a.NewTextPart(text))
	req := &a2a.SendMessageRequest{
		Message: userMsg,
	}
	client := c.client
	c.mu.Unlock()

	var accumulated strings.Builder
	for event, err := range client.SendStreamingMessage(ctx, req) {
		if err != nil {
			onChunk("", true, err)
			return err
		}

		if event == nil {
			continue
		}

		// Update session IDs
		info := event.TaskInfo()
		if info.TaskID != "" || info.ContextID != "" {
			c.mu.Lock()
			if info.TaskID != "" {
				c.taskID = info.TaskID
			}
			if info.ContextID != "" {
				c.contextID = info.ContextID
			}
			c.mu.Unlock()
		}

		switch ev := event.(type) {
		case *a2a.Message:
			text := ExtractText(ev)
			if text != "" {
				accumulated.WriteString(text)
				onChunk(accumulated.String(), false, nil)
			}
		case *a2a.TaskStatusUpdateEvent:
			isFinal := ev.Status.State.Terminal()
			if isFinal {
				c.mu.Lock()
				c.taskID = ""
				c.mu.Unlock()
			}
			if ev.Status.Message != nil {
				text := ExtractText(ev.Status.Message)
				if text != "" {
					accumulated.WriteString(text)
					onChunk(accumulated.String(), isFinal, nil)
				}
			}
			if isFinal {
				onChunk(accumulated.String(), true, nil)
				return nil
			}
		case *a2a.TaskArtifactUpdateEvent:
			if ev.Artifact != nil {
				text := ExtractArtifactText(ev.Artifact)
				if text != "" {
					accumulated.WriteString(text)
					onChunk(accumulated.String(), ev.LastChunk, nil)
				}
			}
		case *a2a.Task:
			isTerminal := ev.Status.State.Terminal()
			if isTerminal {
				c.mu.Lock()
				c.taskID = ""
				c.mu.Unlock()
			}
			var text string
			if ev.Status.Message != nil {
				text = ExtractText(ev.Status.Message)
			}
			if text == "" && len(ev.Artifacts) > 0 {
				var sb strings.Builder
				for _, art := range ev.Artifacts {
					sb.WriteString(ExtractArtifactText(art))
				}
				text = sb.String()
			}
			if text == "" && len(ev.History) > 0 {
				for i := len(ev.History) - 1; i >= 0; i-- {
					if ev.History[i].Role == a2a.MessageRoleAgent {
						text = ExtractText(ev.History[i])
						if text != "" {
							break
						}
					}
				}
			}
			if text != "" {
				accumulated.WriteString(text)
			}
			onChunk(accumulated.String(), isTerminal, nil)
			if isTerminal {
				return nil
			}
		}
	}

	onChunk(accumulated.String(), true, nil)
	return nil
}

func (c *Client) processResult(result a2a.SendMessageResult) (string, error) {
	if result == nil {
		return "", nil
	}

	info := result.TaskInfo()
	c.mu.Lock()
	if info.TaskID != "" {
		c.taskID = info.TaskID
	}
	if info.ContextID != "" {
		c.contextID = info.ContextID
	}
	c.mu.Unlock()

	switch r := result.(type) {
	case *a2a.Message:
		return ExtractText(r), nil
	case *a2a.Task:
		if r.Status.State.Terminal() {
			c.mu.Lock()
			c.taskID = ""
			c.mu.Unlock()
		}

		// 1. Check Status.Message
		if r.Status.Message != nil {
			txt := ExtractText(r.Status.Message)
			if txt != "" {
				return txt, nil
			}
		}

		// 2. Check Artifacts (common pattern for ADK and generative agents)
		if len(r.Artifacts) > 0 {
			var sb strings.Builder
			for _, art := range r.Artifacts {
				sb.WriteString(ExtractArtifactText(art))
			}
			if txt := sb.String(); txt != "" {
				return txt, nil
			}
		}

		// 3. Check History
		if len(r.History) > 0 {
			// Find latest agent message in history
			for i := len(r.History) - 1; i >= 0; i-- {
				if r.History[i].Role == a2a.MessageRoleAgent {
					return ExtractText(r.History[i]), nil
				}
			}
		}
		return fmt.Sprintf("Task %s: %s", r.ID, r.Status.State), nil
	default:
		return fmt.Sprintf("%v", result), nil
	}
}

// ExtractArtifactText retrieves all text content from an Artifact.
func ExtractArtifactText(art *a2a.Artifact) string {
	if art == nil {
		return ""
	}
	return extractPartsText(art.Parts)
}

// ExtractText retrieves all text content from an A2A Message.
func ExtractText(msg *a2a.Message) string {
	if msg == nil {
		return ""
	}
	return extractPartsText(msg.Parts)
}

// extractPartsText iterates through a slice of A2A Parts and concatenates all text/raw/url/data representations.
func extractPartsText(parts a2a.ContentParts) string {
	var sb strings.Builder
	for _, part := range parts {
		if part == nil {
			continue
		}
		if t := part.Text(); t != "" {
			sb.WriteString(t)
		} else if r := part.Raw(); len(r) > 0 {
			sb.WriteString(string(r))
		} else if u := part.URL(); u != "" {
			sb.WriteString(string(u))
		} else if d := part.Data(); d != nil {
			switch v := d.(type) {
			case string:
				sb.WriteString(v)
			case map[string]any:
				if raw, ok := v["raw"].(string); ok {
					sb.WriteString(raw)
				} else if text, ok := v["text"].(string); ok {
					sb.WriteString(text)
				} else if jsonStr, err := json.Marshal(v); err == nil {
					sb.WriteString(string(jsonStr))
				}
			default:
				if jsonStr, err := json.Marshal(v); err == nil {
					sb.WriteString(string(jsonStr))
				}
			}
		}
	}
	return sb.String()
}

