package store

import (
	"path/filepath"
	"testing"
)

// newMomentStore 建库并确保 moments 表（含 Spec 43 的 images/visibility 迁移）。
func newMomentStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestEnsureMomentsTableIdempotent 重复建表 + 迁移应幂等（旧库不报错）。
func TestEnsureMomentsTableIdempotent(t *testing.T) {
	s := newMomentStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureMomentsTable(); err != nil {
			t.Fatalf("ensure moments table (iter %d): %v", i, err)
		}
	}
}

// TestMomentDefaultVisibilityPublic 未指定可见性时缺省 public。
func TestMomentDefaultVisibilityPublic(t *testing.T) {
	s := newMomentStore(t)
	m := &Moment{ContentMD: "hi", ContentHTML: "<p>hi</p>"}
	if _, err := s.CreateMoment(m); err != nil {
		t.Fatalf("create moment: %v", err)
	}
	if m.Visibility != MomentVisibilityPublic {
		t.Fatalf("want default visibility public, got %q", m.Visibility)
	}
	got, err := s.GetMoment(m.ID)
	if err != nil || got == nil {
		t.Fatalf("get moment: %v (nil=%v)", err, got == nil)
	}
	if got.Visibility != MomentVisibilityPublic {
		t.Fatalf("persisted visibility want public, got %q", got.Visibility)
	}
}

// TestListPublicMomentsFiltersPrivate 私密瞬间不出现在公开查询/计数中，但计入全量。
func TestListPublicMomentsFiltersPrivate(t *testing.T) {
	s := newMomentStore(t)
	if _, err := s.CreateMoment(&Moment{ContentMD: "pub", Visibility: MomentVisibilityPublic}); err != nil {
		t.Fatalf("create public: %v", err)
	}
	if _, err := s.CreateMoment(&Moment{ContentMD: "priv", Visibility: MomentVisibilityPrivate}); err != nil {
		t.Fatalf("create private: %v", err)
	}

	pub, err := s.ListPublicMoments(1, 20)
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	if len(pub) != 1 {
		t.Fatalf("public list want 1 item, got %d", len(pub))
	}
	if pub[0].ContentMD != "pub" {
		t.Fatalf("public list should only contain public moment, got %q", pub[0].ContentMD)
	}

	all, err := s.ListMoments(1, 20)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("full list want 2 items (incl private), got %d", len(all))
	}

	pubCount, err := s.CountPublicMoments()
	if err != nil {
		t.Fatalf("count public: %v", err)
	}
	if pubCount != 1 {
		t.Fatalf("public count want 1, got %d", pubCount)
	}
	allCount, err := s.CountMoments()
	if err != nil {
		t.Fatalf("count all: %v", err)
	}
	if allCount != 2 {
		t.Fatalf("all count want 2, got %d", allCount)
	}
}

// TestUpdateMomentVisibilityAndImages 编辑瞬间可切换可见性并增删图片。
func TestUpdateMomentVisibilityAndImages(t *testing.T) {
	s := newMomentStore(t)
	m := &Moment{ContentMD: "x", Images: EncodeMomentImages([]string{"/uploads/a.png"}), Visibility: MomentVisibilityPublic}
	if _, err := s.CreateMoment(m); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 切私密 + 增图
	m.Visibility = MomentVisibilityPrivate
	m.Images = EncodeMomentImages([]string{"/uploads/a.png", "/uploads/b.png"})
	if err := s.UpdateMoment(m); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetMoment(m.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.Visibility != MomentVisibilityPrivate {
		t.Fatalf("visibility want private, got %q", got.Visibility)
	}
	imgs := DecodeMomentImages(got.Images)
	if len(imgs) != 2 || imgs[0] != "/uploads/a.png" || imgs[1] != "/uploads/b.png" {
		t.Fatalf("images after update mismatch: %#v", imgs)
	}
	// 切回后应从公开列表消失（此刻是 private）
	pub, _ := s.ListPublicMoments(1, 20)
	if len(pub) != 0 {
		t.Fatalf("private moment should not appear in public list, got %d", len(pub))
	}
}

// TestMomentImagesEncodeDecode JSON 编解码 + 兼容旧库逗号分隔。
func TestMomentImagesEncodeDecode(t *testing.T) {
	enc := EncodeMomentImages([]string{" /a.png ", "", "/b.png"})
	dec := DecodeMomentImages(enc)
	if len(dec) != 2 || dec[0] != "/a.png" || dec[1] != "/b.png" {
		t.Fatalf("encode/decode mismatch: enc=%q dec=%#v", enc, dec)
	}
	// 空切片编码为空串
	if EncodeMomentImages(nil) != "" {
		t.Fatalf("empty encode should be empty string")
	}
	// 兼容旧库逗号分隔
	legacy := DecodeMomentImages("/a.png,/b.png ,")
	if len(legacy) != 2 || legacy[0] != "/a.png" || legacy[1] != "/b.png" {
		t.Fatalf("legacy comma decode mismatch: %#v", legacy)
	}
}
