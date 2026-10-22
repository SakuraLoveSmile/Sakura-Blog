package routes

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
)

func TestStellarMomentsLocalesActualSSRAndOfflinePagination(t *testing.T) {
	for _, test := range []struct{ locale, title, just, min, hour, day string }{
		{"zh-CN", "瞬间", "刚刚", "2 分钟前", "3 小时前", "2 天前"},
		{"zh-TW", "瞬間", "剛剛", "2 分鐘前", "3 小時前", "2 天前"},
		{"en", "Moments", "Just", "2 minutes ago", "3 hours ago", "2 days ago"},
	} {
		t.Run(test.locale, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			site := *ws.site.Load()
			version := *site.StellarV2
			version.Blog = stellarCopyObject(version.Blog)
			version.Blog["site"] = map[string]any{"language": test.locale}
			site.StellarV2 = &version
			site.URL = "https://fixture.example/sub"
			ws.site.Store(&site)
			now := time.Now()
			ages := []time.Duration{20 * time.Second, 2*time.Minute + 10*time.Second, 3*time.Hour + 10*time.Second, 48*time.Hour + 10*time.Second}
			for index, age := range ages {
				if _, err := st.CreateMoment(&store.Moment{ContentMD: fmt.Sprintf("Public recent %d", index), Images: `["/moment-image.png"]`, Visibility: store.MomentVisibilityPublic, CreatedAt: now.Add(-age)}); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < pageSize-2; i++ {
				if _, err := st.CreateMoment(&store.Moment{ContentMD: "Public older", Visibility: store.MomentVisibilityPublic, CreatedAt: now.Add(-32 * 24 * time.Hour)}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := st.CreateMoment(&store.Moment{ContentMD: "PRIVATE_MOMENT_LOCALE_SENTINEL", Visibility: store.MomentVisibilityPrivate, CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			engine.SetStellarMarkdownRender(ws.posts.RenderStellarV2WithContext)
			ws.engine = engine
			for _, path := range []string{"/sub/moments", "/sub/moments/page/2"} {
				response := stellarRequest(router, path)
				if response.Code != http.StatusOK {
					t.Fatalf("%s => %d %s", path, response.Code, response.Body.String())
				}
				body := response.Body.String()
				for _, fragment := range []string{`<h1 class="text title"><span>` + test.title + `</span></h1>`, `data-page-layout="moments"`, `class="timenode"`, `tag-plugin timeline`, `rel="canonical" href="https://fixture.example` + path + `"`} {
					if !strings.Contains(body, fragment) {
						t.Fatalf("%s missing %s", path, fragment)
					}
				}
				if strings.Contains(body, "PRIVATE_MOMENT_LOCALE_SENTINEL") {
					t.Fatal("private moment leaked")
				}
				wantNodes := pageSize
				if path == "/sub/moments/page/2" {
					wantNodes = 2
				}
				if nodes := strings.Count(body, `class="timenode"`); nodes != wantNodes {
					t.Fatalf("%s public nodes=%d, want %d", path, nodes, wantNodes)
				}
				if path == "/sub/moments" {
					for _, fragment := range []string{test.just, test.min, test.hour, test.day, `data-src="/sub/moment-image.png"`, `data-fancybox="moment-`, `href="/sub/moments/page/2/"`} {
						if !strings.Contains(body, fragment) {
							t.Fatalf("missing first-page locale/gallery/pagination %s", fragment)
						}
					}
				}
			}
			if response := stellarRequest(router, "/sub/moments/page/3"); response.Code != http.StatusNotFound {
				t.Fatalf("out-of-range public page accepted: %d", response.Code)
			}
			ws.engine = &stellarOfflineSnapshotCapture{t: t, renderer: engine}
			snapshots, err := ws.renderSnapshots(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, snapshot := range snapshots {
				if snapshot.Path == "moments/index.html" || snapshot.Path == "moments/page/2/index.html" {
					count++
					if !strings.Contains(snapshot.HTML, `<h1 class="text title"><span>`+test.title+`</span></h1>`) || strings.Contains(snapshot.HTML, "PRIVATE_MOMENT_LOCALE_SENTINEL") {
						t.Fatal("Offline moments lost locale/public filtering")
					}
				}
			}
			if count != 2 {
				t.Fatalf("Offline pages=%d, want 2", count)
			}
		})
	}
}

func TestStellarMomentLocaleTimeBoundariesAndFallback(t *testing.T) {
	now := time.Date(2026, 10, 13, 20, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		age  time.Duration
		want string
	}{
		{-time.Hour, "Just"}, {59 * time.Second, "Just"}, {time.Minute, "1 minutes ago"},
		{59*time.Minute + 59*time.Second, "59 minutes ago"}, {time.Hour, "1 hours ago"},
		{24 * time.Hour, "1 days ago"}, {30*24*time.Hour - time.Second, "29 days ago"},
		{30 * 24 * time.Hour, "2026-09-13"},
	} {
		if got := stellarMomentTimeAgo(now.Add(-test.age), now, "en"); got != test.want {
			t.Fatalf("age=%v: %q, want %q", test.age, got, test.want)
		}
	}
	for _, test := range []struct{ locale, title, just string }{
		{"zh_TW", "瞬間", "剛剛"}, {"zh-Hant", "瞬間", "剛剛"}, {"zh-HK", "瞬間", "剛剛"},
		{"en-US", "Moments", "Just"}, {"zh-Hans", "瞬间", "刚刚"}, {"unsupported", "瞬间", "刚刚"},
	} {
		if title := stellarMomentsTitle(test.locale); title != test.title {
			t.Fatalf("%s title %q", test.locale, title)
		}
		if label := stellarMomentTimeAgo(now, now, test.locale); label != test.just {
			t.Fatalf("%s time %q", test.locale, label)
		}
	}
}
