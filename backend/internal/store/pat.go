package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// PAT 个人访问令牌持久化记录，对齐 pat_tokens 表字段。
// 只存 token_hash（sha256 hex），明文仅在创建时返回一次，DB 永不落明文。
type PAT struct {
	ID          int64
	Name        string
	TokenHash   string // sha256 hex，不可逆，仅认证查询用
	Description string
	Scopes      string    // 逗号分隔 scope 列表（当前仅 "read"）
	ExpiresAt   time.Time // 零值=永不过期
	CreatedAt   time.Time
	LastUsedAt  time.Time // 零值=从未使用
}

// patCols 统一 SELECT 列清单。
const patCols = "id, name, token_hash, description, scopes, expires_at, created_at, last_used_at"

// HashToken 对明文令牌做 sha256 摘要，返回 hex 串（认证查询与入库统一口径）。
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// formatPATTime 时间序列化：零值存空串（永不过期/从未使用），非零存 RFC3339 UTC。
func formatPATTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// parsePATTime 反序列化：空串返回零值；兼容 RFC3339 与 "2006-01-02 15:04:05" 等格式。
func parsePATTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// EnsurePATTable 独立建 pat_tokens 表（仿 users 表范式：CREATE TABLE IF NOT EXISTS，无 ALTER）。
func (s *Store) EnsurePATTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS pat_tokens (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		name         TEXT NOT NULL DEFAULT '',
		token_hash   TEXT NOT NULL UNIQUE,
		description  TEXT NOT NULL DEFAULT '',
		scopes       TEXT NOT NULL DEFAULT 'read',
		expires_at   TEXT NOT NULL DEFAULT '',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at TEXT NOT NULL DEFAULT ''
	);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create pat_tokens table: %w", err)
	}
	return nil
}

// CreatePAT 新建令牌，LastInsertId 回填 pat.ID。入参只携带 TokenHash（明文由调用方生成后仅返回一次）。
func (s *Store) CreatePAT(pat *PAT) error {
	now := time.Now().UTC()
	if pat.CreatedAt.IsZero() {
		pat.CreatedAt = now
	}
	if strings.TrimSpace(pat.Scopes) == "" {
		pat.Scopes = "read"
	}
	res, err := s.db.Exec(`INSERT INTO pat_tokens (name, token_hash, description, scopes, expires_at, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		pat.Name, pat.TokenHash, pat.Description, pat.Scopes, formatPATTime(pat.ExpiresAt), pat.CreatedAt, "")
	if err != nil {
		return fmt.Errorf("create pat %s: %w", pat.Name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create pat %s: last insert id: %w", pat.Name, err)
	}
	pat.ID = id
	return nil
}

// GetPATByHash 按令牌哈希查询（认证路径）。未找到返回 nil, nil。
func (s *Store) GetPATByHash(hash string) (*PAT, error) {
	row := s.db.QueryRow("SELECT "+patCols+" FROM pat_tokens WHERE token_hash = ?", hash)
	pat, err := scanPAT(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pat by hash: %w", err)
	}
	return pat, nil
}

// ListPATs 返回全部令牌（按创建时间倒序）。token_hash 不可逆，列表仅用于管理展示。
func (s *Store) ListPATs() ([]*PAT, error) {
	rows, err := s.db.Query("SELECT " + patCols + " FROM pat_tokens ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("list pats: %w", err)
	}
	defer rows.Close()
	var out []*PAT
	for rows.Next() {
		pat, err := scanPAT(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pat: %w", err)
		}
		out = append(out, pat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pats: %w", err)
	}
	return out, nil
}

// DeletePAT 按 ID 删除令牌（即吊销）。不存在不报错（幂等）。
func (s *Store) DeletePAT(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM pat_tokens WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete pat %d: %w", id, err)
	}
	return nil
}

// TouchPATLastUsed 更新令牌最后使用时间。令牌不存在时静默（并发吊销竞态下不报错）。
func (s *Store) TouchPATLastUsed(id int64) error {
	if _, err := s.db.Exec(`UPDATE pat_tokens SET last_used_at = ? WHERE id = ?`,
		formatPATTime(time.Now()), id); err != nil {
		return fmt.Errorf("touch pat %d last used: %w", id, err)
	}
	return nil
}

type patScanner interface {
	Scan(dest ...any) error
}

func scanPAT(row patScanner) (*PAT, error) {
	pat := &PAT{}
	var expiresAt, lastUsedAt string
	err := row.Scan(&pat.ID, &pat.Name, &pat.TokenHash, &pat.Description, &pat.Scopes, &expiresAt, &pat.CreatedAt, &lastUsedAt)
	if err != nil {
		return nil, err
	}
	pat.ExpiresAt = parsePATTime(expiresAt)
	pat.LastUsedAt = parsePATTime(lastUsedAt)
	return pat, nil
}
