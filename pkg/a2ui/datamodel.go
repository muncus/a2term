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
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

// DataModelStore manages hierarchical data model state for an A2UI surface.
// It implements RFC 6901 JSON Pointer path traversal with upsert/merge semantics.
type DataModelStore struct {
	mu   sync.RWMutex
	root map[string]any
}

// NewDataModelStore creates an empty data model store.
func NewDataModelStore() *DataModelStore {
	return &DataModelStore{
		root: make(map[string]any),
	}
}

// parseJSONPointer decodes an RFC 6901 JSON pointer into tokens.
func parseJSONPointer(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return nil
	}
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	rawTokens := strings.Split(path, "/")
	tokens := make([]string, len(rawTokens))
	for i, token := range rawTokens {
		// RFC 6901 unescape: ~1 -> /, ~0 -> ~
		t := strings.ReplaceAll(token, "~1", "/")
		t = strings.ReplaceAll(t, "~0", "~")
		tokens[i] = t
	}
	return tokens
}

// Upsert updates or inserts a value at the specified JSON pointer path.
// For JSON objects (map[string]any), upsert performs a recursive merge.
// For arrays and primitive scalars, the existing value is replaced.
func (s *DataModelStore) Upsert(path string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokens := parseJSONPointer(path)
	if len(tokens) == 0 {
		// Root replacement or merge
		if vMap, ok := toMap(value); ok {
			mergeMaps(s.root, vMap)
		} else if value != nil {
			s.root[""] = value
		}
		return
	}

	current := s.root
	for i := 0; i < len(tokens)-1; i++ {
		token := tokens[i]
		next, exists := current[token]
		if !exists || next == nil {
			newMap := make(map[string]any)
			current[token] = newMap
			current = newMap
			continue
		}

		if nextMap, ok := next.(map[string]any); ok {
			current = nextMap
		} else if m, ok := toMap(next); ok {
			current[token] = m
			current = m
		} else {
			// Path collision: replace non-map with map
			newMap := make(map[string]any)
			current[token] = newMap
			current = newMap
		}
	}

	lastToken := tokens[len(tokens)-1]
	existing, exists := current[lastToken]
	if exists {
		existingMap, isExistingMap := existing.(map[string]any)
		incomingMap, isIncomingMap := toMap(value)
		if isExistingMap && isIncomingMap {
			mergeMaps(existingMap, incomingMap)
			return
		}
	}

	if incomingMap, ok := toMap(value); ok {
		newMap := make(map[string]any)
		mergeMaps(newMap, incomingMap)
		current[lastToken] = newMap
	} else {
		current[lastToken] = deepCopyValue(value)
	}
}

// Get retrieves the value at the given JSON pointer path.
func (s *DataModelStore) Get(path string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens := parseJSONPointer(path)
	if len(tokens) == 0 {
		return s.Export(), true
	}

	var current any = s.root
	for _, token := range tokens {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[token]
			if !ok {
				return nil, false
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(token)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]
		default:
			return nil, false
		}
	}
	return deepCopyValue(current), true
}

// Export returns a full deep copy of the stored data model.
func (s *DataModelStore) Export() map[string]any {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]any, len(s.root))
	for k, v := range s.root {
		out[k] = deepCopyValue(v)
	}
	return out
}

// mergeMaps recursively merges src into dst.
func mergeMaps(dst, src map[string]any) {
	for k, v := range src {
		if dstVal, exists := dst[k]; exists {
			dstMap, dstIsMap := dstVal.(map[string]any)
			srcMap, srcIsMap := toMap(v)
			if dstIsMap && srcIsMap {
				mergeMaps(dstMap, srcMap)
				continue
			}
		}
		dst[k] = deepCopyValue(v)
	}
}

func toMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	// Try JSON roundtrip for custom struct or map types
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var res map[string]any
	if err := json.Unmarshal(b, &res); err == nil && res != nil {
		return res, true
	}
	return nil, false
}

func deepCopyValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case map[string]any:
		cp := make(map[string]any, len(val))
		for k, item := range val {
			cp[k] = deepCopyValue(item)
		}
		return cp
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = deepCopyValue(item)
		}
		return cp
	default:
		return val
	}
}
