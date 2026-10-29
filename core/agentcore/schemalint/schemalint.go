// Package schemalint statically checks tool argument schemas against the two
// consumers that actually interpret them at runtime:
//
//   - the call-time validator (agentcore.validateToolArgs → validateSchemaValue):
//     it enforces type, enum, required, properties, additionalProperties and
//     items — and silently ignores malformed schema fragments or keywords it
//     does not implement, so a schema can claim guarantees it never enforces;
//   - provider strict mode (openai.normalizeStrictSchema): when a tool opts in
//     via StrictSchemaTool, the adapter rejects schemas with const anywhere,
//     with additionalProperties set to anything but false on an object node,
//     or with a properties entry that is not listed in required. Unsupported
//     constraint keywords (minLength, pattern, ...) are forwarded and the
//     provider rejects them server-side.
//
// The lint encodes only what that code enforces — a schema that never rejects
// calls or that a strict request would refuse is a defect worth flagging at
// test time instead of burning model turns at run time.
package schemalint

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/datnt1998/verse/core/agentcore"
)

// Issue describes one schema defect found by Check / CheckTool.
type Issue struct {
	Tool  string // tool name; empty for Check (use CheckTool or join caller-side)
	Path  string // schema path, e.g. "schema.properties.issues.items.properties.quote"
	Rule  string // stable rule identifier, e.g. "required.unknown_property"
	Why   string // human-readable explanation
	Field string // offending value rendered for messages (optional context)
}

func (i Issue) String() string {
	where := i.Path
	if i.Tool != "" {
		where = i.Tool + ": " + where
	}
	return fmt.Sprintf("%s [%s] %s", where, i.Rule, i.Why)
}

// Check lints one schema. strict applies the extra provider-strict rules a
// tool opting into StrictSchemaTool must satisfy; pass false for tools that
// do not implement it.
func Check(schema map[string]any, strict bool) []Issue {
	var out []Issue
	walk(schema, "schema", strict, &out)
	return out
}

// CheckTool lints a registered tool's schema. Strict rules apply when the
// tool implements agentcore.StrictSchemaTool and returns true.
func CheckTool(tool agentcore.Tool) []Issue {
	if tool == nil {
		return nil
	}
	strict := false
	if s, ok := tool.(agentcore.StrictSchemaTool); ok {
		strict = s.StrictSchema()
	}
	issues := Check(tool.Schema(), strict)
	for i := range issues {
		issues[i].Tool = tool.Name()
	}
	return issues
}

// jsonTypes is the set the call-time validator's type names can express.
var jsonTypes = map[string]bool{
	"object": true, "array": true, "string": true,
	"integer": true, "number": true, "boolean": true, "null": true,
}

// ignoredKeywords are schema keywords the call-time validator does not
// implement: presence is silently dropped, so a schema relying on them
// enforces nothing. const additionally breaks strict normalization, and the
// combinators/ref forms are outside the strict subset providers accept.
var ignoredKeywords = []string{
	"const", "anyOf", "oneOf", "allOf", "not", "$ref", "$defs", "definitions",
	"if", "then", "else", "dependentRequired", "dependentSchemas",
	"patternProperties", "propertyNames", "contains",
	"minLength", "maxLength", "pattern", "format",
	"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf",
	"minItems", "maxItems", "uniqueItems",
	"minProperties", "maxProperties",
}

