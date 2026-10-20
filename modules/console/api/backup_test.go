package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blog/internal/config"
)

// Wave 4 测试覆盖补强：备份导出/状态/导入/恢复。

// TestBackupExportStatus 备份导出与状态：zip 流式下载 + 状态接口。
func TestBackupExportStatus(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 全量导出 → 200 + zip 头
	w := do2FA(r, http.MethodGet, "/api/v1/backup/export", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("export: status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("export content-type = %q, want application/zip", ct)
	}
	if w.Body.Len() < 100 {
		t.Errorf("export zip suspiciously small: %d bytes", w.Body.Len())
	}
	// zip 魔数 PK\x03\x04
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Errorf("export body not a zip: prefix %q", w.Body.Bytes()[:4])
	}

	// Markdown 导出 → zip（无文章时为空 zip，EOCD 记录 PK\x05\x06 开头）
	w = do2FA(r, http.MethodGet, "/api/v1/backup/export/markdown", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("export markdown: status = %d", w.Code)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK")) {
		t.Errorf("markdown export not a zip: prefix %q", w.Body.Bytes()[:4])
	}

	// 状态：auto_backup + backups 数组
	w = do2FA(r, http.MethodGet, "/api/v1/backup/status", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("backup status: status = %d", w.Code)
	}
	var status struct {
		AutoBackup bool            `json:"auto_backup"`
		BackupKeep int             `json:"backup_keep"`
		Backups    json.RawMessage `json:"backups"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Backups == nil {
		t.Errorf("backups list should be present: %s", w.Body.String())
	}
}

// TestBackupRestoreValidation 恢复/导入参数校验：缺文件、缺 name、空 body。
func TestBackupRestoreValidation(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 缺上传文件 → 400
	w := do2FA(r, http.MethodPost, "/api/v1/backup/restore", "", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("restore without file: status = %d, want 400", w.Code)
	}
	// 空 multipart body（有 file 字段但为空文件）→ 400（Restore 校验失败）
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "backup.zip")
	fw.Write([]byte("not a zip"))
	mw.Close()
	w = do2FARaw(r, http.MethodPost, "/api/v1/backup/restore", &body, mw.FormDataContentType(), cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("restore bad zip: status = %d, want 400", w.Code)
	}

	// restore-snapshot：缺 name → 400；不存在 name → 400
	w = do2FA(r, http.MethodPost, "/api/v1/backup/restore-snapshot", `{}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("restore-snapshot empty: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/backup/restore-snapshot", `{"name":"no-such.zip"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("restore-snapshot missing: status = %d, want 400", w.Code)
	}

	// import markdown：缺文件 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/backup/import/markdown", "", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("import without file: status = %d, want 400", w.Code)
	}
	// 未迁移配置拒绝导入，不能通过上传绕过迁移预览。
	body.Reset()
	mw = multipart.NewWriter(&body)
	fw, _ = mw.CreateFormFile("file", "x.md")
	fw.Write([]byte("---\ntitle: Strict\n---\n正文"))
	mw.Close()
	w = do2FARaw(r, http.MethodPost, "/api/v1/backup/import/markdown", &body, mw.FormDataContentType(), cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("legacy configuration import: status=%d", w.Code)
	}
	if value, _ := st.GetBySlug("x"); value != nil {
		t.Fatal("unmigrated import wrote data")
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	// 新格式上传成功；实际经过完整候选和恢复任务，不用旧轻量 parser。
	body.Reset()
	mw = multipart.NewWriter(&body)
	fw, _ = mw.CreateFormFile("file", "x.md")
	fw.Write([]byte(";;;\n{\"title\":\"Strict\",\"blog\":{\"status\":\"draft\"}}\n;;;\n正文"))
	mw.Close()
	w = do2FARaw(r, http.MethodPost, "/api/v1/backup/import/markdown", &body, mw.FormDataContentType(), cookie)
	if w.Code != http.StatusOK {
		t.Errorf("import md: status = %d, want 200", w.Code)
	}
	var imp struct {
		Imported int `json:"imported"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &imp); err != nil || imp.Imported != 1 {
		t.Errorf("import result: %s (err %v), want imported=1", w.Body.String(), err)
	}
	value, err := st.GetBySlug("x")
	if err != nil || value == nil || value.StellarVersion != 2 || value.Status != "draft" {
		t.Fatal("strict upload did not persist canonical/draft metadata")
	}
}

func TestLegacyMarkdownMultipartRequiresExplicitFlagAndMigration(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	upload := func(legacy, cookie string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("legacy", legacy)
		entry, _ := writer.CreateFormFile("file", "original.md")
		_, _ = entry.Write([]byte("---\nslug: imported\npin: true\n---\nbody"))
		_ = writer.Close()
		return do2FARaw(r, http.MethodPost, "/api/v1/backup/import/markdown", &body, writer.FormDataContentType(), cookie)
	}
	if w := upload("true", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous import=%d", w.Code)
	}
	if w := upload("true", authCookie(user)); w.Code != http.StatusBadRequest {
		t.Fatalf("unmigrated import=%d", w.Code)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if w := upload("false", authCookie(user)); w.Code != http.StatusBadRequest {
		t.Fatalf("strict fell back=%d", w.Code)
	} else {
		var problem Problem
		if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || problem.Type != "legacy_markdown_requires_conversion" || !strings.Contains(problem.Detail, "1.x") {
			t.Fatalf("missing actionable legacy conversion response: %s (%v)", w.Body.String(), err)
		}
		if value, _ := st.GetBySlug("imported"); value != nil {
			t.Fatal("conversion suggestion wrote a post before confirmation")
		}
	}
	if w := upload("true", authCookie(user)); w.Code != http.StatusOK {
		t.Fatalf("legacy import=%d %s", w.Code, w.Body.String())
	}
	value, _ := st.GetBySlug("imported")
	if value == nil || value.Status != "draft" || value.StellarVersion != 2 {
		t.Fatal("legacy multipart did not safely import new draft")
	}
}

func TestMarkdownImportConversionHintKeepsStrictErrors(t *testing.T) {
	for _, test := range []struct {
		name, header, problemType string
	}{
		{"legacy-render", "mathjax: true", "legacy_markdown_requires_conversion"},
		{"legacy-banner", "banner: /banner.jpg", "legacy_markdown_requires_conversion"},
		{"unknown-field", "mystery: true", "validation_error"},
		{"invalid-v2-render", "render: {math: true}", "validation_error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, st, user, _ := newFullEnv(t)
			if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
				t.Fatal(err)
			}
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			entry, _ := writer.CreateFormFile("file", "hint.md")
			_, _ = entry.Write([]byte("---\n" + test.header + "\n---\nbody"))
			_ = writer.Close()
			w := do2FARaw(r, http.MethodPost, "/api/v1/backup/import/markdown", &body, writer.FormDataContentType(), authCookie(user))
			var problem Problem
			if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || w.Code != http.StatusBadRequest || problem.Type != test.problemType {
				t.Fatalf("unexpected conversion hint: %d %s (%v)", w.Code, w.Body.String(), err)
			}
			if value, _ := st.GetBySlug("hint"); value != nil {
				t.Fatal("rejected import wrote data")
			}
		})
	}
}
