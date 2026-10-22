package stellarv2

import (
	"sort"
	"strings"
)

// ValidateCollectionProfile applies the upstream profile capability warning policy
// to an already parsed runtime object. It never guesses a profile from a URL.
func ValidateCollectionProfile(config map[string]any, source, profile string) (Result, error) {
	type policy struct {
		hero, start, tree bool
		listing           []string
	}
	policies := map[string]policy{
		"wiki":     {hero: true, tree: true, listing: []string{"priority", "order"}},
		"topic":    {start: true, listing: []string{"excerptLength", "sort"}},
		"notebook": {listing: []string{"order", "excerptLength", "perPage", "sort"}},
	}
	rule, ok := policies[profile]
	if !ok {
		issue := profileIssue(source, "root", profile)
		return Result{Issues: []Issue{issue}}, &SchemaError{Issues: []Issue{issue}}
	}
	issues := []Issue{}
	if _, exists := config["hero"]; exists && !rule.hero {
		issues = append(issues, profileIssue(source, "hero", profile))
	}
	if _, exists := Get(config, "route.start"); exists && !rule.start {
		issues = append(issues, profileIssue(source, "route.start", profile))
	}
	if _, exists := Get(config, "navigation.tree"); exists && !rule.tree {
		issues = append(issues, profileIssue(source, "navigation.tree", profile))
	}
	listing, _ := atObject(config, "listing")
	for _, key := range keys(listing) {
		if !includes(rule.listing, key) {
			issues = append(issues, profileIssue(source, "listing."+snake(key), profile))
		}
	}
	return omitUnsupported(config, issues), nil
}
func ValidatePageProfile(config map[string]any, source, profile string) Result {
	issues := []Issue{}
	if _, exists := Get(config, "listing.priority"); exists && !includes([]string{"post", "topic", "notebook"}, profile) {
		issues = append(issues, profileIssue(source, "listing.priority", profile))
	}
	return omitUnsupported(config, issues)
}
func profileIssue(source, field, profile string) Issue {
	return Issue{Code: "invalid_value", Source: source, Path: field, ActualType: "declared field", Expected: "field supported by the " + profile + " profile", Migration: "content-schema/profile-capabilities"}
}
func camel(s string) string { return replacePatternFunc(`_([a-z])`, s) }
func omitUnsupported(config map[string]any, issues []Issue) Result {
	result := clone(config).(map[string]any)
	for i := range issues {
		parts := strings.Split(issues[i].Path, ".")
		for j := range parts {
			parts[j] = camel(parts[j])
		}
		tokens := make([]token, len(parts))
		for j, part := range parts {
			tokens[j] = token{key: part}
		}
		deleteTokens(result, tokens)
		issues[i].Severity = "warning"
		issues[i].Action = "忽略不适用的表现参数"
	}
	return Result{Value: result, Issues: issues}
}

// CollectionRecord uses parsed camelCase Config, keyed by source/_data relative
// names such as wiki/docs, topic/tutorial or notebooks/journal (without .yml).
type CollectionRecord struct {
	Profile   string
	ID        string
	Key       string
	SourceKey string
	Config    map[string]any
}
type CollectionRegistry struct {
	Records     []CollectionRecord
	byKey       map[string]CollectionRecord
	wikiMembers map[string]map[string]bool
	topicStarts map[string]map[string]bool
}

func CreateCollectionRegistry(configs map[string]map[string]any) *CollectionRegistry {
	r := &CollectionRegistry{byKey: map[string]CollectionRecord{}, wikiMembers: map[string]map[string]bool{}, topicStarts: map[string]map[string]bool{}}
	names := []string{}
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, sourceKey := range names {
		parts := strings.SplitN(sourceKey, "/", 2)
		if len(parts) != 2 || parts[1] == "" || !includes([]string{"wiki", "topic", "notebooks"}, parts[0]) {
			continue
		}
		profile := parts[0]
		if profile == "notebooks" {
			profile = "notebook"
		}
		id := parts[1]
		key := profile + ":" + id
		config := clone(configs[sourceKey]).(map[string]any)
		record := CollectionRecord{Profile: profile, ID: id, Key: key, SourceKey: sourceKey, Config: config}
		r.Records = append(r.Records, record)
		r.byKey[key] = record
		if profile == "wiki" {
			route, _ := Get(config, "route.path")
			base := text(route)
			if base == "" {
				base = "wiki/" + id
			}
			base = NormalizeMembershipPath(base)
			tree, _ := Get(config, "navigation.tree")
			for _, member := range flattenTree(tree) {
				addIndex(r.wikiMembers, base+"/"+member, key)
			}
		} else if profile == "topic" {
			start, _ := Get(config, "route.start")
			if _, ok := start.(string); ok {
				addIndex(r.topicStarts, text(start), key)
			}
		}
	}
	return r
}
func addIndex(index map[string]map[string]bool, signal, key string) {
	signal = NormalizeMembershipPath(signal)
	if signal == "" {
		return
	}
	if index[signal] == nil {
		index[signal] = map[string]bool{}
	}
	index[signal][key] = true
}
func flattenTree(tree any) []string {
	result := []string{}
	switch v := tree.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case map[string]any:
		for _, key := range keys(v) {
			items, _ := v[key].([]any)
			for _, item := range items {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
		}
	}
	return result
}
func NormalizeMembershipPath(value string) string {
	value = strings.TrimLeft(strings.TrimPrefix(strings.ReplaceAll(value, "\\", "/"), "source/"), "/")
	value = replacePattern(`(?i)\.md$`, value, "")
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = strings.TrimSuffix(value, "/index.html")
		value = strings.TrimSuffix(value, "/index")
		value = strings.TrimSuffix(value, ".html")
		if len(value) > 1 {
			value = strings.TrimSuffix(value, "/")
		}
	}
	return strings.Trim(value, "/")
}

