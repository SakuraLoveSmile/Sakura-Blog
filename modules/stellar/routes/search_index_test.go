package routes

import (
	"path/filepath"
	"testing"

	"blog/internal/og"
	"blog/internal/search"
	"blog/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSetupSearchIndex(t *testing.T) {
	st := newTestStore(t)
	idx := setupSearchIndex(st)
	if idx == nil {
		t.Fatal("fts index should be available on fresh sqlite store")
	}
	if n, err := idx.Count(); err != nil || n != 0 {
		t.Errorf("fresh index should be empty: n=%d err=%v", n, err)
	}
}

func TestNewPostWriteHookNilSearchIndex(t *testing.T) {
	st := newTestStore(t)
	ogGen := og.New("", t.TempDir())
	hook := newPostWriteHook(st, nil, ogGen)
	// searchIdx 降级为 nil 时钩子不 panic（只做 OG 失效）
	hook("update", "some-slug")
	hook("delete", "some-slug")
}

// TestPageMomentWriteHooks 页面/瞬间写路径回调：发布入索引，下线/私密/删除/回收站移除。
func TestPageMomentWriteHooks(t *testing.T) {
	st := newTestStore(t)
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages: %v", err)
	}
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments: %v", err)
	}
	idx := search.New(st.DB())
	if err := idx.Ensure(); err != nil {
		t.Fatalf("ensure fts: %v", err)
	}
	st.SetPageWriteHook(newPageWriteHook(st, idx))
	st.SetMomentWriteHook(newMomentWriteHook(st, idx))

	// 页面：草稿不入索引，发布后入索引，回收站移除，恢复再入，彻底删除移除
	if _, err := st.CreatePage(&store.Page{Slug: "about", Title: "关于页", ContentHTML: "<p>页面正文含页面独特词</p>", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	if n, _ := idx.Count(); n != 0 {
		t.Fatalf("draft page should not be indexed, count=%d", n)
	}
	if err := st.UpdatePage("about", &store.Page{Slug: "about", Title: "关于页", ContentHTML: "<p>页面正文含页面独特词</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("页面独特词", search.TypePage, 20); len(res) != 1 {
		t.Fatalf("published page should be indexed, got %d", len(res))
	}
	if err := st.DeletePage("about"); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("页面独特词", search.TypePage, 20); len(res) != 0 {
		t.Fatalf("trashed page should be removed, got %d", len(res))
	}
	if err := st.RestorePage("about"); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("页面独特词", search.TypePage, 20); len(res) != 1 {
		t.Fatalf("restored page should be indexed again, got %d", len(res))
	}
	if err := st.PurgePage("about"); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("页面独特词", search.TypePage, 20); len(res) != 0 {
		t.Fatalf("purged page should be removed, got %d", len(res))
	}

	// 瞬间：公开入索引，改私密移除，改回公开再入，删除移除
	m := &store.Moment{ContentHTML: "<p>瞬间正文含瞬间独特词</p>", Visibility: store.MomentVisibilityPublic}
	id, err := st.CreateMoment(m)
	if err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("瞬间独特词", search.TypeMoment, 20); len(res) != 1 {
		t.Fatalf("public moment should be indexed, got %d", len(res))
	}
	m.Visibility = store.MomentVisibilityPrivate
	if err := st.UpdateMoment(m); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("瞬间独特词", search.TypeMoment, 20); len(res) != 0 {
		t.Fatalf("private moment should be removed, got %d", len(res))
	}
	m.Visibility = store.MomentVisibilityPublic
	if err := st.UpdateMoment(m); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("瞬间独特词", search.TypeMoment, 20); len(res) != 1 {
		t.Fatalf("public moment should be re-indexed, got %d", len(res))
	}
	if err := st.DeleteMoment(id); err != nil {
		t.Fatal(err)
	}
	if res, _ := idx.Search("瞬间独特词", search.TypeMoment, 20); len(res) != 0 {
		t.Fatalf("deleted moment should be removed, got %d", len(res))
	}
}

// TestSearchItemsAll 回填条目覆盖三类：草稿/私密/回收站不入，密码文章正文置空。
func TestSearchItemsAll(t *testing.T) {
	st := newTestStore(t)
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages: %v", err)
	}
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments: %v", err)
	}
	if _, err := st.Create(&store.Post{Slug: "pub", Title: "公开文", ContentHTML: "<p>正文甲</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(&store.Post{Slug: "locked", Title: "加密文", ContentHTML: "<p>秘密正文</p>", Password: "pw", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(&store.Post{Slug: "draft", Title: "草稿文", ContentHTML: "<p>正文乙</p>", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "about", Title: "关于", ContentHTML: "<p>正文丙</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "todo", Title: "待办", ContentHTML: "<p>正文丁</p>", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentHTML: "<p>瞬间甲</p>", Visibility: store.MomentVisibilityPublic}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentHTML: "<p>瞬间乙</p>", Visibility: store.MomentVisibilityPrivate}); err != nil {
		t.Fatal(err)
	}

	items := searchItemsAll(st)
	if len(items) != 4 {
		t.Fatalf("want 4 items (pub+locked+about+moment), got %d", len(items))
	}
	types := map[string]int{}
	for _, it := range items {
		types[it.Type]++
		if it.Type == search.TypePost && it.Slug == "locked" && it.Content != "" {
			t.Errorf("password post content should be empty")
		}
	}
	if types[search.TypePost] != 2 || types[search.TypePage] != 1 || types[search.TypeMoment] != 1 {
		t.Errorf("type distribution mismatch: %v", types)
	}
}
