package routes

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	stellarweb "blog/modules/stellar/web"
)

func TestStellarMigratedGoDatesRenderWithoutRewritingSource(t *testing.T) {
	for _, tc := range []struct {
		name, date, updated, iso, display string
	}{
		{"utc", "2026-09-23 08:11:09.744169834 +0000 UTC", "2026-09-23 08:12:22.376099468 +0000 UTC m=+98019.328184131", "2026-09-23T08:11:09Z", "2026-09-23"},
		{"offset", "2026-09-24 01:11:09.744169834 +0800 CST", "2026-09-24 01:12:22 +0800 CST", "2026-09-23T17:11:09Z", "2026-09-23"},
		{"canonical", "2026-09-23T08:11:09Z", "2026-09-23T08:12:22Z", "2026-09-23T08:11:09Z", "2026-09-23"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			ws.engine = engine
			row := createStellarPost(t, st, "legacy-date", "")
			cfg, err := json.Marshal(map[string]any{"date": tc.date, "updated": tc.updated})
			if err != nil {
				t.Fatal(err)
			}
			row.StellarConfig = string(cfg)
			if err := st.Update(row.Slug, row); err != nil {
				t.Fatal(err)
			}
			before, err := st.GetBySlug(row.Slug)
			if err != nil {
				t.Fatal(err)
			}
			response := stellarRequest(router, "/post/legacy-date")
			want := `<time datetime="` + tc.iso + `">` + tc.display + `</time>`
			if response.Code != 200 || !strings.Contains(response.Body.String(), want) {
				t.Fatalf("SSR date missing %s (status %d)", want, response.Code)
			}
			if strings.Contains(response.Body.String(), ` m=+`) || strings.Contains(response.Body.String(), `<time datetime="2026-09-23 08:`) {
				t.Fatal("legacy date escaped normalization")
			}
			feed := stellarRequest(router, "/feed.json")
			var data struct {
				Items []struct {
					DatePublished time.Time `json:"date_published"`
				} `json:"items"`
			}
			wantTime, _ := time.Parse(time.RFC3339, tc.iso)
			if feed.Code != 200 || json.Unmarshal(feed.Body.Bytes(), &data) != nil || len(data.Items) != 1 || data.Items[0].DatePublished.UTC().Format(time.RFC3339) != wantTime.Format(time.RFC3339) {
				t.Fatalf("JSON feed date missing (status %d): %s", feed.Code, feed.Body.String())
			}
			after, err := st.GetBySlug(row.Slug)
			if err != nil {
				t.Fatal(err)
			}
			if after.StellarConfig != before.StellarConfig || !after.CreatedAt.Equal(before.CreatedAt) || !after.UpdatedAt.Equal(before.UpdatedAt) {
				t.Fatal("rendering rewrote stored dates or config")
			}
		})
	}
}
