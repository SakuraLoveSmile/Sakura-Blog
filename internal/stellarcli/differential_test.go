package stellarcli

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"blog/internal/config"
)

func TestOfficialCommandRules(t *testing.T) {
	var oracle struct {
		Commit    string `json:"commit"`
		NoteCases []struct {
			Title           string   `json:"title"`
			OK              bool     `json:"ok"`
			NormalizedTitle string   `json:"normalizedTitle"`
			Tags            []string `json:"tags"`
		} `json:"noteCases"`
		TagCases []struct {
			Input string   `json:"input"`
			OK    bool     `json:"ok"`
			Tags  []string `json:"tags"`
		} `json:"tagCases"`
		DoctorCases []struct {
			Theme    map[string]any     `json:"theme"`
			Widgets  map[string]any     `json:"widgets"`
			Warnings []widgetProjection `json:"warnings"`
		} `json:"doctorCases"`
	}
	b, err := os.ReadFile("testdata/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Commit != testEnvironment().Commit {
		t.Fatal("oracle commit mismatch")
	}
	st, _, owner := fixture(t)
	for _, c := range oracle.NoteCases {
		t.Run("title/"+c.Title, func(t *testing.T) {
			plan, err := BuildNotePlan(st, NoteOptions{Notebook: "journal", Title: c.Title, Tags: "go, go, GO,a/b", Author: owner, GeneratedAt: testTime()})
			if (err == nil) != c.OK {
				t.Fatalf("official ok=%t; Go err=%v", c.OK, err)
			}
			if err == nil && (plan.Title != c.NormalizedTitle || !reflect.DeepEqual(plan.Tags, c.Tags)) {
				t.Fatalf("official title/tags mismatch: %#v", plan)
			}
		})
	}
	for _, c := range oracle.TagCases {
		tags, err := NormalizeTags(c.Input)
		if (err == nil) != c.OK || err == nil && !reflect.DeepEqual(tags, c.Tags) {
			t.Fatalf("tags %q: got %v,%v; official=%v,ok=%t", c.Input, tags, err, c.Tags, c.OK)
		}
	}
	for i, c := range oracle.DoctorCases {
		if _, err := config.SaveStellarV2(st, c.Theme, map[string]any{"data": map[string]any{"widgets": c.Widgets}}); err != nil {
			t.Fatal(err)
		}
		report := Doctor(context.Background(), st, testEnvironment())
		if !report.OK {
			t.Fatalf("doctor case %d issues=%#v", i, report.Issues)
		}
		got := []widgetProjection{}
		for _, d := range report.Warnings {
			if d.Widget != "" {
				got = append(got, widgetProjection{d.Code, d.Widget, d.Layout, d.Region, d.Supported})
			}
		}
		if !reflect.DeepEqual(sortedProjection(got), sortedProjection(c.Warnings)) {
			t.Fatalf("doctor case %d\nGo %v\nOfficial %v", i, sortedProjection(got), sortedProjection(c.Warnings))
		}
	}
}

type widgetProjection struct {
	Code      string   `json:"code"`
	Widget    string   `json:"widget"`
	Layout    string   `json:"layout"`
	Region    string   `json:"region"`
	Supported []string `json:"supported"`
}

func sortedProjection(values []widgetProjection) []string {
	out := []string{}
	for _, v := range values {
		b, _ := json.Marshal(v)
		out = append(out, string(b))
	}
	sort.Strings(out)
	return out
}