func walk(schema map[string]any, path string, strict bool, out *[]Issue) {
	if schema == nil {
		return
	}
	report := func(rule, why string) {
		*out = append(*out, Issue{Path: path, Rule: rule, Why: why})
	}

	types, typesOK := schemaTypeNames(schema["type"])
	if schema["type"] != nil && !typesOK {
		report("type.invalid", "type must be a JSON type name or a non-empty list of names")
	}
	for _, name := range types {
		if !jsonTypes[name] {
			report("type.unknown", fmt.Sprintf("type %q is not a JSON type the validator understands", name))
		}
	}
	isObject := contains(types, "object")
	isArray := contains(types, "array")

	for _, kw := range ignoredKeywords {
		if _, present := schema[kw]; present {
			report("keyword.ignored", fmt.Sprintf("keyword %q is not enforced by the call-time validator and is outside the strict subset", kw))
		}
	}

	// properties: must be a map of subschemas; non-map entries make the
	// validator skip the property entirely.
	properties, propsOK := schema["properties"].(map[string]any)
	if _, present := schema["properties"]; present && !propsOK {
		report("properties.invalid", "properties must be an object of subschemas")
	}
	propNames := sortedKeys(properties)

	// required: must be a list of strings naming declared properties. The
	// validator accepts any list and then reports every call as missing the
	// undeclared name — a schema that can never pass cleanly.
	required, requiredOK := schemaStringList(schema["required"])
	if _, present := schema["required"]; present && !requiredOK {
		report("required.invalid", "required must be a list of property names")
	}
	for _, name := range required {
		if _, exists := properties[name]; !exists {
			report("required.unknown_property", fmt.Sprintf("required names %q which is not in properties; every call would fail IssueMissing", name))
		}
	}

	// enum: must be a non-empty list; each value must match the declared
	// type or every call fails IssueValue.
	if raw, present := schema["enum"]; present {
		values, enumOK := schemaEnumList(raw)
		switch {
		case !enumOK:
			report("enum.invalid", "enum must be a list of values")
		case len(values) == 0:
			report("enum.empty", "enum is empty; every call would fail IssueValue")
		default:
			for i, v := range values {
				if typesOK && len(types) > 0 && !matchesAnyType(v, types) {
					report("enum.item_type", fmt.Sprintf("enum[%d] has JSON type %q which never satisfies type %q", i, jsonTypeOf(v), strings.Join(types, "|")))
				}
			}
		}
	}

	// items: the validator recurses only when items is an object schema.
	// An array without a usable items schema validates nothing about its
	// elements, and strict providers reject it.
	if raw, present := schema["items"]; present {
		if itemSchema, ok := raw.(map[string]any); ok {
			walk(itemSchema, path+".items", strict, out)
		} else {
			report("items.invalid", "items must be an object schema")
		}
	} else if isArray {
		report("items.missing", "array schema has no items; element validation is skipped")
	}

	// additionalProperties: the validator honours false (reject unknown
	// keys) and an object (validate unknown keys against it); strict mode
	// requires exactly false — anything else the request would refuse.
	switch additional := schema["additionalProperties"].(type) {
	case nil:
		// Absent: non-strict allows unknown keys; strict normalization
		// supplies false automatically, so no issue.
	case bool:
		if strict && additional {
			report("strict.additional_properties", "strict mode requires additionalProperties:false; true would be rejected at request build")
		}
	case map[string]any:
		if strict {
			report("strict.additional_properties", "strict mode requires additionalProperties:false; a subschema would be rejected at request build")
		}
		walk(additional, path+".additionalProperties", strict, out)
	default:
		report("additional_properties.invalid", "additionalProperties must be false or an object schema")
	}

	if isObject {
		if strict {
			// Provider strict: every declared property must be required;
			// optional is expressed with a null union, not by omission.
			requiredSet := make(map[string]bool, len(required))
			for _, name := range required {
				requiredSet[name] = true
			}
			for _, name := range propNames {
				if !requiredSet[name] {
					report("strict.property_not_required", fmt.Sprintf("property %q is not in required; strict mode rejects optional properties (use a null union)", name))
				}
			}
			if _, present := schema["properties"]; !present {
				report("strict.properties_missing", "strict object has no properties; there is nothing to constrain")
			}
		}
	}

	for _, name := range propNames {
		if child, ok := properties[name].(map[string]any); ok {
			walk(child, path+".properties."+name, strict, out)
		} else {
			report("properties.invalid", fmt.Sprintf("properties.%s must be an object schema", name))
		}
	}
}

// schemaTypeNames mirrors agentcore's schemaTypeNames: string, []string, or
// []any of non-empty strings. Anything else is not a usable type list.
func schemaTypeNames(value any) ([]string, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case string:
		if v == "" {
			return nil, false
		}
		return []string{v}, true
	case []string:
		if len(v) == 0 {
			return nil, false
		}
		for _, s := range v {
			if s == "" {
				return nil, false
			}
		}
		return v, true
	case []any:
		if len(v) == 0 {
			return nil, false
		}
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok || s == "" {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func schemaStringList(value any) ([]string, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func schemaEnumList(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out, true
	default:
		return nil, false
	}
}

// jsonTypeOf mirrors the validator's jsonTypeName for decoded JSON values.
func jsonTypeOf(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64:
		if v == math.Trunc(v) {
			return "integer"
		}
		return "number"
	case int:
		return "integer"
	case json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// matchesAnyType applies the validator's rule: a value matches when its JSON
// type name equals a declared name, with integer accepted for number.
func matchesAnyType(value any, types []string) bool {
	actual := jsonTypeOf(value)
	for _, typ := range types {
		if typ == actual || (typ == "number" && actual == "integer") {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
