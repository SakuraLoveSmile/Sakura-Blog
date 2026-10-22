package stellarv2

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func parse(schema *node, input any, options Options, defaults, semantics, page bool) (Result, error) {
	candidate := clone(input)
	diagnostics := []Issue{}
	for attempt := 0; attempt < 20; attempt++ {
		p := parser{source: options.Source, defaults: defaults, issues: []Issue{}}
		parsed := p.node(schema, candidate, "")
		if len(p.issues) == 0 && semantics {
			p.semantic(parsed.(map[string]any))
		}
		if len(p.issues) == 0 {
			return Result{Value: parsed.(map[string]any), Issues: uniqueIssues(diagnostics)}, nil
		}
		if !options.Recover {
			return Result{Issues: p.issues}, &SchemaError{Issues: p.issues}
		}
		fatal, recoverable := []Issue{}, []Issue{}
		for _, issue := range p.issues {
			if issue.Path == "root" || (page && (issue.Path == "collection.profile" || issue.Path == "collection.id")) {
				fatal = append(fatal, issue)
			} else {
				recoverable = append(recoverable, issue)
			}
		}
		if len(recoverable) > 0 {
			var changed bool
			var current []Issue
			candidate, changed, current = recoverInput(candidate, recoverable)
			diagnostics = append(diagnostics, current...)
			if changed {
				continue
			}
		}
		if len(fatal) > 0 {
			return Result{Issues: append(uniqueIssues(diagnostics), fatal...)}, &SchemaError{Issues: fatal}
		}
		return Result{Issues: append(uniqueIssues(diagnostics), p.issues...)}, &SchemaError{Issues: p.issues}
	}
	issue := Issue{Code: "invalid_value", Source: options.Source, Path: "root", ActualType: "object", Expected: "recoverable configuration"}
	return Result{Issues: []Issue{issue}}, &SchemaError{Issues: []Issue{issue}}
}
func (p *parser) semantic(config map[string]any) {
	profiles, _ := config["profiles"].(map[string]any)
	for _, profile := range keys(profiles) {
		definition, _ := profiles[profile].(map[string]any)
		active, _ := Get(definition, "activeMenu")
		if active == nil {
			continue
		}
		ids := map[string]bool{}
		for _, region := range []string{"topbar", "leftbar"} {
			menu, _ := Get(definition, region+".menu")
			items, ok := menu.([]any)
			if !ok {
				menu, _ = Get(config, region+".menu")
				items, _ = menu.([]any)
			}
			for _, item := range items {
				m, _ := item.(map[string]any)
				if id, ok := m["id"].(string); ok {
					ids[id] = true
				}
			}
		}
		if !ids[active.(string)] {
			p.issues = append(p.issues, Issue{Code: "invalid_value", Source: p.source, Path: "profiles." + profile + ".active_menu", ActualType: "string", Expected: "id present in topbar.menu or leftbar.menu"})
		}
	}
}

type token struct {
	key   string
	index int
	array bool
}

func pathTokens(path string) []token {
	result := []token{}
	for _, match := range regexp.MustCompile(`(?:^|\.)([^.\[]+)|\[(\d+)\]`).FindAllStringSubmatch(path, -1) {
		if match[1] != "" {
			result = append(result, token{key: match[1]})
		} else {
			i, _ := strconv.Atoi(match[2])
			result = append(result, token{index: i, array: true})
		}
	}
	return result
}
func getTokens(value any, tokens []token) (any, bool) {
	for _, t := range tokens {
		if t.array {
			a, ok := value.([]any)
			if !ok || t.index < 0 || t.index >= len(a) {
				return nil, false
			}
			value = a[t.index]
		} else {
			m, ok := value.(map[string]any)
			if !ok {
				return nil, false
			}
			value, ok = m[t.key]
			if !ok {
				return nil, false
			}
		}
	}
	return value, true
}
func inputTokens(root any, tokens []token) []token {
	result := []token{}
	current := root
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if next, ok := getTokens(current, []token{t}); ok {
			result = append(result, t)
			current = next
			continue
		}
		matched := false
		if !t.array {
			if m, ok := current.(map[string]any); ok {
				for end := len(tokens); end > i+1; end-- {
					parts := []string{}
					hasArray := false
					for _, part := range tokens[i:end] {
						if part.array {
							hasArray = true
							break
						}
						parts = append(parts, part.key)
					}
					if hasArray {
						continue
					}
					joined := strings.Join(parts, ".")
					if v, exists := m[joined]; exists {
						result = append(result, token{key: joined})
						current = v
						i = end - 1
						matched = true
						break
					}
				}
			}
		}
		if matched {
			continue
		}
		return append(result, tokens[i:]...)
	}
	return result
}

