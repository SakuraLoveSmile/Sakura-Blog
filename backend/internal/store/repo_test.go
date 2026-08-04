package store

import (
	"path/filepath"
	"testing"
)

// newRepoStore 建库 + links 表（泛型 Repository 用最简单的 Link 实体做 CRUD/QueryOption 覆盖）。
func newRepoStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureLinksTable(); err != nil {
		t.Fatalf("ensure links table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mkLink 便捷插入友链（ID 回填），返回指针供断言用。
func mkLink(t *testing.T, s *Store, name, url string, sort int) *Link {
	t.Helper()
	l := &Link{Name: name, URL: url, Sort: sort}
	id, err := s.CreateLink(l)
	if err != nil {
		t.Fatalf("create link %s: %v", name, err)
	}
	if id != l.ID {
		t.Fatalf("create link %s: returned id %d != backfilled %d", name, id, l.ID)
	}
	return l
}

// TestRepoCreateGetByID 泛型 Create 回填 ID；GetByID 查回；未找到返回 nil。
func TestRepoCreateGetByID(t *testing.T) {
	s := newRepoStore(t)
	l := mkLink(t, s, "友链 A", "https://a.example", 1)

	got, err := s.GetLink(l.ID)
	if err != nil {
		t.Fatalf("get link %d: %v", l.ID, err)
	}
	if got == nil || got.Name != "友链 A" || got.URL != "https://a.example" || got.Sort != 1 {
		t.Fatalf("get link mismatch: %+v", got)
	}

	missing, err := s.GetLink(l.ID + 999)
	if err != nil {
		t.Fatalf("get missing link: %v", err)
	}
	if missing != nil {
		t.Fatalf("missing link should be nil, got %+v", missing)
	}
}

// TestRepoUpdate 泛型 Update 全列生效；不存在的 id 报错。
func TestRepoUpdate(t *testing.T) {
	s := newRepoStore(t)
	l := mkLink(t, s, "旧名", "https://old.example", 0)

	l.Name = "新名"
	l.URL = "https://new.example"
	l.Sort = 5
	if err := s.UpdateLink(l.ID, l); err != nil {
		t.Fatalf("update link: %v", err)
	}

	got, err := s.GetLink(l.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != "新名" || got.URL != "https://new.example" || got.Sort != 5 {
		t.Fatalf("update not persisted: %+v", got)
	}

	ghost := &Link{Name: "幽灵"}
	if err := s.UpdateLink(l.ID+999, ghost); err == nil {
		t.Fatal("update missing link should error")
	}
}

// TestRepoDelete 泛型 Delete 后 GetByID 返回 nil；重复删除幂等。
func TestRepoDelete(t *testing.T) {
	s := newRepoStore(t)
	l := mkLink(t, s, "待删", "https://del.example", 0)

	if err := s.DeleteLink(l.ID); err != nil {
		t.Fatalf("delete link: %v", err)
	}
	got, err := s.GetLink(l.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("deleted link should be nil, got %+v", got)
	}
	if err := s.DeleteLink(l.ID); err != nil {
		t.Fatalf("re-delete should be idempotent: %v", err)
	}
}

// TestRepoListQueryOptions Where/OrderBy/Limit/Offset 组合过滤与排序。
func TestRepoListQueryOptions(t *testing.T) {
	s := newRepoStore(t)
	mkLink(t, s, "B", "https://b.example", 2)
	mkLink(t, s, "A", "https://a.example", 3)
	mkLink(t, s, "C", "https://c.example", 1)

	// 全部按 sort 倒序
	all, err := s.ListLinks()
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(all) != 3 || all[0].Name != "A" || all[1].Name != "B" || all[2].Name != "C" {
		t.Fatalf("list order mismatch: %+v", names(all))
	}

	// Where 过滤 + Limit/Offset 分页
	repo := repo(s, &Link{})
	paged, err := repo.List(Where("url LIKE ?", "%example%"), OrderBy("sort DESC"), Limit(2), Offset(1))
	if err != nil {
		t.Fatalf("list with options: %v", err)
	}
	if len(paged) != 2 || paged[0].Name != "B" || paged[1].Name != "C" {
		t.Fatalf("paged mismatch: %+v", names(paged))
	}
}

// TestRepoCount Where 过滤计数与总数。
func TestRepoCount(t *testing.T) {
	s := newRepoStore(t)
	mkLink(t, s, "A", "https://a.example", 0)
	mkLink(t, s, "B", "https://b.example", 0)

	repo := repo(s, &Link{})
	total, err := repo.Count()
	if err != nil {
		t.Fatalf("count all: %v", err)
	}
	if total != 2 {
		t.Fatalf("count all = %d, want 2", total)
	}
	filtered, err := repo.Count(Where("name = ?", "A"))
	if err != nil {
		t.Fatalf("count filtered: %v", err)
	}
	if filtered != 1 {
		t.Fatalf("count filtered = %d, want 1", filtered)
	}
}

// TestRepoGetOne 条件查单条：命中与未找到（nil）。
func TestRepoGetOne(t *testing.T) {
	s := newRepoStore(t)
	mkLink(t, s, "唯一", "https://one.example", 0)

	repo := repo(s, &Link{})
	hit, err := repo.GetOne("name = ?", "唯一")
	if err != nil {
		t.Fatalf("get one: %v", err)
	}
	if hit == nil || hit.URL != "https://one.example" {
		t.Fatalf("get one mismatch: %+v", hit)
	}
	miss, err := repo.GetOne("name = ?", "不存在")
	if err != nil {
		t.Fatalf("get one missing: %v", err)
	}
	if miss != nil {
		t.Fatalf("get one missing should be nil, got %+v", miss)
	}
}

// names 提取 Link 列表的 Name 序列，断言可读性用。
func names(links []*Link) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, l.Name)
	}
	return out
}
