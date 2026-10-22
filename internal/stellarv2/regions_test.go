package stellarv2

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
)

type regionFixture struct {
	Kind  string
	Input map[string]any
	Value any
}
type regionOracleData struct {
	Metadata struct {
		Tag, Commit string
		InputHashes map[string]string
	}
	Fixtures []regionFixture
}

func regionOracle(t *testing.T) regionOracleData {
	t.Helper()
	file, err := os.Open("testdata/regions-oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var data regionOracleData
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		t.Fatal(err)
	}
	return data
}
func fixtureMap(input map[string]any, key string) map[string]any {
	m, _ := input[key].(map[string]any)
	return m
}
func decodeRegionOptions(value any) RegionOptions {
	var result RegionOptions
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}
func decodeWidgetOptions(value any) WidgetOptions {
	var result WidgetOptions
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}
func fixtureLayers(input map[string]any) []map[string]any {
	result := []map[string]any{}
	layers, _ := input["layers"].([]any)
	for _, layer := range layers {
		m, _ := layer.(map[string]any)
		result = append(result, m)
	}
	return result
}
func runRegionFixture(fixture regionFixture) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	input := fixture.Input
	switch fixture.Kind {
	case "regions":
		return ResolveRegions(decodeRegionOptions(input)), nil
	case "cascade":
		return CascadeRegions(fixtureLayers(input), fixtureMap(input, "brandSources")), nil
	case "widgets":
		return ResolveRegionWidgets(input["widgets"], fixtureMap(input, "catalog"), decodeWidgetOptions(input["options"])), nil
	case "widget":
		return resolveWidget(input["specification"], fixtureMap(input, "catalog"), decodeWidgetOptions(input["options"]), int(input["index"].(float64))), nil
	case "merge":
		return MergeBrand(input["base"], input["override"], fixtureMap(input, "sources")), nil
	case "brand":
		return NormalizeBrand(input["brand"]), nil
	case "siteBrand":
		return SiteBrandModel(fixtureMap(input, "globalConfig")), nil
	case "mobile":
		return ShouldShowMobileBrand(text(input["profileKey"])), nil
	case "tokens":
		return ReplaceConfigTokens(input["value"], fixtureMap(input, "config")), nil
	case "navigation":
		return ToRenderNavigation(fixtureMap(input, "profile")), nil
	case "path":
		return map[string]any{"profilePath": ProfilePath(input["value"]), "generatorPath": GeneratorPath(input["value"])}, nil
	case "layout":
		return ToRenderRegions(fixtureMap(input, "globalConfig"), fixtureMap(input, "profile"), fixtureMap(input, "defaults"), decodeRegionOptions(input["options"])), nil
	default:
		panic("unknown Regions oracle kind: " + fixture.Kind)
	}
}
func TestOfficialRegionsOracle(t *testing.T) {
	data := regionOracle(t)
	if data.Metadata.Tag != "2.0.0" || data.Metadata.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" || len(data.Metadata.InputHashes) != 5 {
		t.Fatal("oracle is not pinned official Stellar 2.0.0")
	}
	failures := 0
	kinds := map[string]int{}
	for i, fixture := range data.Fixtures {
		value, err := runRegionFixture(fixture)
		kinds[fixture.Kind]++
		if err != nil || canonicalJSON(value) != canonicalJSON(fixture.Value) {
			failures++
			if failures <= 12 {
				t.Errorf("fixture %d %s: %v\ninput=%s\ngot=%s\nwant=%s", i, fixture.Kind, err, canonicalJSON(fixture.Input), canonicalJSON(value), canonicalJSON(fixture.Value))
			}
		}
	}
	if failures > 12 {
		t.Errorf("%d oracle mismatches (first 12 shown)", failures)
	}
	if len(kinds) != 12 || len(data.Fixtures) < 4000 {
		t.Fatal("incomplete region/brand/widget behavior fixture coverage")
	}
	t.Logf("Compared %d official Regions/Widgets/Brand fixtures across %d behavior families", len(data.Fixtures), len(kinds))
}
func TestRegionsDoNotAliasConfigurationCatalogOrBrandSources(t *testing.T) {
	layers := []map[string]any{{"leftbar": map[string]any{"brand": map[string]any{"name": "Site", "image": map[string]any{"src": "/site.png", "variant": "avatar"}}, "widgets": []any{"custom", "custom"}, "menu": []any{map[string]any{"id": "home"}}, "footer": map[string]any{"actions": []any{map[string]any{"type": "link"}}}}}}
	catalog := map[string]any{"custom": map[string]any{"layout": "markdown", "presentations": []any{"leftbar"}, "nested": map[string]any{"list": []any{map[string]any{"value": false}}}}}
	sources := map[string]any{"collection": map[string]any{"name": "Docs", "image": map[string]any{"src": "/docs.png", "variant": "plain"}}}
	beforeLayers, beforeCatalog, beforeSources := canonicalJSON(layers), canonicalJSON(catalog), canonicalJSON(sources)
	result := ResolveRegions(RegionOptions{Layers: layers, Catalog: catalog, BrandSources: sources})
	leftbar := result["leftbar"].(map[string]any)
	brand := leftbar["brand"].(map[string]any)
	brand["image"].(map[string]any)["src"] = "changed"
	leftbar["menu"].([]any)[0].(map[string]any)["id"] = "changed"
	leftbar["footer"].(map[string]any)["actions"].([]any)[0].(map[string]any)["type"] = "changed"
	instances := leftbar["widgets"].([]any)
	first, second := instances[0].(map[string]any), instances[1].(map[string]any)
	first["presentations"].([]any)[0] = "changed"
	first["item"].(map[string]any)["nested"].(map[string]any)["list"].([]any)[0].(map[string]any)["value"] = true
	if second["presentations"].([]any)[0] != "leftbar" || second["item"].(map[string]any)["nested"].(map[string]any)["list"].([]any)[0].(map[string]any)["value"] != false {
		t.Fatal("duplicate Widget instances share mutable fields")
	}
	merged := MergeBrand(brand, map[string]any{"source": "collection", "image": map[string]any{"variant": "avatar"}}, sources).(map[string]any)
	merged["image"].(map[string]any)["src"] = "changed"
	if beforeLayers != canonicalJSON(layers) || beforeCatalog != canonicalJSON(catalog) || beforeSources != canonicalJSON(sources) {
		t.Fatal("resolved result aliases input")
	}
	other := ResolveRegions(RegionOptions{Layers: layers, Catalog: catalog})
	if canonicalJSON(other) == canonicalJSON(result) {
		t.Fatal("mutations leaked across calls")
	}
}
func TestRegionCascadeRetainsExplicitPresenceAndScope(t *testing.T) {
	global := map[string]any{"leftbar": map[string]any{"brand": map[string]any{"name": "Site", "image": map[string]any{"src": "/site.png", "variant": "avatar"}}, "widgets": []any{"recent"}, "menu": []any{map[string]any{"id": "home"}}}}
	page, err := ParsePage(map[string]any{"leftbar": map[string]any{"brand": map[string]any{"name": nil, "image": map[string]any{"variant": "icon"}}, "widgets": []any{}, "menu": nil}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	result := CascadeRegions([]map[string]any{global, page.Value}, nil)
	leftbar := result["leftbar"].(map[string]any)
	if !reflect.DeepEqual(leftbar["widgets"], []any{}) || len(leftbar["menu"].([]any)) != 1 {
		t.Fatal("empty list must clear; null menu must inherit")
	}
	if name, exists := leftbar["brand"].(map[string]any)["name"]; !exists || name != nil {
		t.Fatal("explicit brand field null lost")
	}
	if image := leftbar["brand"].(map[string]any)["image"].(map[string]any); image["src"] != "/site.png" || image["variant"] != "icon" {
		t.Fatal("image field override replaced inherited src")
	}
	if result["topbar"].(map[string]any)["enabled"] != false || result["rightbar"].(map[string]any)["enabled"] != true {
		t.Fatal("initial region enabled states differ from upstream")
	}
}
func TestWidgetInstancesPreserveOriginalIndicesAndAuthoritativeCapabilities(t *testing.T) {
	result := ResolveRegionWidgets([]any{"missing", "toc", "toc", map[string]any{"layout": "timeline", "presentations": []any{"topbar"}}}, map[string]any{"toc": map[string]any{"layout": "toc"}}, WidgetOptions{Region: "topbar", Profile: "post"})
	instances := result["instances"].([]any)
	if len(instances) != 2 || instances[0].(map[string]any)["instanceId"] != "topbar:1:toc" || instances[1].(map[string]any)["instanceId"] != "topbar:2:toc" {
		t.Fatal("filtered instances were reindexed or deduplicated")
	}
	warnings := result["warnings"].([]any)
	if len(warnings) != 2 || warnings[1].(map[string]any)["code"] != "unsupported_widget_presentation" || warnings[1].(map[string]any)["profile"] != "post" {
		t.Fatal("inline parameter elevated placement capabilities")
	}
}
func TestRegionResolutionIsIndependentAcrossConcurrentCalls(t *testing.T) {
	options := RegionOptions{Catalog: map[string]any{"tree": map[string]any{"layout": "tree"}}, Layers: []map[string]any{{"leftbar": map[string]any{"widgets": []any{"tree"}, "brand": map[string]any{"name": "Site"}}}}}
	expected := canonicalJSON(ResolveRegions(options))
	var wait sync.WaitGroup
	failures := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result := ResolveRegions(options)
			if canonicalJSON(result) != expected {
				failures <- true
			}
			result["leftbar"].(map[string]any)["brand"].(map[string]any)["name"] = "local"
		}()
	}
	wait.Wait()
	close(failures)
	if len(failures) > 0 {
		t.Fatal("concurrent results changed shared inputs")
	}
}