// deleteTokens returns a replacement slice because deleting an array item changes
// its length. Parents keep that replacement instead of aliasing an old header.
func deleteTokens(root any, tokens []token) (any, bool) {
	if len(tokens) == 0 {
		return root, false
	}
	t := tokens[0]
	if t.array {
		a, ok := root.([]any)
		if !ok || t.index < 0 || t.index >= len(a) {
			return root, false
		}
		if len(tokens) == 1 {
			return append(a[:t.index], a[t.index+1:]...), true
		}
		next, changed := deleteTokens(a[t.index], tokens[1:])
		a[t.index] = next
		return a, changed
	}
	m, ok := root.(map[string]any)
	if !ok {
		return root, false
	}
	v, exists := m[t.key]
	if !exists {
		return root, false
	}
	if len(tokens) == 1 {
		delete(m, t.key)
		return m, true
	}
	next, changed := deleteTokens(v, tokens[1:])
	m[t.key] = next
	return m, changed
}
func snake(key string) string {
	return regexp.MustCompile(`[A-Z]`).ReplaceAllStringFunc(key, func(s string) string { return "_" + strings.ToLower(s) })
}
func setTokens(root any, tokens []token, value any) bool {
	if len(tokens) == 0 {
		return false
	}
	m, ok := root.(map[string]any)
	if !ok {
		return false
	}
	for _, t := range tokens[:len(tokens)-1] {
		key := snake(t.key)
		next, ok := m[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[key] = next
		}
		m = next
	}
	m[snake(tokens[len(tokens)-1].key)] = value
	return true
}
func arrayIndex(tokens []token) int {
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].array {
			return i
		}
	}
	return -1
}
func tokenKey(tokens []token) string {
	r := []string{}
	for _, t := range tokens {
		if t.array {
			r = append(r, fmt.Sprintf("[%d]", t.index))
		} else {
			r = append(r, strconv.Quote(t.key))
		}
	}
	return strings.Join(r, "/")
}

type recovery struct {
	issue  Issue
	tokens []token
}

func recoverInput(input any, issues []Issue) (any, bool, []Issue) {
	root := clone(input)
	targets := []recovery{}
	diagnostics := []Issue{}
	for _, issue := range issues {
		tokens := inputTokens(root, pathTokens(issue.Path))
		i := arrayIndex(tokens)
		if i >= 0 {
			tokens = tokens[:i+1]
		} else if issue.Code == "missing_field" {
			if _, exists := getTokens(root, tokens); !exists && len(tokens) > 0 {
				tokens = tokens[:len(tokens)-1]
			}
		}
		action := "使用默认值或上一层有效配置"
		if arrayIndex(tokens) >= 0 {
			action = "忽略无效列表项"
		} else if issue.Code == "unknown_field" || issue.Code == "removed_field" {
			action = "忽略字段"
		}
		issue.Action = action
		issue.Severity = "warning"
		diagnostics = append(diagnostics, issue)
		targets = append(targets, recovery{issue, tokens})
	}
	seen := map[string]bool{}
	unique := []recovery{}
	for _, target := range targets {
		key := tokenKey(target.tokens)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, target)
		}
	}
	sort.SliceStable(unique, func(i, j int) bool {
		a, b := unique[i].tokens, unique[j].tokens
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		ai, bi := arrayIndex(a), arrayIndex(b)
		av, bv := 0, 0
		if ai >= 0 {
			av = a[ai].index
		}
		if bi >= 0 {
			bv = b[bi].index
		}
		return av > bv
	})
	originalArrays := map[string][]token{}
	changed := false
	for _, target := range unique {
		if i := arrayIndex(target.tokens); i >= 0 {
			a := target.tokens[:i]
			v, _ := getTokens(input, a)
			if items, ok := v.([]any); ok && len(items) > 0 {
				originalArrays[tokenKey(a)] = a
			}
		}
		var deleted bool
		root, deleted = deleteTokens(root, target.tokens)
		if !deleted && target.issue.Expected == "id present in topbar.menu or leftbar.menu" {
			changed = setTokens(root, target.tokens, nil) || changed
		} else {
			changed = deleted || changed
		}
	}
	for _, tokens := range originalArrays {
		v, _ := getTokens(root, tokens)
		if a, ok := v.([]any); ok && len(a) == 0 {
			var deleted bool
			root, deleted = deleteTokens(root, tokens)
			changed = deleted || changed
		}
	}
	return root, changed, diagnostics
}
func uniqueIssues(issues []Issue) []Issue {
	result := []Issue{}
	seen := map[string]bool{}
	for _, issue := range issues {
		key := strings.Join([]string{issue.Code, issue.Source, issue.Path, issue.Expected, issue.Action}, "\x00")
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}
	return result
}
