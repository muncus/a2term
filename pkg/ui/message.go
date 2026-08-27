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

package ui

import (
	"time"

	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"
)

// MessageKind specifies the type of item in the conversation feed.
type MessageKind int

const (
	KindUser MessageKind = iota
	KindAgentText
	KindAgentSurface
	KindSystem
	KindAction
	KindError
	KindDiagnostic
)

// DiagnosticInfo provides structured troubleshooting information for connection or transport errors.
type DiagnosticInfo struct {
	Title     string
	TargetURL string
	Endpoint  string
	Phase     string
	ErrorMsg  string
	Details   string
	Tips      []string
}

// FeedItem is an entry in the chat log.
type FeedItem struct {
	ID         string
	Kind       MessageKind
	Content    string
	Surface    render.Model
	Messages   []tmca2ui.ServerMessage
	SurfaceID  string
	Diagnostic *DiagnosticInfo
	Timestamp  time.Time
}

// NewUserItem creates a feed item for user input.
func NewUserItem(id, text string) FeedItem {
	return FeedItem{
		ID:        id,
		Kind:      KindUser,
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewAgentTextItem creates a feed item for agent text output.
func NewAgentTextItem(id, text string) FeedItem {
	return FeedItem{
		ID:        id,
		Kind:      KindAgentText,
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewAgentSurfaceItem creates a feed item for an interactive A2UI surface.
func NewAgentSurfaceItem(id, surfaceID string, surface render.Model, msgs []tmca2ui.ServerMessage) FeedItem {
	return FeedItem{
		ID:        id,
		Kind:      KindAgentSurface,
		Surface:   surface,
		SurfaceID: surfaceID,
		Messages:  msgs,
		Timestamp: time.Now(),
	}
}

// NewSystemItem creates a feed item for system status.
func NewSystemItem(id, text string) FeedItem {
	return FeedItem{
		ID:        id,
		Kind:      KindSystem,
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewActionItem creates a feed item for an action triggered by the user in an A2UI surface.
func NewActionItem(id, text string) FeedItem {
	return FeedItem{
		ID:        id,
		Kind:      KindAction,
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewErrorItem creates a feed item for errors.
func NewErrorItem(id, text string) FeedItem {
	return FeedItem{
		ID:        id,
		Kind:      KindError,
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewDiagnosticItem creates a feed item with rich diagnostic information.
func NewDiagnosticItem(id string, diag DiagnosticInfo) FeedItem {
	return FeedItem{
		ID:         id,
		Kind:       KindDiagnostic,
		Diagnostic: &diag,
		Timestamp:  time.Now(),
	}
}
