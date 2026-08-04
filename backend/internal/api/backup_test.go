package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"
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
	r, _, user, _ := newFullEnv(t)
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
	// 上传 .md（二进制内容也被当作正文导入，slug 取文件名 x）→ 200 + imported
	body.Reset()
	mw = multipart.NewWriter(&body)
	fw, _ = mw.CreateFormFile("file", "x.md")
	fw.Write([]byte{0xff, 0xfe})
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
}
