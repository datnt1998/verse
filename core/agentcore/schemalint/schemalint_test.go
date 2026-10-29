package schemalint

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/core/agentcore"
)

// wantRule asserts the first defect a row triggers, identified by rule id so
// message wording can evolve freely.
type lintCase struct {
	name     string
	schema   map[string]any
	strict   bool
	wantRule string // "" means clean
	wantPath string // substring of Issue.Path; "" skips path check
}

func lintSchemaCases() []lintCase {
	obj := func(props map[string]any, required ...string) map[string]any {
		m := map[string]any{"type": "object"}
		if props != nil {
			m["properties"] = props
		}
		if required != nil {
			m["required"] = required
		}
		return m
	}

	return []lintCase{
		{
			name: "clean optional object",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"chapter": map[string]any{"type": "integer"},
					"mode":    map[string]any{"type": "string", "enum": []any{"a", "b"}},
					"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
				"required": []any{"chapter"},
			},
		},
		{
			name: "clean strict object with null union",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"chapter": map[string]any{"type": "integer"},
					"note":    map[string]any{"type": []any{"string", "null"}, "enum": []any{"x", "y", nil}},
				},
				"required":             []any{"chapter", "note"},
				"additionalProperties": false,
			},
			strict: true,
		},
		{
			name:     "required names missing property",
			schema:   obj(map[string]any{"kind": map[string]any{"type": "string"}}, "kind", "axes"),
			wantRule: "required.unknown_property",
			wantPath: "schema",
		},
		{
			name:     "required not a list",
			schema:   map[string]any{"type": "object", "properties": map[string]any{}, "required": "chapter"},
			wantRule: "required.invalid",
		},
		{
			name:     "properties not an object",
			schema:   map[string]any{"type": "object", "properties": []any{"chapter"}},
			wantRule: "properties.invalid",
		},
		{
			name: "nested required missing property is flagged",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"issues": map[string]any{
						"type":  "array",
						"items": obj(map[string]any{"sev": map[string]any{"type": "string"}}, "sev", "quote"),
					},
				},
			},
			wantRule: "required.unknown_property",
			wantPath: "schema.properties.issues.items",
		},
		{
			name:     "unknown type name",
			schema:   obj(map[string]any{"n": map[string]any{"type": "float"}}, "n"),
			wantRule: "type.unknown",
			wantPath: "schema.properties.n",
		},
		{
			name:     "empty enum",
			schema:   obj(map[string]any{"m": map[string]any{"type": "string", "enum": []any{}}}, "m"),
			wantRule: "enum.empty",
		},
		{
			name:     "enum item mismatched type",
			schema:   obj(map[string]any{"m": map[string]any{"type": "string", "enum": []any{"a", 3.0}}}, "m"),
			wantRule: "enum.item_type",
		},
		{
			name:     "enum item integer matches declared number",
			schema:   obj(map[string]any{"m": map[string]any{"type": "number", "enum": []any{1.0, 2.5}}}, "m"),
			wantRule: "",
		},
		{
			name:     "enum accepts null member under null union",
			schema:   obj(map[string]any{"m": map[string]any{"type": []any{"integer", "null"}, "enum": []any{1.0, nil}}}, "m"),
			wantRule: "",
		},
		{
			name:     "array without items",
			schema:   obj(map[string]any{"ids": map[string]any{"type": "array"}}, "ids"),
			wantRule: "items.missing",
			wantPath: "schema.properties.ids",
		},
		{
			name:     "items not an object schema",
			schema:   obj(map[string]any{"ids": map[string]any{"type": "array", "items": "string"}}, "ids"),
			wantRule: "items.invalid",
		},
		{
			name:     "additionalProperties non-bool non-object",
			schema:   map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": "yes"},
			wantRule: "additional_properties.invalid",
		},
		{
			name:     "additionalProperties schema is walked",
			schema:   map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": map[string]any{"type": "unknown_x"}},
			wantRule: "type.unknown",
			wantPath: "schema.additionalProperties",
		},
		{
			name:     "const rejected even without strict",
			schema:   obj(map[string]any{"m": map[string]any{"const": "x"}}, "m"),
			wantRule: "keyword.ignored",
			wantPath: "schema.properties.m",
		},
		{
			name:     "combinator flagged",
			schema:   obj(map[string]any{"v": map[string]any{"anyOf": []any{map[string]any{"type": "string"}}}}, "v"),
			wantRule: "keyword.ignored",
		},
		{
			name:     "constraint keyword flagged",
			schema:   obj(map[string]any{"v": map[string]any{"type": "string", "minLength": 3.0}}, "v"),
			wantRule: "keyword.ignored",
		},
		{
			name:     "strict property missing from required",
			schema:   obj(map[string]any{"a": map[string]any{"type": "string"}, "b": map[string]any{"type": "string"}}, "a"),
			strict:   true,
			wantRule: "strict.property_not_required",
			wantPath: "schema",
		},
		{
			name:   "strict allows absent additionalProperties (normalized to false)",
			schema: obj(map[string]any{"a": map[string]any{"type": "string"}}, "a"),
			strict: true,
		},
		{
			name:     "strict rejects additionalProperties true",
			schema:   map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": true},
			strict:   true,
			wantRule: "strict.additional_properties",
		},
		{
			name:     "strict rejects additionalProperties subschema",
			schema:   map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": map[string]any{"type": "string"}},
			strict:   true,
			wantRule: "strict.additional_properties",
		},
		{
			name:     "strict object without properties",
			schema:   map[string]any{"type": "object", "additionalProperties": false},
			strict:   true,
			wantRule: "strict.properties_missing",
		},
		{
			name:   "non-strict object without properties is fine",
			schema: map[string]any{"type": "object"},
		},
		{
			name:   "non-strict additionalProperties true is fine",
			schema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": true},
		},
	}
}

