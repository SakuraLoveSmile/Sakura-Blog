// Package stellarv2 implements the official Stellar 2.0.0 configuration boundary.
// Raw public keys use snake_case; parsed runtime keys follow the upstream schema.
package stellarv2

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
)

// Options selects strict parsing or upstream recovery. Recovery is for presentation
// parameters, not a substitute for a product's explicit legacy-data migration gate.
type Options struct {
	Source  string
	Recover bool
}
type Result struct {
	Value  map[string]any
	Issues []Issue
}
type Issue struct {
	Code       string `json:"code"`
	Source     string `json:"source"`
	Path       string `json:"path"`
	ActualType string `json:"actualType"`
	Expected   string `json:"expected"`
	Migration  any    `json:"migration,omitempty"`
	Severity   string `json:"severity,omitempty"`
	Action     string `json:"action,omitempty"`
}
type SchemaError struct{ Issues []Issue }

func (e *SchemaError) Error() string {
	parts := make([]string, len(e.Issues))
	for i, issue := range e.Issues {
		parts[i] = fmt.Sprintf("%s: %s (%s): expected %s, actual %s", issue.Source, issue.Path, issue.Code, issue.Expected, issue.ActualType)
	}
	return "Stellar v2 schema validation failed: " + strings.Join(parts, "; ")
}

type definition struct {
	Kind  string `json:"kind"`
	Value any    `json:"value"`
}
type node struct {
	Type                   []string         `json:"type"`
	Default                *definition      `json:"default"`
	Normalizer             string           `json:"normalizer"`
	RuntimeKey             string           `json:"runtimeKey"`
	Validator              string           `json:"validator"`
	Migration              any              `json:"migration"`
	Values                 []any            `json:"values"`
	Minimum                *float64         `json:"minimum"`
	Maximum                *float64         `json:"maximum"`
	ExclusiveMinimum       *float64         `json:"exclusiveMinimum"`
	Properties             map[string]*node `json:"properties"`
	PropertyOrder          []string         `json:"propertyOrder"`
	Items                  *node            `json:"items"`
	AdditionalProperties   *node            `json:"additionalProperties"`
	RemovedProperties      map[string]any   `json:"removedProperties"`
	RequiredProperties     []string         `json:"requiredProperties"`
	ExternalProperties     []string         `json:"externalProperties"`
	AllowedPropertyKeys    []string         `json:"allowedPropertyKeys"`
	Sealed                 bool             `json:"sealed"`
	ValidateInParameterBag bool             `json:"validateInParameterBag"`
}
type effectOption struct {
	Type     string `json:"type"`
	Values   []any  `json:"values"`
	MinItems *int   `json:"minItems"`
}
type effectDefinition struct {
	ID      string                  `json:"id"`
	Label   string                  `json:"label"`
	Options map[string]effectOption `json:"options"`
}
type registry struct {
	Theme       *node              `json:"theme"`
	Page        *node              `json:"page"`
	Collection  *node              `json:"collection"`
	RawDefaults map[string]any     `json:"rawDefaults"`
	Effects     []effectDefinition `json:"effects"`
}

//go:embed schema.json
var schemaData []byte
var schemas = func() registry {
	var r registry
	if err := json.Unmarshal(schemaData, &r); err != nil {
		panic(err)
	}
	return r
}()

// ParseTheme merges the complete official defaults before validation.
func ParseTheme(input any, options Options) (Result, error) {
	if options.Source == "" {
		options.Source = "_config.stellar.yml"
	}
	input = clone(input)
	if raw, ok := input.(map[string]any); ok {
		if katex, ok := atObject(raw, "features.math.katex"); ok {
			css, exists := katex["css"]
			defaults, _ := Get(schemas.RawDefaults, "features.math.katex.css")
			if exists && css != nil && !reflect.DeepEqual(css, defaults) {
				if _, exists := katex["css_integrity"]; !exists {
					katex["css_integrity"] = nil
				}
			}
		}
	}
	return parse(schemas.Theme, input, options, true, true, false)
}
func ParsePage(input any, options Options) (Result, error) {
	if options.Source == "" {
		options.Source = "<page>"
	}
	return parse(schemas.Page, clone(input), options, false, false, true)
}
func ParseCollection(input any, options Options) (Result, error) {
	if options.Source == "" {
		options.Source = "<collection>"
	}
	result, err := parse(schemas.Collection, clone(input), options, false, false, false)
	if err != nil {
		return result, err
	}
	name, ok := result.Value["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		id := options.Source[strings.LastIndex(strings.ReplaceAll(options.Source, "\\", "/"), "/")+1:]
		id = replacePattern(`(?i)\.ya?ml$`, id, "")
		result.Issues = append(result.Issues, Issue{Code: "invalid_value", Source: options.Source, Path: "name", ActualType: typeOf(valueOrUndefined(result.Value, "name")), Expected: "non-empty display name", Severity: "warning", Action: "使用 Collection ID"})
		result.Value["name"] = id
	}
	return result, nil
}
func DefaultTheme() map[string]any {
	result, err := ParseTheme(map[string]any{}, Options{})
	if err != nil {
		panic(err)
	}
	return result.Value
}

