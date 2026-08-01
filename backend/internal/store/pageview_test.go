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

func TestHourlyZeroFill(t *testing.T) {
	s := newTestStorePV(t)

	// 无任何数据时，24 小时全零
	hours, err := s.HourlyVisits(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 24 {
		t.Fatalf("expect 24, got %d", len(hours))
	}
	for i, h := range hours {
		// 验证小时序号连续（0-23）且计数为零
		if h.Hour != i {
			t.Errorf("hour[%d].Hour = %d, want %d", i, h.Hour, i)
		}
		if h.Count != 0 {
			t.Errorf("hour %d not zero: %+v", i, h)
		}
	}
}

func TestHourlyCrossDayBoundary(t *testing.T) {
	s := newTestStorePV(t)

	insert := func(ts string) {
		t.Helper()
		_, err := s.db.Exec(`INSERT INTO page_views(path, referrer, ua_hash, created_at) VALUES (?, '', ?, ?)`,
			"/post/hello", "ua_abc", ts)
		if err != nil {
			t.Fatal(err)
		}
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	// 跨天边界：今天 00 点两次 + 23 点一次，昨天 23 点一次（与今天 23 点合并入同一小时桶）
	insert(today + " 00:15:00")
	insert(today + " 00:45:00")
	insert(today + " 23:30:00")
	insert(yesterday + " 23:10:00")
	// 窗口之外（8 天前）不应计入 days=7
	old := time.Now().AddDate(0, 0, -8).Format("2006-01-02")
	insert(old + " 00:00:00")

	hours, err := s.HourlyVisits(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 24 {
		t.Fatalf("expect 24, got %d", len(hours))
	}
	if hours[0].Count != 2 {
		t.Errorf("hour 0 count = %d, want 2 (窗口外记录不应计入)", hours[0].Count)
	}
	if hours[23].Count != 2 {
		t.Errorf("hour 23 count = %d, want 2 (昨天+今天跨天合并)", hours[23].Count)
	}
	// 其余小时补零
	for i := 1; i < 23; i++ {
		if hours[i].Count != 0 {
			t.Errorf("hour %d should be zero, got %d", i, hours[i].Count)
		}
	}
}

func TestHourlyLocalTimezone(t *testing.T) {
	s := newTestStorePV(t)

	// RecordView 默认按 localtime 落库，聚合小时应与 Go 本地时区一致
	before := time.Now().Hour()
	if err := s.RecordView("/post/hello", "", "ua_abc"); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Hour()

	hours, err := s.HourlyVisits(1)
	if err != nil {
		t.Fatal(err)
	}
	got := -1
	for _, h := range hours {
		if h.Count > 0 {
			got = h.Hour
			break
		}
	}
	// 允许恰好跨小时边界执行的情形
	if got != before && got != after {
		t.Errorf("local hour bucket = %d, want %d or %d", got, before, after)
	}
}

func TestHourlyTimezoneNaiveAggregation(t *testing.T) {
	s := newTestStorePV(t)

	insert := func(ts string) {
		t.Helper()
		_, err := s.db.Exec(`INSERT INTO page_views(path, referrer, ua_hash, created_at) VALUES (?, '', ?, ?)`,
			"/post/hello", "ua_abc", ts)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 同一时刻（今天本地正午）分别按 UTC / 东京 / 纽约时区格式化落库，
	// 聚合应按字面小时入桶，不做任何时区换算
	now := time.Now()
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	expected := make(map[int]int, 4)
	for _, name := range []string{"UTC", "Asia/Tokyo", "America/New_York"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		zoned := noon.In(loc)
		insert(zoned.Format("2006-01-02 15:04:05"))
		expected[zoned.Hour()]++
	}

	// days=7 窗口下，字面日期最多偏移 ±1 天，三条记录均应计入
	hours, err := s.HourlyVisits(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(hours) != 24 {
		t.Fatalf("expect 24, got %d", len(hours))
	}
	for i, h := range hours {
		if h.Count != expected[i] {
			t.Errorf("hour %d count = %d, want %d (应按字面小时入桶)", i, h.Count, expected[i])
		}
	}
}

func TestHourlyTimezoneWindowBoundary(t *testing.T) {
	s := newTestStorePV(t)

	insert := func(ts string) {
		t.Helper()
		_, err := s.db.Exec(`INSERT INTO page_views(path, referrer, ua_hash, created_at) VALUES (?, '', ?, ?)`,
			"/post/hello", "ua_abc", ts)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 今天本地 00:30 这一时刻按 UTC 格式化：若本地时区在 UTC 以东，
	// 字面日期会回退到昨天，被 days=1 窗口过滤；days=2 则必然计入
	now := time.Now()
	early := time.Date(now.Year(), now.Month(), now.Day(), 0, 30, 0, 0, time.Local)
	utc := early.In(time.UTC)
	insert(utc.Format("2006-01-02 15:04:05"))

	want1 := 0
	if utc.Format("2006-01-02") >= now.Format("2006-01-02") {
		want1 = 1
	}
	hours1, err := s.HourlyVisits(1)
	if err != nil {
		t.Fatal(err)
	}
	if hours1[utc.Hour()].Count != want1 {
		t.Errorf("days=1 hour %d count = %d, want %d (按字面日期过滤)", utc.Hour(), hours1[utc.Hour()].Count, want1)
	}

	hours2, err := s.HourlyVisits(2)
	if err != nil {
		t.Fatal(err)
	}
	if hours2[utc.Hour()].Count != 1 {
		t.Errorf("days=2 hour %d count = %d, want 1 (字面日期最多回退一天)", utc.Hour(), hours2[utc.Hour()].Count)
	}
}

func TestHourlyDaysParamBoundary(t *testing.T) {
	s := newTestStorePV(t)

	insert := func(ts string) {
		t.Helper()
		_, err := s.db.Exec(`INSERT INTO page_views(path, referrer, ua_hash, created_at) VALUES (?, '', ?, ?)`,
			"/post/hello", "ua_abc", ts)
		if err != nil {
			t.Fatal(err)
		}
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	// 窗口边界：昨天 23:59:59 在 days=1 时排除、days=2 时计入；今天 00:00:00 始终计入
	insert(yesterday + " 23:59:59")
	insert(today + " 00:00:00")

	hours1, err := s.HourlyVisits(1)
	if err != nil {
		t.Fatal(err)
	}
	if hours1[0].Count != 1 || hours1[23].Count != 0 {
		t.Errorf("days=1: hour0=%d hour23=%d, want 1,0", hours1[0].Count, hours1[23].Count)
	}

	hours2, err := s.HourlyVisits(2)
	if err != nil {
		t.Fatal(err)
	}
	if hours2[0].Count != 1 || hours2[23].Count != 1 {
		t.Errorf("days=2: hour0=%d hour23=%d, want 1,1", hours2[0].Count, hours2[23].Count)
	}

	// days<=0 回退默认 7 天：6 天前计入，7 天前排除
	insert(time.Now().AddDate(0, 0, -6).Format("2006-01-02") + " 05:00:00")
	insert(time.Now().AddDate(0, 0, -7).Format("2006-01-02") + " 06:00:00")

	hours0, err := s.HourlyVisits(0)
	if err != nil {
		t.Fatal(err)
	}
	if hours0[5].Count != 1 || hours0[6].Count != 0 {
		t.Errorf("days=0(默认7): hour5=%d hour6=%d, want 1,0", hours0[5].Count, hours0[6].Count)
	}
}
