package app

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"blog/internal/config"
	"golang.org/x/sys/unix"
)

// StellarMigrationConfig复用Bootstrap的路径优先级，且不打开Store。
func StellarMigrationConfig(options Options) (*config.App, string, error) {
	root := options.Root
	if root == "" {
		root = os.Getenv("BLOG_ROOT")
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, "", err
	}
	path := options.ConfigPath
	if path == "" {
		path = filepath.Join(root, "config", "app.yml")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	a, err := config.LoadApp(path)
	if err != nil {
		return nil, "", err
	}
	if options.Root != "" {
		a.Root = options.Root
	}
	return config.ResolveApp(a), path, nil
}

// StellarRootGuard是进程整个生命周期的资源锁，独立于备份/恢复操作锁。
// guard不放入业务root，不删除锁文件；旧二进制仍必须另行确认停机。
type StellarRootGuard struct {
	files       []*os.File
	exclusive   bool
	AppSnapshot *config.App
	AppPath     string
}

func canonicalGuardPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	remaining := []string{}
	for {
		resolved, e := filepath.EvalSymlinks(path)
		if e == nil {
			for i := len(remaining) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, remaining[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", e
		}
		remaining = append(remaining, filepath.Base(path))
		path = parent
	}
}

func AcquireStellarRootGuard(a *config.App, appPath string, exclusive bool) (*StellarRootGuard, error) {
	// 固定系统路径，不受各实例TMPDIR覆盖影响；macOS的/tmp会统一解析到/private/tmp。
	dir := filepath.Join("/tmp", "blog-stellar-runtime-"+strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return nil, fmt.Errorf("unsafe Stellar runtime guard directory")
	}
	var stat unix.Stat_t
	if unix.Lstat(dir, &stat) != nil || stat.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf("unsafe Stellar runtime guard owner")
	}
	paths := []string{}
	seen := map[string]bool{}
	canonical := make([]string, 0, 4)
	for _, p := range []string{a.Root, a.DBPath, a.UploadDir, appPath} {
		p, err = canonicalGuardPath(p)
		if err != nil {
			return nil, err
		}
		canonical = append(canonical, p)
		key := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(p)))
		if !seen[key] {
			seen[key] = true
			paths = append(paths, filepath.Join(dir, key))
		}
	}
	sort.Strings(paths)
	snapshot := *a
	snapshot.Root, snapshot.DBPath, snapshot.UploadDir = canonical[0], canonical[1], canonical[2]
	g := &StellarRootGuard{exclusive: exclusive, AppSnapshot: &snapshot, AppPath: canonical[3]}
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	for _, path := range paths {
		fd, e := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
		if e != nil {
			_ = g.Close()
			return nil, e
		}
		f := os.NewFile(uintptr(fd), path)
		var fileStat unix.Stat_t
		if e = unix.Fstat(fd, &fileStat); e != nil || fileStat.Uid != uint32(os.Getuid()) || fileStat.Mode&unix.S_IFMT != unix.S_IFREG || fileStat.Mode&0777 != 0600 {
			_ = f.Close()
			_ = g.Close()
			return nil, fmt.Errorf("unsafe Stellar lifetime guard file")
		}
		if e = unix.Flock(fd, mode|unix.LOCK_NB); e != nil {
			_ = f.Close()
			_ = g.Close()
			return nil, fmt.Errorf("stellar root is in use; stop its server and workers before migration: %w", e)
		}
		g.files = append(g.files, f)
	}
	return g, nil
}

func (g *StellarRootGuard) Downgrade() error {
	if g == nil || !g.exclusive {
		return fmt.Errorf("exclusive Stellar guard required")
	}
	for _, f := range g.files {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_SH); err != nil {
			return err
		}
	}
	g.exclusive = false
	return nil
}

func (g *StellarRootGuard) Exclusive() bool { return g != nil && g.exclusive && len(g.files) > 0 }
func (g *StellarRootGuard) Close() error {
	if g == nil {
		return nil
	}
	var first error
	for i := len(g.files) - 1; i >= 0; i-- {
		if e := g.files[i].Close(); e != nil && first == nil {
			first = e
		}
	}
	g.files = nil
	g.exclusive = false
	return first
}

// CheckStellarOfflineListen只证明本机配置地址当前可绑定，不能代替旧实例PID/root核验。
func CheckStellarOfflineListen(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(port) == "" || port == "0" {
		return fmt.Errorf("offline migration requires an explicit nonzero local listen port")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("configured listen address is occupied; stop the old server first")
	}
	return listener.Close()
}
