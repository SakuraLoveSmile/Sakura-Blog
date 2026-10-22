// 开发期 oracle 对照所有夹具，保留尚未闭合的 SVG 等效失败。
package main

import (
	"blog/internal/stellarimages"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

type result struct {
	File     string                 `json:"file"`
	Match    bool                   `json:"match"`
	Expected stellarimages.Analysis `json:"expected"`
	Actual   stellarimages.Analysis `json:"actual"`
	Error    string                 `json:"error,omitempty"`
}

func main() {
	dir := flag.String("fixtures", "internal/stellarimages/testdata", "fixture directory")
	flag.Parse()
	expectedRaw, err := os.ReadFile(filepath.Join(*dir, "sharp.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var expected map[string]stellarimages.Analysis
	if err = json.Unmarshal(expectedRaw, &expected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	names := make([]string, 0, len(expected))
	for name := range expected {
		names = append(names, name)
	}
	sort.Strings(names)
	results := []result{}
	failed := false
	for _, name := range names {
		r := result{File: name, Expected: expected[name]}
		data, e := os.ReadFile(filepath.Join(*dir, name))
		if e == nil {
			r.Actual, e = stellarimages.Analyze(data)
		}
		if e != nil {
			r.Error = e.Error()
		}
		r.Match = e == nil && reflect.DeepEqual(r.Expected, r.Actual)
		failed = failed || !r.Match
		results = append(results, r)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if failed {
		os.Exit(1)
	}
}