func TestCheckTable(t *testing.T) {
	for _, tc := range lintSchemaCases() {
		t.Run(tc.name, func(t *testing.T) {
			issues := Check(tc.schema, tc.strict)
			if tc.wantRule == "" {
				if len(issues) != 0 {
					t.Fatalf("expected clean schema, got %v", issues)
				}
				return
			}
			if len(issues) == 0 {
				t.Fatalf("expected rule %s, got no issues", tc.wantRule)
			}
			found := false
			for _, iss := range issues {
				if iss.Rule != tc.wantRule {
					continue
				}
				if tc.wantPath != "" && !strings.Contains(iss.Path, tc.wantPath) {
					continue
				}
				found = true
			}
			if !found {
				t.Fatalf("expected issue rule=%s path~%q, got %v", tc.wantRule, tc.wantPath, issues)
			}
		})
	}
}

type stubTool struct {
	name   string
	schema map[string]any
	strict *bool
}

func (s *stubTool) Name() string           { return s.name }
func (s *stubTool) Description() string    { return "stub" }
func (s *stubTool) Schema() map[string]any { return s.schema }
func (s *stubTool) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (s *stubTool) StrictSchema() bool {
	if s.strict == nil {
		return false
	}
	return *s.strict
}

func TestCheckToolAppliesStrictByInterface(t *testing.T) {
	optionalProp := map[string]any{
		"type":       "object",
		"properties": map[string]any{"a": map[string]any{"type": "string"}},
	}

	nonStrict := CheckTool(agentcore.NewFuncTool("plain", "d", optionalProp, func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	}))
	if len(nonStrict) != 0 {
		t.Fatalf("tool without StrictSchemaTool must not get strict rules: %v", nonStrict)
	}

	strictTrue := true
	strictIssues := CheckTool(&stubTool{name: "strict_tool", schema: optionalProp, strict: &strictTrue})
	if len(strictIssues) == 0 || strictIssues[0].Rule != "strict.property_not_required" {
		t.Fatalf("strict tool must flag optional property, got %v", strictIssues)
	}
	if strictIssues[0].Tool != "strict_tool" {
		t.Fatalf("issue must carry tool name, got %+v", strictIssues[0])
	}

	strictFalse := false
	disabled := CheckTool(&stubTool{name: "off", schema: optionalProp, strict: &strictFalse})
	if len(disabled) != 0 {
		t.Fatalf("StrictSchema()==false must skip strict rules, got %v", disabled)
	}
}
