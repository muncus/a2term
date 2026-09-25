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
	tmca2ui "github.com/tmc/a2ui"
)

// HydrateComponents walks a slice of A2UI components and resolves all dynamic bindings
// and function calls (such as formatString) using the provided DataModelStore.
// This ensures that when the components are rendered, all dynamic strings, numbers,
// booleans, and list selections are converted into concrete literals.
func HydrateComponents(components []tmca2ui.Component, store *DataModelStore, scope map[string]any) []tmca2ui.Component {
	if len(components) == 0 {
		return components
	}

	hydrated := make([]tmca2ui.Component, len(components))
	for i, c := range components {
		hydrated[i] = hydrateComponent(c, store, scope)
	}
	return hydrated
}

func hydrateComponent(c tmca2ui.Component, store *DataModelStore, scope map[string]any) tmca2ui.Component {
	// 1. Accessibility attributes
	if c.Accessibility != nil {
		acc := *c.Accessibility
		if acc.Label != nil {
			if s, ok := ResolveDynamicString(acc.Label, store, scope); ok {
				acc.Label = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		if acc.Description != nil {
			if s, ok := ResolveDynamicString(acc.Description, store, scope); ok {
				acc.Description = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		c.Accessibility = &acc
	}

	// 2. Concrete component types
	switch {
	case c.Text != nil:
		t := *c.Text
		if s, ok := ResolveDynamicString(&t.Text, store, scope); ok {
			t.Text = tmca2ui.DynamicString{Literal: &s}
		}
		c.Text = &t

	case c.TextField != nil:
		tf := *c.TextField
		if s, ok := ResolveDynamicString(&tf.Label, store, scope); ok {
			tf.Label = tmca2ui.DynamicString{Literal: &s}
		}
		if tf.Value != nil {
			if s, ok := ResolveDynamicString(tf.Value, store, scope); ok {
				tf.Value = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		c.TextField = &tf

	case c.CheckBox != nil:
		cb := *c.CheckBox
		if s, ok := ResolveDynamicString(&cb.Label, store, scope); ok {
			cb.Label = tmca2ui.DynamicString{Literal: &s}
		}
		if b, ok := ResolveDynamicBoolean(&cb.Value, store, scope); ok {
			cb.Value = tmca2ui.DynamicBoolean{Literal: &b}
		}
		c.CheckBox = &cb

	case c.ChoicePicker != nil:
		cp := *c.ChoicePicker
		if cp.Label != nil {
			if s, ok := ResolveDynamicString(cp.Label, store, scope); ok {
				cp.Label = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		if len(cp.Options) > 0 {
			opts := make([]tmca2ui.ChoiceOption, len(cp.Options))
			for i, opt := range cp.Options {
				opts[i] = opt
				if s, ok := ResolveDynamicString(&opt.Label, store, scope); ok {
					opts[i].Label = tmca2ui.DynamicString{Literal: &s}
				}
			}
			cp.Options = opts
		}
		if sl, ok := ResolveDynamicStringList(&cp.Value, store, scope); ok {
			cp.Value = tmca2ui.DynamicStringList{Literal: sl}
		}
		c.ChoicePicker = &cp

	case c.Slider != nil:
		sl := *c.Slider
		if sl.Label != nil {
			if s, ok := ResolveDynamicString(sl.Label, store, scope); ok {
				sl.Label = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		if n, ok := ResolveDynamicNumber(&sl.Value, store, scope); ok {
			sl.Value = tmca2ui.DynamicNumber{Literal: &n}
		}
		c.Slider = &sl

	case c.DateTimeInput != nil:
		dt := *c.DateTimeInput
		if dt.Label != nil {
			if s, ok := ResolveDynamicString(dt.Label, store, scope); ok {
				dt.Label = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		if s, ok := ResolveDynamicString(&dt.Value, store, scope); ok {
			dt.Value = tmca2ui.DynamicString{Literal: &s}
		}
		c.DateTimeInput = &dt

	case c.Image != nil:
		img := *c.Image
		if s, ok := ResolveDynamicString(&img.URL, store, scope); ok {
			img.URL = tmca2ui.DynamicString{Literal: &s}
		}
		if img.Description != nil {
			if s, ok := ResolveDynamicString(img.Description, store, scope); ok {
				img.Description = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		c.Image = &img

	case c.Video != nil:
		v := *c.Video
		if s, ok := ResolveDynamicString(&v.URL, store, scope); ok {
			v.URL = tmca2ui.DynamicString{Literal: &s}
		}
		c.Video = &v

	case c.AudioPlayer != nil:
		ap := *c.AudioPlayer
		if s, ok := ResolveDynamicString(&ap.URL, store, scope); ok {
			ap.URL = tmca2ui.DynamicString{Literal: &s}
		}
		if ap.Description != nil {
			if s, ok := ResolveDynamicString(ap.Description, store, scope); ok {
				ap.Description = &tmca2ui.DynamicString{Literal: &s}
			}
		}
		c.AudioPlayer = &ap

	case c.Tabs != nil:
		tabs := *c.Tabs
		if len(tabs.Tabs) > 0 {
			tabDefs := make([]tmca2ui.TabDef, len(tabs.Tabs))
			for i, tb := range tabs.Tabs {
				tabDefs[i] = tb
				if s, ok := ResolveDynamicString(&tb.Title, store, scope); ok {
					tabDefs[i].Title = tmca2ui.DynamicString{Literal: &s}
				}
			}
			tabs.Tabs = tabDefs
		}
		c.Tabs = &tabs
	}

	return c
}
