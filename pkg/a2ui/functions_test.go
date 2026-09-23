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
	"testing"

	tmca2ui "github.com/tmc/a2ui"
)

func TestFunctionEvaluator_FormatString(t *testing.T) {
	store := NewDataModelStore()
	store.Upsert("/user/name", "Bob")
	store.Upsert("/weather/city", "Seattle")
	store.Upsert("/weather/temp", 55)

	reg := DefaultRegistry()

	// 1. Interpolation using ${var} mapped to args with data binding
	fc := &tmca2ui.FunctionCall{
		Call: "formatString",
		Args: map[string]any{
			"template": "Hello ${name}, the temperature in ${city} is ${temp}°F",
			"name":     map[string]any{"path": "/user/name"},
			"city":     map[string]any{"path": "/weather/city"},
			"temp":     map[string]any{"path": "/weather/temp"},
		},
	}

	res, err := reg.Execute(fc, store, nil)
	if err != nil {
		t.Fatalf("Execute formatString failed: %v", err)
	}
	expected := "Hello Bob, the temperature in Seattle is 55°F"
	if res != expected {
		t.Errorf("got %q, want %q", res, expected)
	}

	// 2. Interpolation using indexed variables {0}, {1}
	fcIndexed := &tmca2ui.FunctionCall{
		Call: "formatString",
		Args: map[string]any{
			"template":  "{0} is located in {1}",
			"arguments": []any{"The Space Needle", "Seattle"},
		},
	}
	resIndexed, err := reg.Execute(fcIndexed, store, nil)
	if err != nil {
		t.Fatalf("Execute indexed formatString failed: %v", err)
	}
	expectedIndexed := "The Space Needle is located in Seattle"
	if resIndexed != expectedIndexed {
		t.Errorf("got %q, want %q", resIndexed, expectedIndexed)
	}
}

func TestFunctionEvaluator_LogicAndComparison(t *testing.T) {
	reg := DefaultRegistry()

	// equal
	resEqual, err := reg.Execute(&tmca2ui.FunctionCall{
		Call: "equal",
		Args: map[string]any{"left": "abc", "right": "abc"},
	}, nil, nil)
	if err != nil || resEqual != true {
		t.Errorf("equal failed: %v (res=%v)", err, resEqual)
	}

	// greaterThan
	resGT, err := reg.Execute(&tmca2ui.FunctionCall{
		Call: "greaterThan",
		Args: map[string]any{"left": 10, "right": 5},
	}, nil, nil)
	if err != nil || resGT != true {
		t.Errorf("greaterThan failed: %v (res=%v)", err, resGT)
	}

	// not
	resNot, err := reg.Execute(&tmca2ui.FunctionCall{
		Call: "not",
		Args: map[string]any{"value": true},
	}, nil, nil)
	if err != nil || resNot != false {
		t.Errorf("not failed: %v (res=%v)", err, resNot)
	}
}

func TestResolveDynamicString_FunctionCall(t *testing.T) {
	store := NewDataModelStore()
	store.Upsert("/status", "Ready")

	ds := &tmca2ui.DynamicString{
		FunctionCall: &tmca2ui.FunctionCall{
			Call: "formatString",
			Args: map[string]any{
				"template": "Status: ${status}",
				"status":   map[string]any{"path": "/status"},
			},
		},
	}

	resolved, ok := ResolveDynamicString(ds, store, nil)
	if !ok || resolved != "Status: Ready" {
		t.Errorf("ResolveDynamicString() = %q, %v; want 'Status: Ready', true", resolved, ok)
	}
}

func TestResolveDynamicBooleanAndNumber(t *testing.T) {
	store := NewDataModelStore()
	store.Upsert("/form/accepted", true)
	store.Upsert("/form/score", 88.5)

	// Boolean binding
	pathAccepted := "/form/accepted"
	db := &tmca2ui.DynamicBoolean{
		Binding: &tmca2ui.DataBinding{Path: pathAccepted},
	}
	bVal, ok := ResolveDynamicBoolean(db, store, nil)
	if !ok || !bVal {
		t.Errorf("ResolveDynamicBoolean() = %v, %v; want true, true", bVal, ok)
	}

	// Number binding
	pathScore := "/form/score"
	dn := &tmca2ui.DynamicNumber{
		Binding: &tmca2ui.DataBinding{Path: pathScore},
	}
	nVal, ok := ResolveDynamicNumber(dn, store, nil)
	if !ok || nVal != 88.5 {
		t.Errorf("ResolveDynamicNumber() = %v, %v; want 88.5, true", nVal, ok)
	}
}