// Get retrieves a runtime camelCase path; map keys containing dots remain exact
// keys and should be retrieved from their parent map.
func Get(object map[string]any, path string) (any, bool) {
	if path == "" {
		return object, true
	}
	var current any = object
	for _, key := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
func atObject(object map[string]any, path string) (map[string]any, bool) {
	v, ok := Get(object, path)
	m, good := v.(map[string]any)
	return m, ok && good
}

type undefinedValue struct{}

var undefined = undefinedValue{}

func valueOrUndefined(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	return undefined
}
func typeOf(value any) string {
	switch value.(type) {
	case undefinedValue:
		return "undefined"
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	if _, ok := number(value); ok {
		return "number"
	}
	return reflect.TypeOf(value).String()
}
func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	}
	return 0, false
}
func clone(value any) any {
	if value == nil {
		return nil
	}
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Map && (v.Type().Key().Kind() == reflect.String || v.Type().Key().Kind() == reflect.Interface) {
		result := map[string]any{}
		iter := v.MapRange()
		for iter.Next() {
			key, ok := iter.Key().Interface().(string)
			if !ok {
				return value
			}
			result[key] = clone(iter.Value().Interface())
		}
		return result
	}
	if v.Kind() == reflect.Array || v.Kind() == reflect.Slice {
		result := make([]any, v.Len())
		for i := range result {
			result[i] = clone(v.Index(i).Interface())
		}
		return result
	}
	return value
}
func merge(base, override map[string]any) map[string]any {
	result := clone(base).(map[string]any)
	for key, value := range override {
		a, aok := result[key].(map[string]any)
		b, bok := value.(map[string]any)
		if aok && bok {
			result[key] = merge(a, b)
		} else {
			result[key] = clone(value)
		}
	}
	return result
}
func includes[T comparable](list []T, value T) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
func equalValue(a, b any) bool {
	an, aok := number(a)
	bn, bok := number(b)
	if aok && bok {
		return an == bn
	}
	return reflect.DeepEqual(a, b)
}
func valuesInclude(list []any, value any) bool {
	for _, item := range list {
		if equalValue(item, value) {
			return true
		}
	}
	return false
}
func keys(m map[string]any) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
func dot(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
func literalDefault(n *node) any {
	if n.Default != nil && n.Default.Kind == "literal" {
		return clone(n.Default.Value)
	}
	for _, t := range []string{"null", "string", "array", "object", "boolean", "number"} {
		if includes(n.Type, t) {
			switch t {
			case "null":
				return nil
			case "string":
				return ""
			case "array":
				return []any{}
			case "object":
				return map[string]any{}
			case "boolean":
				return false
			case "number":
				return float64(0)
			}
		}
	}
	return undefined
}

type parser struct {
	source   string
	defaults bool
	issues   []Issue
}

func (p *parser) issue(n *node, code, path string, input any, expected string) {
	p.issues = append(p.issues, Issue{Code: code, Source: p.source, Path: path, ActualType: typeOf(input), Expected: expected, Migration: n.Migration})
}
func expectedValues(values []any) string {
	r := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			r[i] = "null"
		} else {
			r[i] = fmt.Sprint(v)
		}
	}
	return strings.Join(r, " | ")
}
func (p *parser) node(n *node, input any, path string) any {
	if input == nil && !includes(n.Type, "null") && p.defaults {
		input = literalDefault(n)
	}
	if n.Normalizer == "leftbar_background_type" && typeOf(input) == "string" && !valuesInclude(n.Values, input) {
		input = literalDefault(n)
	}
	if !includes(n.Type, typeOf(input)) {
		field := path
		if field == "" {
			field = "root"
		}
		p.issue(n, "invalid_type", field, input, strings.Join(n.Type, " | "))
		return undefined
	}
	if n.Values != nil && !valuesInclude(n.Values, input) {
		p.issue(n, "invalid_value", path, input, expectedValues(n.Values))
		return undefined
	}
	if v, ok := number(input); ok {
		expected := ""
		switch {
		case math.IsNaN(v) || math.IsInf(v, 0):
			expected = "finite number"
		case n.Minimum != nil && v < *n.Minimum:
			expected = fmt.Sprintf("number >= %v", *n.Minimum)
		case n.Maximum != nil && v > *n.Maximum:
			expected = fmt.Sprintf("number <= %v", *n.Maximum)
		case n.ExclusiveMinimum != nil && v <= *n.ExclusiveMinimum:
			expected = fmt.Sprintf("number > %v", *n.ExclusiveMinimum)
		}
		if expected != "" {
			p.issue(n, "invalid_value", path, input, expected)
			return undefined
		}
	}
	p.validate(n, input, path)
	if items, ok := input.([]any); ok && n.Items != nil {
		result := make([]any, 0, len(items))
		valid := true
		for i, value := range items {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if !includes(n.Items.Type, typeOf(value)) {
				p.issue(n, "invalid_type", itemPath, value, strings.Join(n.Items.Type, " | "))
				valid = false
				continue
			}
			parsed := value
			if n.Items.Properties != nil || n.Items.AdditionalProperties != nil || n.Items.Normalizer != "" {
				parsed = p.node(n.Items, value, itemPath)
			}
			if _, bad := parsed.(undefinedValue); bad {
				valid = false
			} else {
				result = append(result, clone(parsed))
			}
		}
		if !valid {
			return undefined
		}
		return normalize(n, result)
	}
	m, ok := input.(map[string]any)
	if !ok {
		return normalize(n, input)
	}
	if p.defaults && n.Default != nil && n.Default.Kind == "literal" {
		if base, ok := n.Default.Value.(map[string]any); ok {
			m = merge(base, m)
		}
	}
	for _, key := range keys(m) {
		if replacement, exists := n.RemovedProperties[key]; exists {
			target := "remove field without replacement"
			if replacement != nil {
				target = dot(path, fmt.Sprint(replacement))
			}
			p.issue(n, "removed_field", dot(path, key), m[key], target)
		} else if includes(n.ExternalProperties, key) {
			continue
		} else if n.AllowedPropertyKeys != nil && !includes(n.AllowedPropertyKeys, key) {
			p.issue(n, "unknown_field", dot(path, key), m[key], strings.Join(n.AllowedPropertyKeys, " | "))
		} else if n.Sealed && n.AdditionalProperties == nil && n.Properties[key] == nil {
			p.issue(n, "unknown_field", dot(path, key), m[key], "known field")
		}
	}
	if n.Normalizer == "parameter_bag" {
		for _, key := range n.PropertyOrder {
			child := n.Properties[key]
			if child.ValidateInParameterBag {
				if v, exists := m[key]; exists {
					p.node(child, v, dot(path, key))
				}
			}
		}
		return normalize(n, m)
	}
	if n.Normalizer == "effect" {
		return normalize(n, m)
	}
	result := map[string]any{}
	for _, key := range n.ExternalProperties {
		if v, exists := m[key]; exists {
			result[key] = clone(v)
		}
	}
	for _, key := range n.RequiredProperties {
		if _, exists := m[key]; !exists {
			child := n.Properties[key]
			if child == nil {
				child = n
			}
			migration := child.Migration
			if migration == nil {
				migration = n.Migration
			}
			p.issues = append(p.issues, Issue{Code: "missing_field", Source: p.source, Path: dot(path, key), ActualType: "undefined", Expected: strings.Join(child.Type, " | "), Migration: migration})
		}
	}
	for _, key := range n.PropertyOrder {
		child := n.Properties[key]
		value, exists := m[key]
		if !exists && !p.defaults {
			continue
		}
		if !exists {
			value = literalDefault(child)
		}
		parsed := p.node(child, value, dot(path, key))
		if _, bad := parsed.(undefinedValue); !bad {
			runtimeKey := child.RuntimeKey
			if runtimeKey == "" {
				runtimeKey = key
			}
			result[runtimeKey] = parsed
		}
	}
	if n.AdditionalProperties != nil {
		for _, key := range keys(m) {
			if n.Properties[key] != nil {
				continue
			}
			parsed := p.node(n.AdditionalProperties, m[key], dot(path, key))
			if _, bad := parsed.(undefinedValue); !bad {
				result[key] = parsed
			}
		}
	}
	if includes([]string{"menu_item", "footer_action", "footer_action_item"}, n.Normalizer) {
		return normalize(n, result)
	}
	return result
}
