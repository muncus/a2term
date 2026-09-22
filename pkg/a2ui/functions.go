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
	"regexp"
	"strconv"
	"strings"

	tmca2ui "github.com/tmc/a2ui"
)

var (
	namedVarPattern    = regexp.MustCompile(`\$\{([a-zA-Z0-9_/-]+)\}`)
	indexedVarPattern  = regexp.MustCompile(`\{([0-9]+)\}`)
)

// FunctionHandler evaluates a named client-side function call.
type FunctionHandler func(args map[string]any, store *DataModelStore, scope map[string]any) (any, error)

// FunctionRegistry manages registered client-side functions for catalog execution.
type FunctionRegistry struct {
	handlers map[string]FunctionHandler
}

// DefaultRegistry returns the default registry pre-loaded with standard v0.9 basic catalog functions.
func DefaultRegistry() *FunctionRegistry {
	r := &FunctionRegistry{
		handlers: make(map[string]FunctionHandler),
	}
	r.Register("formatString", fnFormatString)
	r.Register("concat", fnConcat)
	r.Register("equal", fnEqual)
	r.Register("not", fnNot)
	r.Register("and", fnAnd)
	r.Register("or", fnOr)
	r.Register("greaterThan", fnGreaterThan)
	r.Register("lessThan", fnLessThan)
	return r
}

// Register registers a function handler by name.
func (r *FunctionRegistry) Register(name string, h FunctionHandler) {
	r.handlers[strings.ToLower(name)] = h
}

// Execute evaluates a FunctionCall against the data store and optional scope.
func (r *FunctionRegistry) Execute(fc *tmca2ui.FunctionCall, store *DataModelStore, scope map[string]any) (any, error) {
	if fc == nil {
		return nil, fmt.Errorf("nil function call")
	}
	handler, ok := r.handlers[strings.ToLower(fc.Call)]
	if !ok {
		return nil, fmt.Errorf("unsupported client function %q", fc.Call)
	}
	return handler(fc.Args, store, scope)
}

// ResolveValue resolves an argument which might be a literal, a binding map ({"path": "..."}), or a nested FunctionCall.
func ResolveValue(val any, store *DataModelStore, scope map[string]any) any {
	if val == nil {
		return nil
	}
	if ds, ok := val.(*tmca2ui.DynamicString); ok {
		if s, ok := ResolveDynamicString(ds, store, scope); ok {
			return s
		}
	}
	if ds, ok := val.(tmca2ui.DynamicString); ok {
		if s, ok := ResolveDynamicString(&ds, store, scope); ok {
			return s
		}
	}
	if m, ok := val.(map[string]any); ok {
		// Check for data binding: {"path": "/..."}
		if path, hasPath := m["path"].(string); hasPath {
			if scope != nil {
				clean := strings.TrimPrefix(path, "/")
				if v, exists := scope[clean]; exists {
					return v
				}
				if v, exists := scope[path]; exists {
					return v
				}
			}
			if store != nil {
				if v, exists := store.Get(path); exists {
					return v
				}
			}
			return ""
		}
		// Check for nested function call: {"call": "...", "args": {...}}
		if callName, hasCall := m["call"].(string); hasCall {
			args, _ := m["args"].(map[string]any)
			fc := &tmca2ui.FunctionCall{Call: callName, Args: args}
			res, err := DefaultRegistry().Execute(fc, store, scope)
			if err == nil {
				return res
			}
		}
	}
	return val
}

func fnFormatString(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	if args == nil {
		return "", nil
	}
	tplVal, ok := args["template"]
	if !ok {
		return "", fmt.Errorf("formatString missing 'template' argument")
	}
	templateStr := fmt.Sprintf("%v", ResolveValue(tplVal, store, scope))

	// 1. Interpolate named variables ${name} or ${/user/name}
	result := namedVarPattern.ReplaceAllStringFunc(templateStr, func(match string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		// Check args map directly
		if directVal, exists := args[varName]; exists {
			resolved := ResolveValue(directVal, store, scope)
			return fmt.Sprintf("%v", resolved)
		}
		// Check scope
		if scope != nil {
			if sVal, exists := scope[varName]; exists {
				return fmt.Sprintf("%v", sVal)
			}
			clean := strings.TrimPrefix(varName, "/")
			if sVal, exists := scope[clean]; exists {
				return fmt.Sprintf("%v", sVal)
			}
		}
		// Check data store
		if store != nil {
			path := varName
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if sVal, exists := store.Get(path); exists {
				return fmt.Sprintf("%v", sVal)
			}
		}
		return match
	})

	// 2. Interpolate indexed variables {0}, {1} from "arguments" or "args" array if present
	var arrayArgs []any
	if rawList, exists := args["arguments"]; exists {
		if l, ok := rawList.([]any); ok {
			arrayArgs = l
		}
	} else if rawList, exists := args["args"]; exists {
		if l, ok := rawList.([]any); ok {
			arrayArgs = l
		}
	}

	if len(arrayArgs) > 0 {
		result = indexedVarPattern.ReplaceAllStringFunc(result, func(match string) string {
			idxStr := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
			idx, err := strconv.Atoi(idxStr)
			if err == nil && idx >= 0 && idx < len(arrayArgs) {
				resolved := ResolveValue(arrayArgs[idx], store, scope)
				return fmt.Sprintf("%v", resolved)
			}
			return match
		})
	}

	return result, nil
}

func fnConcat(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	var sb strings.Builder
	if list, ok := args["strings"].([]any); ok {
		for _, item := range list {
			sb.WriteString(fmt.Sprintf("%v", ResolveValue(item, store, scope)))
		}
		return sb.String(), nil
	}
	if list, ok := args["values"].([]any); ok {
		for _, item := range list {
			sb.WriteString(fmt.Sprintf("%v", ResolveValue(item, store, scope)))
		}
		return sb.String(), nil
	}
	return "", nil
}

