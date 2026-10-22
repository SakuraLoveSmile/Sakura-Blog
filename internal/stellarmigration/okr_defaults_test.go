package stellarmigration

import (
	"database/sql"
	"strings"
	"testing"
)

// 旧配置形状；固定上游 JS/Stylus/zh-CN 给出这些值，不依赖生产私密输入。
const officialOKRFixture = `{"border":true,"status":{"unfinished":{"label":"未完成","color":"red"},"finished":{"color":"green","label":"已完成"},"off_track":{"label":"延期","color":"orange"},"at_risk":{"color":"yellow","label":"风险"},"in_track":{"color":"blue","label":"正常"}}}`

func settingsSnapshot(t *testing.T, sections map[string]any) Snapshot {
	t.Helper()
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);`)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range sortedKeys(sections) {
		if _, err = db.Exec(`INSERT INTO settings(section,value) VALUES(?,?)`, key, encode(sections[key])); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	return snapshot(t, path, nil)
}
func TestOfficialOKRDefaultsInheritWithoutLosingPrivateSource(t *testing.T) {
	value, err := decode(officialOKRFixture)
	if err != nil {
		t.Fatal(err)
	}
	s := settingsSnapshot(t, map[string]any{"tag_plugins": map[string]any{"okr": value}})
	c := candidate(t, s)
	if hasCode(c.Theme.Diagnostics, "tag_config_removed") || c.PublicReport().Blocking != 0 {
		t.Fatal("exact locked upstream defaults still require user policy decision")
	}
	if encode(c.Source) != encode(s) {
		t.Fatal("matching defaults dropped private source")
	}
	raw, err := decode(c.Theme.ThemeJSON)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := get(object(raw), "tags.okr"); exists {
		t.Fatal("removed OKR configuration injected into strict schema")
	}
	matched := false
	for _, mapping := range c.Theme.Mappings {
		if mapping.Source == "settings.tag_plugins.okr" && mapping.Target == "source.snapshot" && strings.Contains(mapping.Behavior, "继承官方等效") {
			matched = true
		}
	}
	if !matched {
		t.Fatal("official inheritance was not explained in the migration preview")
	}
	if err = ValidateCandidate(c, s); err != nil {
		t.Fatal(err)
	}
}
func TestCustomOrIncompleteOKRIsNeverTreatedAsOfficialDefault(t *testing.T) {
	cases := map[string]func(map[string]any){
		"border disabled":   func(v map[string]any) { v["border"] = false },
		"wrong border type": func(v map[string]any) { v["border"] = "true" },
		"extra config":      func(v map[string]any) { v["custom"] = true },
		"missing border":    func(v map[string]any) { delete(v, "border") },
		"missing status":    func(v map[string]any) { delete(object(v["status"]), "at_risk") },
		"extra status": func(v map[string]any) {
			object(v["status"])["custom"] = map[string]any{"color": "blue", "label": "custom"}
		},
		"custom label":       func(v map[string]any) { object(object(v["status"])["finished"])["label"] = "完成啦" },
		"custom color":       func(v map[string]any) { object(object(v["status"])["in_track"])["color"] = "#123456" },
		"extra status field": func(v map[string]any) { object(object(v["status"])["in_track"])["icon"] = "custom" },
		"missing label":      func(v map[string]any) { delete(object(object(v["status"])["in_track"]), "label") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := decode(officialOKRFixture)
			if err != nil {
				t.Fatal(err)
			}
			value := object(raw)
			change(value)
			s := settingsSnapshot(t, map[string]any{"tag_plugins": map[string]any{"okr": value}})
			c := candidate(t, s)
			if !hasCode(c.Theme.Diagnostics, "tag_config_removed") || c.PublicReport().Blocking != 1 {
				t.Fatal("custom or incomplete OKR silently erased")
			}
			if encode(c.Source) != encode(s) || ValidateCandidate(c, s) == nil {
				t.Fatal("custom source was changed or accepted")
			}
		})
	}
}
