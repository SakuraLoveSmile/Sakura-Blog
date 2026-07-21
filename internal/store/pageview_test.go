package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStorePV(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsurePageViewsTable(); err != nil {
		t.Fatalf("ensure page_views: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecordAndTrend(t *testing.T) {
	s := newTestStorePV(t)

	// 同一 UA 同日访问同一路径 3 次 → PV=3, UV=1
	for i := 0; i < 3; i++ {
		if err := s.RecordView("/post/hello", "", "ua_abc"); err != nil {
			t.Fatal(err)
		}
	}
	// 不同 UA 访问同一路径 → UV+1
	if err := s.RecordView("/post/hello", "", "ua_def"); err != nil {
		t.Fatal(err)
	}

	trend, err := s.TrendPVUV(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(trend) != 7 {
		t.Fatalf("expect 7 days, got %d", len(trend))
	}
	// 最后一天（今天）应有数据
	today := trend[len(trend)-1]
	if today.PV != 4 {
		t.Errorf("today PV = %d, want 4", today.PV)
	}
	if today.UV != 2 {
		t.Errorf("today UV = %d, want 2", today.UV)
	}
	// 前面的空日应为 0
	for i := 0; i < 6; i++ {
		if trend[i].PV != 0 || trend[i].UV != 0 {
			t.Errorf("day %d should be zero, got pv=%d uv=%d", i, trend[i].PV, trend[i].UV)
		}
	}
}

func TestUVCrossDay(t *testing.T) {
	s := newTestStorePV(t)

	// 手动插入昨天的记录（同一 UA）
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(`INSERT INTO page_views(path, referrer, ua_hash, created_at) VALUES (?, '', ?, ?)`,
		"/post/hello", "ua_abc", yesterday)
	if err != nil {
		t.Fatal(err)
	}
	// 今天同一 UA 再访问
	if err := s.RecordView("/post/hello", "", "ua_abc"); err != nil {
		t.Fatal(err)
	}

	trend, err := s.TrendPVUV(7)
	if err != nil {
		t.Fatal(err)
	}
	// 昨天 UV=1，今天 UV=1（跨日重新计）
	day6 := trend[5] // 昨天
	day7 := trend[6] // 今天
	if day6.UV != 1 || day7.UV != 1 {
		t.Errorf("cross-day UV: yesterday=%d today=%d, want 1,1", day6.UV, day7.UV)
	}
}

func TestPopularByViews(t *testing.T) {
	s := newTestStorePV(t)

	// /post/a 3 次，/post/b 1 次
	for i := 0; i < 3; i++ {
		_ = s.RecordView("/post/a", "", "ua1")
	}
	_ = s.RecordView("/post/b", "", "ua2")

	popular, err := s.PopularByViews(7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(popular) != 2 {
		t.Fatalf("expect 2, got %d", len(popular))
	}
	if popular[0].Path != "/post/a" || popular[0].PV != 3 {
		t.Errorf("first popular = %+v, want /post/a pv=3", popular[0])
	}
}

func TestTopReferrers(t *testing.T) {
	s := newTestStorePV(t)

	_ = s.RecordView("/post/a", "https://google.com", "ua1")
	_ = s.RecordView("/post/a", "https://google.com", "ua2")
	_ = s.RecordView("/post/a", "https://github.com", "ua3")
	_ = s.RecordView("/post/a", "", "ua4") // 空 referrer 不计

	refs, err := s.TopReferrers(7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("expect 2 referrers, got %d", len(refs))
	}
	if refs[0].Referrer != "https://google.com" || refs[0].Count != 2 {
		t.Errorf("top referrer = %+v", refs[0])
	}
}

func TestPrunePageViews(t *testing.T) {
	s := newTestStorePV(t)

	// 插入一条 100 天前的记录
	old := time.Now().AddDate(0, 0, -100).Format("2006-01-02 12:00:00")
	_, err := s.db.Exec(`INSERT INTO page_views(path, referrer, ua_hash, created_at) VALUES (?, '', ?, ?)`,
		"/post/old", "ua_old", old)
	if err != nil {
		t.Fatal(err)
	}
	// 插入一条今天的记录
	_ = s.RecordView("/post/new", "", "ua_new")

	// 保留 90 天 → 100 天前的应被清理
	n, err := s.PrunePageViews(90)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pruned = %d, want 1", n)
	}

	// 今天的仍在
	trend, _ := s.TrendPVUV(1)
	if trend[0].PV != 1 {
		t.Errorf("after prune today PV = %d, want 1", trend[0].PV)
	}
}

func TestTrendZeroFill(t *testing.T) {
	s := newTestStorePV(t)

	// 无任何数据时，7 天全零
	trend, err := s.TrendPVUV(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(trend) != 7 {
		t.Fatalf("expect 7, got %d", len(trend))
	}
	for i, d := range trend {
		if d.PV != 0 || d.UV != 0 {
			t.Errorf("day %d not zero: %+v", i, d)
		}
	}
	// 验证日期连续性（第一天 = 6 天前）
	expected := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	if trend[0].Date != expected {
		t.Errorf("first date = %s, want %s", trend[0].Date, expected)
	}
}