func fnEqual(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	left := ResolveValue(args["left"], store, scope)
	right := ResolveValue(args["right"], store, scope)
	return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right), nil
}

func fnNot(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	val := ResolveValue(args["value"], store, scope)
	if b, ok := val.(bool); ok {
		return !b, nil
	}
	if s, ok := val.(string); ok {
		return s == "" || s == "false", nil
	}
	return val == nil, nil
}

func fnAnd(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	if values, ok := args["values"].([]any); ok {
		for _, item := range values {
			v := ResolveValue(item, store, scope)
			if b, ok := v.(bool); ok && !b {
				return false, nil
			}
		}
		return true, nil
	}
	left, _ := ResolveValue(args["left"], store, scope).(bool)
	right, _ := ResolveValue(args["right"], store, scope).(bool)
	return left && right, nil
}

func fnOr(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	if values, ok := args["values"].([]any); ok {
		for _, item := range values {
			v := ResolveValue(item, store, scope)
			if b, ok := v.(bool); ok && b {
				return true, nil
			}
		}
		return false, nil
	}
	left, _ := ResolveValue(args["left"], store, scope).(bool)
	right, _ := ResolveValue(args["right"], store, scope).(bool)
	return left || right, nil
}

func fnGreaterThan(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	leftNum := toFloat(ResolveValue(args["left"], store, scope))
	rightNum := toFloat(ResolveValue(args["right"], store, scope))
	return leftNum > rightNum, nil
}

func fnLessThan(args map[string]any, store *DataModelStore, scope map[string]any) (any, error) {
	leftNum := toFloat(ResolveValue(args["left"], store, scope))
	rightNum := toFloat(ResolveValue(args["right"], store, scope))
	return leftNum < rightNum, nil
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

// ResolveDynamicString resolves a DynamicString using literal, binding, or function execution.
func ResolveDynamicString(ds *tmca2ui.DynamicString, store *DataModelStore, scope map[string]any) (string, bool) {
	if ds == nil {
		return "", false
	}
	if ds.Literal != nil {
		return *ds.Literal, true
	}
	if ds.Binding != nil {
		if scope != nil {
			clean := strings.TrimPrefix(ds.Binding.Path, "/")
			if v, ok := scope[clean]; ok {
				return fmt.Sprintf("%v", v), true
			}
			if v, ok := scope[ds.Binding.Path]; ok {
				return fmt.Sprintf("%v", v), true
			}
		}
		if store != nil {
			if v, ok := store.Get(ds.Binding.Path); ok {
				return fmt.Sprintf("%v", v), true
			}
		}
		return "", false
	}
	if ds.FunctionCall != nil {
		res, err := DefaultRegistry().Execute(ds.FunctionCall, store, scope)
		if err == nil && res != nil {
			return fmt.Sprintf("%v", res), true
		}
	}
	return "", false
}

// ResolveDynamicBoolean resolves a DynamicBoolean using literal, binding, or function execution.
func ResolveDynamicBoolean(db *tmca2ui.DynamicBoolean, store *DataModelStore, scope map[string]any) (bool, bool) {
	if db == nil {
		return false, false
	}
	if db.Literal != nil {
		return *db.Literal, true
	}
	if db.Binding != nil {
		if scope != nil {
			clean := strings.TrimPrefix(db.Binding.Path, "/")
			if v, ok := scope[clean]; ok {
				if b, ok := v.(bool); ok {
					return b, true
				}
			}
		}
		if store != nil {
			if v, ok := store.Get(db.Binding.Path); ok {
				if b, ok := v.(bool); ok {
					return b, true
				}
				if s, ok := v.(string); ok {
					return s == "true", true
				}
			}
		}
		return false, false
	}
	if db.FunctionCall != nil {
		res, err := DefaultRegistry().Execute(db.FunctionCall, store, scope)
		if err == nil {
			if b, ok := res.(bool); ok {
				return b, true
			}
		}
	}
	return false, false
}

// ResolveDynamicNumber resolves a DynamicNumber using literal, binding, or function execution.
func ResolveDynamicNumber(dn *tmca2ui.DynamicNumber, store *DataModelStore, scope map[string]any) (float64, bool) {
	if dn == nil {
		return 0, false
	}
	if dn.Literal != nil {
		return *dn.Literal, true
	}
	if dn.Binding != nil {
		if scope != nil {
			clean := strings.TrimPrefix(dn.Binding.Path, "/")
			if v, ok := scope[clean]; ok {
				return toFloat(v), true
			}
		}
		if store != nil {
			if v, ok := store.Get(dn.Binding.Path); ok {
				return toFloat(v), true
			}
		}
		return 0, false
	}
	if dn.FunctionCall != nil {
		res, err := DefaultRegistry().Execute(dn.FunctionCall, store, scope)
		if err == nil && res != nil {
			return toFloat(res), true
		}
	}
	return 0, false
}

// ResolveDynamicStringList resolves a DynamicStringList using literal, binding, or function execution.
func ResolveDynamicStringList(dsl *tmca2ui.DynamicStringList, store *DataModelStore, scope map[string]any) ([]string, bool) {
	if dsl == nil {
		return nil, false
	}
	if len(dsl.Literal) > 0 {
		return dsl.Literal, true
	}
	if dsl.Binding != nil {
		if store != nil {
			if v, ok := store.Get(dsl.Binding.Path); ok {
				if sl, ok := v.([]string); ok {
					return sl, true
				}
				if anyList, ok := v.([]any); ok {
					var res []string
					for _, item := range anyList {
						res = append(res, fmt.Sprintf("%v", item))
					}
					return res, true
				}
			}
		}
	}
	return nil, false
}
