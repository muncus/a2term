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

package a2ui

import (
	"fmt"
	"sync"

	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"
)

// SurfaceState maintains the live lifecycle, component tree, and data store for a single surface.
type SurfaceState struct {
	SurfaceID     string
	CatalogID     string
	Theme         *tmca2ui.Theme
	SendDataModel bool
	Components    map[string]tmca2ui.Component
	RawComponents []tmca2ui.Component
	DataStore     *DataModelStore
	Model         render.Model
	Deleted       bool
}

// SurfaceManager coordinates multiple concurrent surfaces identified by surfaceId.
type SurfaceManager struct {
	mu       sync.RWMutex
	surfaces map[string]*SurfaceState
	order    []string
}

// NewSurfaceManager creates an empty SurfaceManager.
func NewSurfaceManager() *SurfaceManager {
	return &SurfaceManager{
		surfaces: make(map[string]*SurfaceState),
	}
}

// ensureSurface returns the existing SurfaceState or creates a new one with default configuration.
func (sm *SurfaceManager) ensureSurface(surfaceID string) *SurfaceState {
	if surfaceID == "" {
		surfaceID = "default"
	}
	st, exists := sm.surfaces[surfaceID]
	if !exists || st.Deleted {
		st = &SurfaceState{
			SurfaceID:  surfaceID,
			CatalogID:  "https://a2ui.org/catalogs/v0.9/basic.json",
			Components: make(map[string]tmca2ui.Component),
			DataStore:  NewDataModelStore(),
		}
		sm.surfaces[surfaceID] = st
		sm.order = append(sm.order, surfaceID)
	}
	return st
}

// CreateSurface handles an explicit createSurface server message.
func (sm *SurfaceManager) CreateSurface(msg *tmca2ui.CreateSurface) *SurfaceState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	surfaceID := "default"
	if msg != nil && msg.SurfaceID != "" {
		surfaceID = msg.SurfaceID
	}

	st := sm.ensureSurface(surfaceID)
	st.Deleted = false
	if msg != nil {
		if msg.CatalogID != "" {
			st.CatalogID = msg.CatalogID
		}
		if msg.Theme != nil {
			st.Theme = msg.Theme
		}
		st.SendDataModel = msg.SendDataModel
	}
	return st
}

// UpdateComponents handles an updateComponents server message.
// It merges new components with existing components, hydrates dynamic bindings, and re-renders the surface model.
func (sm *SurfaceManager) UpdateComponents(msg *tmca2ui.UpdateComponents, opts ...render.Option) (render.Model, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if msg == nil {
		return nil, fmt.Errorf("nil UpdateComponents message")
	}

	surfaceID := msg.SurfaceID
	if surfaceID == "" {
		surfaceID = "default"
	}

	st := sm.ensureSurface(surfaceID)

	// Merge incoming components into state map
	for _, c := range msg.Components {
		st.Components[c.ID] = c
	}

	// Build linear component list preserving incoming order with existing components
	merged := make([]tmca2ui.Component, 0, len(st.Components))
	seen := make(map[string]bool)

	// Add root / incoming components first
	for _, c := range msg.Components {
		merged = append(merged, st.Components[c.ID])
		seen[c.ID] = true
	}
	// Add remaining components that were not updated
	for id, c := range st.Components {
		if !seen[id] {
			merged = append(merged, c)
		}
	}
	st.RawComponents = merged

	// Hydrate dynamic values using DataStore
	hydrated := HydrateComponents(merged, st.DataStore, nil)

	// Re-render surface
	msgs := []tmca2ui.ServerMessage{
		{
			UpdateComponents: &tmca2ui.UpdateComponents{
				SurfaceID:  surfaceID,
				Components: hydrated,
			},
		},
	}

	model, err := a2tea.Render(msgs, opts...)
	if err != nil {
		return nil, err
	}
	if rm, ok := model.(render.Model); ok {
		st.Model = rm
		return rm, nil
	}
	return nil, nil
}

// UpdateDataModel handles an updateDataModel server message.
// It upserts data at path, re-hydrates components, and re-renders the surface.
func (sm *SurfaceManager) UpdateDataModel(msg *tmca2ui.UpdateDataModel, opts ...render.Option) (render.Model, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if msg == nil {
		return nil, fmt.Errorf("nil UpdateDataModel message")
	}

	surfaceID := msg.SurfaceID
	if surfaceID == "" {
		surfaceID = "default"
	}

	st := sm.ensureSurface(surfaceID)
	st.DataStore.Upsert(msg.Path, msg.Value)

	if len(st.RawComponents) == 0 {
		return nil, nil
	}

	// Re-hydrate components with newly updated data store
	hydrated := HydrateComponents(st.RawComponents, st.DataStore, nil)
	msgs := []tmca2ui.ServerMessage{
		{
			UpdateComponents: &tmca2ui.UpdateComponents{
				SurfaceID:  surfaceID,
				Components: hydrated,
			},
		},
	}

	model, err := a2tea.Render(msgs, opts...)
	if err != nil {
		return nil, err
	}
	if rm, ok := model.(render.Model); ok {
		st.Model = rm
		return rm, nil
	}
	return nil, nil
}

// DeleteSurface marks the targeted surface as deleted and returns true if found.
func (sm *SurfaceManager) DeleteSurface(msg *tmca2ui.DeleteSurface) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if msg == nil {
		return false
	}
	st, exists := sm.surfaces[msg.SurfaceID]
	if !exists || st.Deleted {
		return false
	}
	st.Deleted = true
	st.Model = nil
	st.Components = make(map[string]tmca2ui.Component)
	st.RawComponents = nil
	return true
}

// GetSurface returns the live state of a surface by ID.
func (sm *SurfaceManager) GetSurface(surfaceID string) (*SurfaceState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	st, exists := sm.surfaces[surfaceID]
	if !exists || st.Deleted {
		return nil, false
	}
	return st, true
}

// ExportClientDataModel exports the data model for surfaceID if SendDataModel is true.
func (sm *SurfaceManager) ExportClientDataModel(surfaceID string) map[string]any {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	st, exists := sm.surfaces[surfaceID]
	if !exists || st.Deleted || !st.SendDataModel {
		return nil
	}

	return map[string]any{
		"version": "v0.9",
		"surfaces": map[string]any{
			surfaceID: st.DataStore.Export(),
		},
	}
}

// ValidateInput validates a component's input value against its configured CheckRules.
// If validation fails, it returns a ClientError with Code "ValidationFailed".
func (sm *SurfaceManager) ValidateInput(surfaceID, componentID string, inputVal any) *tmca2ui.ClientError {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	st, exists := sm.surfaces[surfaceID]
	if !exists || st.Deleted {
		return nil
	}

	comp, ok := st.Components[componentID]
	if !ok || len(comp.Checks) == 0 {
		return nil
	}

	scope := map[string]any{
		"value":        inputVal,
		componentID:    inputVal,
	}

	for _, check := range comp.Checks {
		pass, ok := ResolveDynamicBoolean(&check.Condition, st.DataStore, scope)
		if ok && !pass {
			msg := check.Message
			if msg == "" {
				msg = fmt.Sprintf("Validation failed for %s", componentID)
			}
			return &tmca2ui.ClientError{
				Code:      "ValidationFailed",
				SurfaceID: surfaceID,
				Message:   msg,
				Path:      fmt.Sprintf("/%s", componentID),
			}
		}
	}

	return nil
}
