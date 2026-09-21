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
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"

	"github.com/muncus/a2term/pkg/agent"
)

var _ agent.Client = (*Client)(nil)

const (
	// A2UIMIMEType is the standard A2UI MIME type (v0.9.1+).
	A2UIMIMEType = "application/a2ui+json"
	// A2UIMIMETypeLegacy is the legacy A2UI MIME type (v0.9).
	A2UIMIMETypeLegacy = "application/json+a2ui"
	// ClientCapabilitiesKey is the A2A message metadata key for A2UI capabilities.
	ClientCapabilitiesKey = "a2uiClientCapabilities"
	// ClientDataModelKey is the A2A message metadata key for A2UI client data model.
	ClientDataModelKey = "a2uiClientDataModel"
	// A2UIBasicCatalogID is the canonical v0.9 basic component catalog URI.
	A2UIBasicCatalogID = "https://a2ui.org/catalogs/v0.9/basic.json"
)

// DefaultClientCapabilities returns the standardized A2UI v0.9.1 client capabilities map.
func DefaultClientCapabilities() map[string]any {
	return map[string]any{
		"v0.9.1": map[string]any{
			"supportedCatalogIds": []string{
				A2UIBasicCatalogID,
			},
			"acceptsInlineCatalogs": false,
		},
	}
}

func applyClientCapabilities(msg *a2a.Message) {
	if msg == nil {
		return
	}
	msg.SetMeta(ClientCapabilitiesKey, DefaultClientCapabilities())
}

// IsA2UIPart reports whether a part carries A2UI content based on MIME type or metadata.
func IsA2UIPart(part *a2a.Part) bool {
	if part == nil {
		return false
	}
	if part.MediaType == A2UIMIMEType || part.MediaType == A2UIMIMETypeLegacy {
		return true
	}
	if part.Metadata != nil {
		if mt, ok := part.Metadata["mimeType"].(string); ok && (mt == A2UIMIMEType || mt == A2UIMIMETypeLegacy) {
			return true
		}
	}
	return false
}


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
	applyClientCapabilities(userMsg)
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
	return c.SendA2UIAction(ctx, actionName, "", sourceID, contextValues, nil)
}

// SendA2UIAction sends a full A2UI interaction event with optional surfaceID and client data model.
func (c *Client) SendA2UIAction(ctx context.Context, actionName string, surfaceID string, sourceID string, contextValues map[string]any, clientDataModel map[string]any) (string, error) {
	c.mu.Lock()
	if c.client == nil {
		c.mu.Unlock()
		return "", errors.New("client not connected")
	}

	taskInfo := a2a.TaskInfo{
		TaskID:    c.taskID,
		ContextID: c.contextID,
	}

	actionPayload := map[string]any{
		"name":              actionName,
		"sourceComponentId": sourceID,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
		"context":           contextValues,
	}
	if surfaceID != "" {
		actionPayload["surfaceId"] = surfaceID
	} else if sID, ok := contextValues["surfaceId"].(string); ok && sID != "" {
		actionPayload["surfaceId"] = sID
	}

	clientMsg := map[string]any{
		"version": "v0.9",
		"action":  actionPayload,
		// For backward compatibility with older handlers expecting flat action:
		"actionName": actionName,
		"sourceId":   sourceID,
		"context":    contextValues,
	}

	dataPart := a2a.NewDataPart(clientMsg)
	dataPart.MediaType = A2UIMIMEType
	dataPart.Metadata = map[string]any{
		"mimeType": A2UIMIMEType,
	}

	jsonBytes, _ := json.Marshal(clientMsg)
	textPart := a2a.NewTextPart(string(jsonBytes))

	userMsg := a2a.NewMessageForTask(a2a.MessageRoleUser, taskInfo, dataPart, textPart)
	applyClientCapabilities(userMsg)
	if clientDataModel != nil {
		userMsg.SetMeta(ClientDataModelKey, clientDataModel)
	}

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
	applyClientCapabilities(userMsg)
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
		if IsA2UIPart(part) {
			raw := extractPartRawOrData(part)
			trimmed := strings.TrimSpace(raw)
			if trimmed != "" {
				if strings.HasPrefix(trimmed, "<a2ui-json>") {
					sb.WriteString(trimmed)
				} else {
					sb.WriteString("\n<a2ui-json>\n" + trimmed + "\n</a2ui-json>\n")
				}
			}
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

func extractPartRawOrData(part *a2a.Part) string {
	if t := part.Text(); t != "" {
		return t
	}
	if r := part.Raw(); len(r) > 0 {
		return string(r)
	}
	if d := part.Data(); d != nil {
		switch v := d.(type) {
		case string:
			return v
		default:
			if b, err := json.Marshal(v); err == nil {
				return string(b)
			}
		}
	}
	return ""
}