type MembershipInput struct {
	Kind     string
	Source   string
	PagePath string
	Config   map[string]any
	Registry *CollectionRegistry
}
type MembershipResult struct {
	Config     map[string]any
	Candidates []string
	Inferred   bool
	Issues     []Issue
}

func namespace(kind, source string) *CollectionRecord {
	value := NormalizeMembershipPath(source)
	var pattern string
	switch kind {
	case "pages":
		pattern = `^(wiki|notebooks)/([^/]+)(?:/|$)`
	case "posts":
		pattern = `^_posts/topics?/([^/]+)(?:/|$)`
	default:
		return nil
	}
	m := submatches(pattern, value)
	if len(m) == 0 {
		return nil
	}
	profile, id := "topic", m[1]
	if kind == "pages" {
		profile, id = m[1], m[2]
		if profile == "notebooks" {
			profile = "notebook"
		}
	}
	return &CollectionRecord{Profile: profile, ID: id, Key: profile + ":" + id}
}
func candidates(input MembershipInput, includeNamespace bool) []string {
	result := map[string]bool{}
	r := input.Registry
	if r == nil {
		return []string{}
	}
	if ns := namespace(input.Kind, input.Source); includeNamespace && ns != nil {
		if _, exists := r.byKey[ns.Key]; exists {
			result[ns.Key] = true
		}
	}
	index := r.wikiMembers
	if input.Kind == "posts" {
		index = r.topicStarts
	}
	source := NormalizeMembershipPath(input.Source)
	withoutPosts := strings.TrimPrefix(source, "_posts/")
	permalink := text(input.Config["permalink"])
	signals := []string{source, withoutPosts, NormalizeMembershipPath(withoutPosts), NormalizeMembershipPath(input.PagePath), NormalizeMembershipPath(permalink), NormalizeMembershipPath(withoutPosts[strings.LastIndex(withoutPosts, "/")+1:])}
	for _, signal := range signals {
		if signal == "" {
			continue
		}
		for key := range index[signal] {
			result[key] = true
		}
	}
	return sortedSet(result)
}
func CandidateKeys(input MembershipInput) []string { return candidates(input, true) }
func sortedSet(set map[string]bool) []string {
	r := make([]string, 0, len(set))
	for key := range set {
		r = append(r, key)
	}
	sort.Strings(r)
	return r
}
func membershipIssue(code, source, actual string, candidates []string, fix string) Issue {
	list := strings.Join(candidates, ", ")
	if list == "" {
		list = "<none>"
	}
	return Issue{Code: code, Source: source, Path: "collection", ActualType: actual, Expected: "one registered Collection; candidates=" + list + "; fix=" + fix, Migration: "content-schema/membership"}
}

// ResolveContentMembership implements explicit ownership, path inference,
// relationship conflicts and ambiguity. Membership issues are always structural.
func ResolveContentMembership(input MembershipInput) MembershipResult {
	if input.Source == "" {
		input.Source = "<page>"
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if input.Registry == nil {
		input.Registry = CreateCollectionRegistry(nil)
	}
	r := input.Registry
	candidateKeys := CandidateKeys(input)
	relationships := candidates(input, false)
	hard := map[string]bool{}
	for _, key := range relationships {
		hard[key] = true
	}
	ns := namespace(input.Kind, input.Source)
	if ns != nil && ns.Profile != "wiki" {
		if _, exists := r.byKey[ns.Key]; exists {
			hard[ns.Key] = true
		}
	}
	conflicting := sortedSet(hard)
	collection, _ := input.Config["collection"].(map[string]any)
	explicit := ""
	if collection != nil {
		explicit = text(collection["profile"]) + ":" + text(collection["id"])
	}
	failed := func(issue Issue) MembershipResult {
		return MembershipResult{Candidates: candidateKeys, Issues: []Issue{issue}}
	}
	if explicit != "" {
		profile := text(collection["profile"])
		allowed := []string{"wiki", "notebook"}
		if input.Kind == "posts" {
			allowed = []string{"topic"}
		}
		_, exists := r.byKey[explicit]
		if !includes(allowed, profile) || !exists {
			return failed(membershipIssue("collection_not_found", input.Source, "collection:"+explicit, candidateKeys, "use a profile/id registered in source/_data and valid for this content source"))
		}
		if len(conflicting) > 0 && !includes(conflicting, explicit) {
			return failed(membershipIssue("collection_conflict", input.Source, "collection:"+explicit, conflicting, "change collection.profile/id to the inferred member or move the source file"))
		}
		return MembershipResult{Config: clone(input.Config).(map[string]any), Candidates: candidateKeys, Issues: []Issue{}}
	}
	if len(candidateKeys) == 1 {
		record := r.byKey[candidateKeys[0]]
		config := clone(input.Config).(map[string]any)
		config["collection"] = map[string]any{"profile": record.Profile, "id": record.ID}
		return MembershipResult{Config: config, Candidates: candidateKeys, Inferred: true, Issues: []Issue{}}
	}
	if len(candidateKeys) > 1 {
		return failed(membershipIssue("collection_ambiguous", input.Source, "collection:missing", candidateKeys, "add collection.profile and collection.id to this page"))
	}
	if ns != nil {
		directory := ns.Profile
		if directory == "notebook" {
			directory = "notebooks"
		}
		return failed(membershipIssue("collection_not_found", input.Source, "collection:missing", candidateKeys, "create source/_data/"+directory+"/"+ns.ID+".yml or add a valid collection.profile/id"))
	}
	return MembershipResult{Config: clone(input.Config).(map[string]any), Candidates: candidateKeys, Issues: []Issue{}}
}
