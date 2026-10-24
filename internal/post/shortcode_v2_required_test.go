package post

import "testing"

// 必填键必须是该标签真实存在的参数；否则编辑表单的必填校验永远无法满足，该标签将无法应用。
func TestStellarV2RequiredDeclarationsReferenceRealParams(t *testing.T) {
	for _, entry := range ShortcodeCatalogStellarV2() {
		params := map[string]bool{}
		for _, p := range entry.Params {
			params[p.Key] = true
		}
		for _, key := range entry.Edit.Required {
			if !params[key] {
				t.Fatalf("%s required %q is not a V2 param", entry.Name, key)
			}
		}
		for _, group := range entry.Edit.RequiredAny {
			for _, key := range group {
				if !params[key] {
					t.Fatalf("%s required_any %q is not a V2 param", entry.Name, key)
				}
			}
		}
	}
}
