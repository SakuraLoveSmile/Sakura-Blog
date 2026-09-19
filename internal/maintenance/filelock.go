package maintenance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// LockOperation 与系统更新器共用同一 inode 上的 flock；不跟随锁文件符号链接。
func LockOperation(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create operation lock directory: %w", err)
	}
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, fmt.Errorf("open operation lock: %w", err)
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		unix.Close(fd)
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock maintenance operation: %w", err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = unix.Flock(fd, unix.LOCK_UN); _ = unix.Close(fd) }) }, nil
}
