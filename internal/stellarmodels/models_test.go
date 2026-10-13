package stellarmodels

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
)

type fixture struct {
	Name     string          `json:"name"`
	Op       string          `json:"op"`
	Input    json.RawMessage `json:"input"`
	Expected struct {
		Value json.RawMessage `json:"value"`
		Error string          `json:"error"`
	} `json:"expected"`
}

func oracle(t *testing.T) []fixture {
	t.Helper()
	f, e := os.Open("testdata/official-oracle.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	r, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	var data struct {
		Upstream string    `json:"upstream"`
		Entries  []fixture `json:"entries"`
	}
	if e = json.NewDecoder(r).Decode(&data); e != nil {
		t.Fatal(e)
	}
	if data.Upstream != "9d11978a24fe6610e3a7dbab741cf62634539d05" {
		t.Fatal("oracle version changed")
	}
	return data.Entries
}
func runDecoded(f fixture, useNumber bool) (any, error) {
	decode := func(target any) error {
		d := json.NewDecoder(bytes.NewReader(f.Input))
		if useNumber {
			d.UseNumber()
		}
		return d.Decode(target)
	}
	var in Input
	if err := decode(&in); err != nil {
		return nil, err
	}
	var raw Object
	if err := decode(&raw); err != nil {
		return nil, err
	}
	switch f.Op {
	case "snapshot":
		return PageModelInput(obj(raw["page"]), obj(raw["config"])), nil
	case "discovery":
		var input struct{ Posts, Pages []Member }
		if err := decode(&input); err != nil {
			return nil, err
		}
		return DiscoverContent(input.Posts, input.Pages), nil
	case "select":
		var items, tags []Object
		for _, x := range array(raw["items"]) {
			items = append(items, obj(x))
		}
		for _, x := range array(raw["tags"]) {
			tags = append(tags, obj(x))
		}
		var tagID *string
		if raw["tagId"] != nil {
			s := str(raw["tagId"])
			tagID = &s
		}
		return SelectListingItems(items, tagID, tags, nil), nil
	case "paginate":
		var items []Object
		for _, x := range array(raw["items"]) {
			items = append(items, obj(x))
		}
		return PaginateItems(items, int(number(raw["size"]))), nil
	case "page":
		return BuildPage(in)
	case "collection:post":
		return BuildPostCollection(in.Theme)
	case "collection:wiki":
		return BuildWikiCollection(in)
	case "collection:topic":
		return BuildTopicCollection(in)
	case "collection:notebook":
		return BuildNotebookCollection(in)
	case "topic-index":
		return BuildTopicIndexRender(in)
	case "wiki-listing":
		c, e := BuildWikiCollection(in)
		if e != nil {
			return nil, e
		}
		return BuildWikiListingRender(in, c), nil
	case "comments":
		return ResolveComments(in.Theme, obj(raw["overrides"]), str(raw["title"])), nil
	case "share":
		return BuildShareAction(str(raw["service"]), obj(raw["share"])), nil
	case "ai":
		return BuildAILabelHTML(str(raw["value"]), str(raw["text"]), str(raw["extraClass"]), str(raw["iconHTML"]), raw["noColor"] == true), nil
	}
	panic(f.Op)
}
func jsonValue(v any) any {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	var r any
	if e = json.Unmarshal(b, &r); e != nil {
		panic(e)
	}
	return r
}
func diff(a, b any, path string) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		for k, v := range am {
			if d := diff(v, bm[k], path+"."+k); d != "" {
				return d
			}
		}
		for k := range bm {
			if _, ok := am[k]; !ok {
				return path + "." + k + ": missing actual key"
			}
		}
	}
	aa, aok := a.([]any)
	bb, bok := b.([]any)
	if aok && bok && len(aa) == len(bb) {
		for i := range aa {
			if d := diff(aa[i], bb[i], path+"[]"); d != "" {
				return d
			}
		}
	}
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return path + ": actual=" + string(aJSON) + " expected=" + string(bJSON)
}
func TestOfficialOracle(t *testing.T) {
	fixtures := oracle(t)
	if len(fixtures) < 780 {
		t.Fatalf("incomplete oracle: %d", len(fixtures))
	}
	for _, mode := range []struct {
		name      string
		useNumber bool
	}{{"defaultFloat64", false}, {"useNumber", true}} {
		t.Run(mode.name, func(t *testing.T) {
			for _, f := range fixtures {
				t.Run(f.Name, func(t *testing.T) {
					actual, err := runDecoded(f, mode.useNumber)
					if f.Expected.Error != "" {
						if err == nil {
							t.Fatalf("expected official %s error", f.Expected.Error)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					var expected any
					if err = json.Unmarshal(f.Expected.Value, &expected); err != nil {
						t.Fatal(err)
					}
					if d := diff(jsonValue(actual), expected, "model"); d != "" {
						t.Fatal(d)
					}
				})
			}
		})
	}
}
func TestIndependentConcurrentModels(t *testing.T) {
	fixtures := oracle(t)
	var f fixture
	for _, x := range fixtures {
		if x.Name == "wiki default page" {
			f = x
			break
		}
	}
	var in Input
	if err := json.Unmarshal(f.Input, &in); err != nil {
		t.Fatal(err)
	}
	before := jsonValue(in)
	a, e := BuildPage(in)
	if e != nil {
		t.Fatal(e)
	}
	a.Collection["id"] = "changed"
	obj(a.Item["presentation"])["article"] = Object{"style": "changed"}
	b, e := BuildPage(in)
	if e != nil {
		t.Fatal(e)
	}
	if b.Collection["id"] == "changed" || !reflect.DeepEqual(before, jsonValue(in)) {
		t.Fatal("mutable state aliases input or sibling output")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 10; n++ {
				v, e := BuildPage(in)
				if e != nil {
					t.Error(e)
					return
				}
				v.Render["changed"] = true
			}
		}()
	}
	wg.Wait()
}
func TestRejectUnnormalizedAndMismatchedMembership(t *testing.T) {
	if _, err := BuildPostCollection(Object{"style": Object{}}); err == nil {
		t.Fatal("legacy theme accepted")
	}
	f := oracle(t)[0]
	var in Input
	if err := json.Unmarshal(f.Input, &in); err != nil {
		t.Fatal(err)
	}
	in.FrontMatter = Object{"collection": Object{"profile": "wiki", "id": "missing"}}
	in.CollectionID = "else"
	if _, err := BuildPage(in); err == nil {
		t.Fatal("mismatched membership accepted")
	}
}

func TestStableSelectionAndPaginationCopies(t *testing.T) {
	items := []Object{{"id": "first", "rank": 2}, {"id": "hidden", "rank": 0, "listed": false}, {"id": "second", "rank": 1}, {"id": "third", "rank": 1}}
	result := SelectListingItems(items, nil, nil, func(a, b Object) bool { return number(a["rank"]) < number(b["rank"]) })
	if len(result) != 3 || result[0]["id"] != "second" || result[1]["id"] != "third" || result[2]["id"] != "first" {
		t.Fatalf("sorting changed stable ties: %#v", result)
	}
	result[0]["id"] = "changed"
	if items[2]["id"] != "second" {
		t.Fatal("selection aliases input")
	}
	pages := PaginateItems(items, 2)
	pages[0][0]["id"] = "changed"
	if items[0]["id"] != "first" {
		t.Fatal("pagination aliases input")
	}
}
