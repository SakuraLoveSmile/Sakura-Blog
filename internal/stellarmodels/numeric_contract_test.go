package stellarmodels

import (
	"encoding/json"
	"testing"
)

func TestJSONNumberArithmeticAndTruth(t *testing.T) {
	for _, raw := range []string{"0", "-0", "0e2"} {
		if truth(json.Number(raw)) {
			t.Errorf("json.Number(%q) must be falsy", raw)
		}
	}
	for _, raw := range []string{"2", "2e1", "-1.5"} {
		if !truth(json.Number(raw)) {
			t.Errorf("json.Number(%q) must be truthy", raw)
		}
	}
	if text(json.Number("2e1")) != "20" || jsString(json.Number("-0")) != "0" || jsString(json.Number("1e21")) != "1e+21" {
		t.Fatal("numeric string coercion did not follow JS double semantics")
	}
	if got := number(json.Number("2e1")); got != 20 {
		t.Errorf("double arithmetic lost number: %v", got)
	}
	if got := number(json.Number("9007199254740993")); got != 9007199254740992 {
		t.Errorf("arithmetic does not follow JS double rounding: %v", got)
	}
	if got := normalizeDate(json.Number("1700000000000")); got != "2023-11-14T22:13:20.000Z" {
		t.Errorf("numeric timestamp lost: %v", got)
	}
	for _, raw := range []string{"1e400", "8640000000000001"} {
		if got := normalizeDate(json.Number(raw)); got != nil {
			t.Errorf("out-of-range JS date %s accepted: %v", raw, got)
		}
	}
	if got := normalizeDate(json.Number("8640000000000000")); got != "+275760-09-13T00:00:00.000Z" {
		t.Errorf("expanded ISO date changed: %v", got)
	}
	if got := normalizeDate(json.Number("-1.9")); got != "1969-12-31T23:59:59.999Z" {
		t.Errorf("timestamp does not truncate toward zero: %v", got)
	}
}

// 实际 Settings 保存/重开与 JSON Front Matter 导入链使用 UseNumber；
// 计算遵循 JS double，配置本身与开放参数袋的大整数保持原值。
