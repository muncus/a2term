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
	"github.com/joestump-agent/a2tea/render"
	tmca2ui "github.com/tmc/a2ui"
)

// DispatchEventType indicates the type of mutation produced by dispatching a ServerMessage.
type DispatchEventType int

const (
	EventSurfaceCreated DispatchEventType = iota
	EventSurfaceUpdated
	EventSurfaceDeleted
)

// DispatchResult reports the outcome of dispatching an A2UI ServerMessage.
type DispatchResult struct {
	Type      DispatchEventType
	SurfaceID string
	Model     render.Model
	Message   tmca2ui.ServerMessage
}

// Dispatcher coordinates incoming ServerMessages with a SurfaceManager.
type Dispatcher struct {
	manager *SurfaceManager
	opts    []render.Option
}

// NewDispatcher creates a new Dispatcher for a SurfaceManager.
func NewDispatcher(manager *SurfaceManager, opts ...render.Option) *Dispatcher {
	return &Dispatcher{
		manager: manager,
		opts:    opts,
	}
}

// Dispatch processes a single ServerMessage and updates the targeted surface state.
func (d *Dispatcher) Dispatch(msg tmca2ui.ServerMessage) (*DispatchResult, error) {
	switch {
	case msg.CreateSurface != nil:
		st := d.manager.CreateSurface(msg.CreateSurface)
		return &DispatchResult{
			Type:      EventSurfaceCreated,
			SurfaceID: st.SurfaceID,
			Message:   msg,
		}, nil

	case msg.UpdateComponents != nil:
		m, err := d.manager.UpdateComponents(msg.UpdateComponents, d.opts...)
		if err != nil {
			return nil, err
		}
		return &DispatchResult{
			Type:      EventSurfaceUpdated,
			SurfaceID: msg.UpdateComponents.SurfaceID,
			Model:     m,
			Message:   msg,
		}, nil

	case msg.UpdateDataModel != nil:
		m, err := d.manager.UpdateDataModel(msg.UpdateDataModel, d.opts...)
		if err != nil {
			return nil, err
		}
		return &DispatchResult{
			Type:      EventSurfaceUpdated,
			SurfaceID: msg.UpdateDataModel.SurfaceID,
			Model:     m,
			Message:   msg,
		}, nil

	case msg.DeleteSurface != nil:
		d.manager.DeleteSurface(msg.DeleteSurface)
		return &DispatchResult{
			Type:      EventSurfaceDeleted,
			SurfaceID: msg.DeleteSurface.SurfaceID,
			Message:   msg,
		}, nil
	}

	return nil, nil
}

// DispatchBatch processes a slice of ServerMessages sequentially.
func (d *Dispatcher) DispatchBatch(msgs []tmca2ui.ServerMessage) ([]DispatchResult, error) {
	var results []DispatchResult
	for _, m := range msgs {
		res, err := d.Dispatch(m)
		if err != nil {
			return results, err
		}
		if res != nil {
			results = append(results, *res)
		}
	}
	return results, nil
}
