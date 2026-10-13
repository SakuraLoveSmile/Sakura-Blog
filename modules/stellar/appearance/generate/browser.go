//go:build ignore

// 仅开发使用的浏览器 CSS 夹具生成器，不是生产 CLI 依赖。
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"blog/internal/stellarv2"
	"blog/modules/stellar/appearance"
)

func main() {
	sources := []string{"hsl(192 98% 55%)", "#336699", "var(--visitor-primary)", "var(--unprovided, #336699)", "hwb(90 10% 20%)", "lab(60% 20 30)", "lch(60% 30 90)", "oklab(60% .1 .2)", "oklch(60% .2 90)", "color(display-p3 .2 .5 .8)", "color-mix(in srgb, red 30%, blue)", "hsl(.5turn 60% 40%)", "hsl(3.141592653589793rad 60% 40%)", "hsl(200grad 60% 40%)"}
	fixtures := []map[string]any{}
	for _, preset := range []string{"card", "glass", "minimal", "flat"} {
		for _, source := range sources {
			parsed, e := stellarv2.ParseTheme(map[string]any{"appearance": map[string]any{"preset": preset, "colors": map[string]any{"primary": source, "accent": source, "link": source}, "code_block": map[string]any{"highlight_stylesheet": nil}, "typography": map[string]any{"font_family": map[string]any{"body": "Georgia, serif", "code": "Consolas, monospace"}, "font_size": map[string]any{"root": "18px"}, "font_weight": map[string]any{"400": 650}}}}, stellarv2.Options{})
			if e != nil {
				panic(e)
			}
			css, e := appearance.Build(parsed.Value)
			if e != nil {
				panic(e)
			}
			fixtures = append(fixtures, map[string]any{"preset": preset, "source": source, "css": string(css)})
		}
	}
	if e := json.NewEncoder(os.Stdout).Encode(fixtures); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
