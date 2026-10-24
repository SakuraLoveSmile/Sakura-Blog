package post

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

//go:embed math_katex_data/*
var stellarKatexFiles embed.FS
var stellarKatexProgram *goja.Program
var stellarKatexProgramError error
var stellarKatexCompile sync.Once

const stellarKatexMaxFormulaBytes = 1 << 20
const stellarKatexMaxRenderTime = 2 * time.Second
const stellarKatexMaxPageTime = 10 * time.Second

// 只共享可信编译程序。VM、options/macros 与执行时间属于单次页面渲染。
func (r *shortcodeV2Renderer) katexFormula(tex string, display bool) (string, error) {
	if len(tex) > stellarKatexMaxFormulaBytes {
		return "", fmt.Errorf("katex formula exceeds 1 MiB")
	}
	if r.katexDeadline.IsZero() {
		r.katexDeadline = time.Now().Add(stellarKatexMaxPageTime)
	}
	if time.Until(r.katexDeadline) <= 0 {
		return "", fmt.Errorf("katex page exceeds 10-second rendering budget")
	}
	if r.katexVM == nil {
		stellarKatexCompile.Do(func() {
			source, e := stellarKatexFiles.ReadFile("math_katex_data/katex.min.js")
			if e != nil {
				stellarKatexProgramError = e
				return
			}
			stellarKatexProgram, stellarKatexProgramError = goja.Compile("katex-0.16.23.min.js", string(source), false)
		})
		if stellarKatexProgramError != nil {
			return "", stellarKatexProgramError
		}
		vm := goja.New()
		module := vm.NewObject()
		_ = module.Set("exports", vm.NewObject())
		_ = vm.Set("module", module)
		_ = vm.Set("exports", module.Get("exports"))
		_ = vm.Set("console", map[string]any{"warn": func(...any) {}})
		if _, e := runStellarKatex(vm, min(stellarKatexMaxRenderTime, time.Until(r.katexDeadline)), func() (goja.Value, error) { return vm.RunProgram(stellarKatexProgram) }); e != nil {
			return "", e
		}
		f, ok := goja.AssertFunction(module.Get("exports").ToObject(vm).Get("renderToString"))
		if !ok {
			return "", fmt.Errorf("katex renderToString unavailable")
		}
		cfg := v2Object(v2Object(v2Object(r.theme["features"])["math"])["katex"])
		options := map[string]any{}
		for key, value := range cfg {
			if key != "css" && key != "css_integrity" && key != "cssIntegrity" {
				options[key] = value
			}
		}
		// JSON 复制避免 KaTeX 修改外部 macros 快照；仅 JS 所需 number 按语言语义转换。
		raw, e := json.Marshal(options)
		if e != nil {
			return "", e
		}
		if len(raw) > stellarKatexMaxFormulaBytes {
			return "", fmt.Errorf("katex options exceed 1 MiB")
		}
		parse, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("parse"))
		value, e := runStellarKatex(vm, min(stellarKatexMaxRenderTime, time.Until(r.katexDeadline)), func() (goja.Value, error) { return parse(goja.Undefined(), vm.ToValue(string(raw))) })
		if e != nil {
			return "", e
		}
		r.katexVM, r.katexRender, r.katexOptions = vm, f, value.ToObject(vm)
	}
	_ = r.katexOptions.Set("displayMode", display)
	value, e := runStellarKatex(r.katexVM, min(stellarKatexMaxRenderTime, time.Until(r.katexDeadline)), func() (goja.Value, error) {
		return r.katexRender(goja.Undefined(), r.katexVM.ToValue(tex), r.katexOptions)
	})
	if e != nil {
		return "", fmt.Errorf("katex: %w", e)
	}
	return value.String(), nil
}
func runStellarKatex(vm *goja.Runtime, budget time.Duration, run func() (goja.Value, error)) (goja.Value, error) {
	// 每次调用结束等待 timer 回调归还，避免旧定时器误中断下一个公式。
	done := make(chan struct{})
	timer := time.AfterFunc(budget, func() { vm.Interrupt("katex rendering exceeds 2 seconds"); close(done) })
	value, e := run()
	if !timer.Stop() {
		<-done
	}
	vm.ClearInterrupt()
	return value, e
}
