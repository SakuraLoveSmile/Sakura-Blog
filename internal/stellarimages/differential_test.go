package stellarimages

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSharpOracle(t *testing.T) {
	b, e := os.ReadFile("testdata/sharp.json")
	if e != nil {
		t.Fatal(e)
	}
	var expected map[string]Analysis
	if e = json.Unmarshal(b, &expected); e != nil {
		t.Fatal(e)
	}
	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			data, e := os.ReadFile(filepath.Join("testdata", name))
			if e != nil {
				t.Fatal(e)
			}
			got, e := Analyze(data)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(got.Color, want.Color) {
				t.Fatalf("color got=%+v expected=%+v", got.Color, want.Color)
			}
			if got.Dimensions.Width != want.Dimensions.Width || got.Dimensions.Height != want.Dimensions.Height {
				t.Fatalf("dimensions got %+v want %+v", got.Dimensions, want.Dimensions)
			}
			if !colorReady(got.Color) {
				t.Fatalf("color not ready: %+v", got.Color)
			}
		})
	}
}
