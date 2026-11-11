package store

import (
	"path/filepath"
	"testing"
)

func TestEnsureLinksTableMigratesWatchColumns(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.DB().Exec(`CREATE TABLE links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_title TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '', avatar TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '', rss TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '', sort INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
	link := &Link{Name: "legacy", URL: "https://example.com", LabelsJSON: `[{"name":"老友","color":"30b0c7"}]`}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateLinkCheck(link.ID, 204, 123); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateLinkSnapshot(link.ID, "/uploads/snapshots/1.jpg"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetLink(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Snapshot != "/uploads/snapshots/1.jpg" || got.LastStatus != 204 || got.LastCheckedAt != 123 {
		t.Fatalf("migrated fields not persisted: %+v", got)
	}
	if extras := got.ExtraLabels(); len(extras) != 1 || extras[0].Name != "老友" || extras[0].Color != "30b0c7" {
		t.Fatalf("labels column not persisted: %+v", extras)
	}
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
}

func TestLinkLabelNormalization(t *testing.T) {
	cases := map[string]string{
		"#FF6B9D": "ff6b9d",
		"abc":     "aabbcc",
		"30B0C7":  "30b0c7",
		"":        "",
		"zzz":     "",
		"#12345":  "",
	}
	for in, want := range cases {
		if got := NormalizeLinkColor(in); got != want {
			t.Errorf("NormalizeLinkColor(%q) = %q, want %q", in, got, want)
		}
	}

	labels := NormalizeLinkLabels([]LinkLabel{{Name: " 老友 ", Color: "#30B0C7"}, {Name: "  "}, {Name: "社区先锋"}})
	if len(labels) != 2 || labels[0].Name != "老友" || labels[0].Color != "30b0c7" || labels[1].Name != "社区先锋" {
		t.Fatalf("NormalizeLinkLabels = %+v", labels)
	}

	if raw := EncodeLinkLabels(nil); raw != "" {
		t.Fatalf("EncodeLinkLabels(nil) = %q, want empty", raw)
	}
	raw := EncodeLinkLabels([]LinkLabel{{Name: "老友", Color: "30B0C7"}})
	if raw != `[{"name":"老友","color":"30b0c7"}]` {
		t.Fatalf("EncodeLinkLabels = %s", raw)
	}
	if got := ParseLinkLabels(raw); len(got) != 1 || got[0].Name != "老友" {
		t.Fatalf("ParseLinkLabels = %+v", got)
	}
	for _, broken := range []string{`{`, `null`, `"x"`, `[{"name":""}]`} {
		if got := ParseLinkLabels(broken); len(got) != 0 {
			t.Fatalf("ParseLinkLabels(%q) = %+v, want empty", broken, got)
		}
	}
	// 坏数据最多回放 MaxLinkLabels 枚，避免撑破卡片。
	many := ParseLinkLabels(`[{"name":"a"},{"name":"b"},{"name":"c"},{"name":"d"}]`)
	if len(many) != MaxLinkLabels {
		t.Fatalf("ParseLinkLabels cap = %d, want %d", len(many), MaxLinkLabels)
	}
}
