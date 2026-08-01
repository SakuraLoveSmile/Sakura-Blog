package store

import (
	"fmt"
	"strings"
)

// BackupTo 用 VACUUM INTO 把当前库写成一致性快照文件（WAL 模式下在线安全，不锁库）。
// path 必须是不存在的目标文件路径。路径内嵌 SQL（单引号转义），VACUUM 不支持参数绑定的驱动差异由此规避。
func (s *Store) BackupTo(path string) error {
	escaped := strings.ReplaceAll(path, "'", "''")
	if _, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return fmt.Errorf("vacuum into %s: %w", path, err)
	}
	return nil
}
