package stellarv2

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type oracleFixture struct {
	Kind, Label, Source string
	Input               any
	Recover, Failed     bool
	ValueHash           string
	Issues              []Issue
}
type oracleProfile struct {
	Kind, Profile string
	Config, Value map[string]any
	Failed        bool
	Issues        []Issue
}
type oracleMember struct {
	Kind, Source string
	Config       map[string]any
	Result       MembershipResult
}
type oracleData struct {
	Fixtures        []oracleFixture
	Profiles        []oracleProfile
	Collections     map[string]map[string]any
	Members         []oracleMember
	UpstreamCrashes []any
}

func oracle(t *testing.T) oracleData {
	t.Helper()
	file, err := os.Open("testdata/oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var data oracleData
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		t.Fatal(err)
	}
	return data
}
func canonicalJSON(value any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(value); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
func valueHash(value any) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalJSON(value))))
}
func comparableIssues(issues []Issue) []string {
	result := []string{}
	for _, issue := range issues {
		issue.Severity = ""
		result = append(result, canonicalJSON(issue))
	}
	sort.Strings(result)
	return result
}
func runFixture(f oracleFixture) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	options := Options{Source: f.Source, Recover: f.Recover}
	switch f.Kind {
	case "theme":
		return ParseTheme(f.Input, options)
	case "collection":
		return ParseCollection(f.Input, options)
	default:
		return ParsePage(f.Input, options)
	}
}
func TestOfficialOracle(t *testing.T) {
	data := oracle(t)
	failures := 0
	for _, fixture := range data.Fixtures {
		result, err := runFixture(fixture)
		var issues []Issue
		var schemaError *SchemaError
		if err != nil && errors.As(err, &schemaError) {
			issues = schemaError.Issues
		} else {
			issues = result.Issues
		}
		valid := fixture.Failed == (err != nil) && reflect.DeepEqual(comparableIssues(issues), comparableIssues(fixture.Issues))
		if !fixture.Failed && err == nil {
			valid = valid && fixture.ValueHash == valueHash(result.Value)
		}
		if !valid {
			failures++
			if failures <= 20 {
				t.Errorf("%s %s recover=%v: err=%v\nissues=%v\nwant=%v\nhash=%s want=%s", fixture.Kind, fixture.Label, fixture.Recover, err, comparableIssues(issues), comparableIssues(fixture.Issues), valueHash(result.Value), fixture.ValueHash)
			}
		}
	}
	if failures > 20 {
		t.Errorf("%d oracle failures total (first 20 printed)", failures)
	}
	t.Logf("Compared %d official strict/recover parser fixtures; %d upstream TypeErrors documented by generator", len(data.Fixtures), len(data.UpstreamCrashes))
}
func TestOfficialURLOracle(t *testing.T) {
	for _, fixture := range oracle(t).Fixtures {
		if !strings.HasPrefix(fixture.Label, "nav URL ") && !strings.HasPrefix(fixture.Label, "HTTP URL ") && !strings.HasPrefix(fixture.Label, "Resource ") {
			continue
		}
		result, err := runFixture(fixture)
		issues := result.Issues
		var e *SchemaError
		if errors.As(err, &e) {
			issues = e.Issues
		}
		if fixture.Failed != (err != nil) || !reflect.DeepEqual(comparableIssues(issues), comparableIssues(fixture.Issues)) || (!fixture.Failed && fixture.ValueHash != valueHash(result.Value)) {
			t.Errorf("%s recover=%v: err=%v issues=%v wantFailed=%v want=%v", fixture.Label, fixture.Recover, err, comparableIssues(issues), fixture.Failed, comparableIssues(fixture.Issues))
		}
	}
}
func TestOfficialProfileOracle(t *testing.T) {
	for _, fixture := range oracle(t).Profiles {
		var result Result
		var err error
		if fixture.Kind == "page" {
			result = ValidatePageProfile(fixture.Config, "oracle", fixture.Profile)
		} else {
			result, err = ValidateCollectionProfile(fixture.Config, "oracle", fixture.Profile)
		}
		if fixture.Failed != (err != nil) || canonicalJSON(result.Value) != canonicalJSON(fixture.Value) || !reflect.DeepEqual(comparableIssues(result.Issues), comparableIssues(fixture.Issues)) {
			t.Errorf("%s %s: value=%v issues=%v err=%v expected=%v %v", fixture.Kind, fixture.Profile, result.Value, result.Issues, err, fixture.Value, fixture.Issues)
		}
	}
}
func TestOfficialMembershipOracle(t *testing.T) {
	data := oracle(t)
	registry := CreateCollectionRegistry(data.Collections)
	for _, fixture := range data.Members {
		result := ResolveContentMembership(MembershipInput{Kind: fixture.Kind, Source: fixture.Source, Config: fixture.Config, Registry: registry})
		if canonicalJSON(result) != canonicalJSON(fixture.Result) {
			t.Errorf("%s %s %v: got=%v want=%v", fixture.Kind, fixture.Source, fixture.Config, result, fixture.Result)
		}
	}
}
func TestDefaultsAndInputHaveIndependentOwnership(t *testing.T) {
	a, b := DefaultTheme(), DefaultTheme()
	appearance, _ := atObject(a, "appearance")
	appearance["preset"] = "changed"
	if v, _ := Get(b, "appearance.preset"); v != "card" {
		t.Fatal("DefaultTheme aliases shared registry")
	}
	input := map[string]any{"comments": map[string]any{"giscus": map[string]any{"nested": []any{map[string]any{"value": false}}}}}
	original := canonicalJSON(input)
	result, err := ParseTheme(input, Options{})
	if err != nil {
		t.Fatal(err)
	}
	bag, _ := atObject(result.Value, "comments.giscus")
	bag["nested"].([]any)[0].(map[string]any)["value"] = true
	if canonicalJSON(input) != original {
		t.Fatal("parsed bag aliases caller input")
	}
}
func TestNativeYAMLValuesAndPresence(t *testing.T) {
	when := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	input := map[any]any{"date": when, "leftbar": map[any]any{"enabled": false, "widgets": []string{}}, "listing": map[any]any{"priority": 0}, "footer": map[any]any{"license": false, "share": []string{}}}
	result, err := ParsePage(input, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Value["date"], when) {
		t.Fatal("external date not preserved")
	}
	if v, ok := Get(result.Value, "leftbar.enabled"); !ok || v != false {
		t.Fatal("false lost")
	}
	if _, ok := Get(result.Value, "comments"); ok {
		t.Fatal("omitted override populated")
	}
	if v, _ := Get(result.Value, "listing.priority"); v != 0 {
		t.Fatal("zero lost")
	}
	bad := map[string]any{"listing": map[string]any{"priority": math.Inf(1)}}
	_, err = ParsePage(bad, Options{})
	if err == nil {
		t.Fatal("nonfinite number accepted")
	}
}
func TestRecoveryDoesNotModifyInputOrOwnership(t *testing.T) {
	input := map[string]any{"collection": map[string]any{"profile": "wrong", "id": 0}, "banner": map[string]any{"invalid": true}}
	original := canonicalJSON(input)
	result, err := ParsePage(input, Options{Source: "source/doc.md", Recover: true})
	var schemaError *SchemaError
	if !errors.As(err, &schemaError) {
		t.Fatalf("ownership should remain fatal: %+v", result)
	}
	if len(schemaError.Issues) != 2 {
		t.Fatalf("unexpected ownership issues: %+v", schemaError.Issues)
	}
	if canonicalJSON(input) != original {
		t.Fatal("recovery mutated input")
	}
}
func TestSchemaRegistryHasImplementations(t *testing.T) {
	// 通过空/default oracle 与每个声明节点覆盖，未知 normalizer/validator
	// 会 panic 并使测试失败，不允许生成数据新增能力而解析器静默遗漏。
	if len(schemas.Theme.PropertyOrder) != 18 || len(schemas.Page.PropertyOrder) != 18 || len(schemas.Collection.PropertyOrder) != 22 || len(schemas.Effects) != 4 {
		t.Fatal("incomplete generated official declarations")
	}
}
