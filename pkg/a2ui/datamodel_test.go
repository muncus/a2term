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
	"reflect"
	"testing"
)

func TestDataModelStore_RootUpsert(t *testing.T) {
	store := NewDataModelStore()
	store.Upsert("", map[string]any{"city": "Paris", "temp": 22})

	val, ok := store.Get("/city")
	if !ok || val != "Paris" {
		t.Fatalf("expected /city to be 'Paris', got %v (found: %v)", val, ok)
	}

	val, ok = store.Get("/temp")
	if !ok || val != 22 {
		t.Fatalf("expected /temp to be 22, got %v (found: %v)", val, ok)
	}
}

func TestDataModelStore_NestedUpsertAndMerge(t *testing.T) {
	store := NewDataModelStore()

	// 1. Initial object at /user
	store.Upsert("/user", map[string]any{
		"name": "Alice",
		"address": map[string]any{
			"city": "London",
		},
	})

	// 2. Incremental upsert at /user/age
	store.Upsert("/user/age", 30)

	// 3. Incremental merge at /user/address
	store.Upsert("/user/address", map[string]any{
		"zip": "SW1A 1AA",
	})

	// Verify name was preserved
	name, ok := store.Get("/user/name")
	if !ok || name != "Alice" {
		t.Errorf("expected /user/name to be 'Alice', got %v", name)
	}

	// Verify age was added
	age, ok := store.Get("/user/age")
	if !ok || age != 30 {
		t.Errorf("expected /user/age to be 30, got %v", age)
	}

	// Verify address was merged (contains both city and zip)
	city, ok := store.Get("/user/address/city")
	if !ok || city != "London" {
		t.Errorf("expected /user/address/city to be 'London', got %v", city)
	}

	zip, ok := store.Get("/user/address/zip")
	if !ok || zip != "SW1A 1AA" {
		t.Errorf("expected /user/address/zip to be 'SW1A 1AA', got %v", zip)
	}
}

func TestDataModelStore_ArrayTraversal(t *testing.T) {
	store := NewDataModelStore()
	store.Upsert("/items", []any{
		map[string]any{"id": "a", "qty": 10},
		map[string]any{"id": "b", "qty": 20},
	})

	item0ID, ok := store.Get("/items/0/id")
	if !ok || item0ID != "a" {
		t.Errorf("expected /items/0/id to be 'a', got %v", item0ID)
	}

	item1Qty, ok := store.Get("/items/1/qty")
	if !ok || item1Qty != 20 {
		t.Errorf("expected /items/1/qty to be 20, got %v", item1Qty)
	}

	// Out of bounds
	_, ok = store.Get("/items/5")
	if ok {
		t.Errorf("expected out-of-bounds array get to fail")
	}
}

func TestDataModelStore_Export(t *testing.T) {
	store := NewDataModelStore()
	store.Upsert("/settings/theme", "dark")
	store.Upsert("/settings/notifications", true)

	exported := store.Export()
	expected := map[string]any{
		"settings": map[string]any{
			"theme":         "dark",
			"notifications": true,
		},
	}

	if !reflect.DeepEqual(exported, expected) {
		t.Errorf("Export() = %v, want %v", exported, expected)
	}
}
