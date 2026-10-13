package stellarmigration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const RootCaptureVersion = 1

// RootLayout 必须来自composition root的已解析配置，而不是请求指定的目标目录。
// root外的DB/uploads/app文件按各自真实anchor捕获，路径也参与批准摘要。
type RootLayout struct {
	Root              string   `json:"root"`
	DBPath            string   `json:"db_path"`
	UploadDir         string   `json:"upload_dir"`
	AppPath           string   `json:"app_path"`
	OperationLock     string   `json:"operation_lock"`
	ExtraRuntimePaths []string `json:"extra_runtime_paths,omitempty"`
}

type RootResource struct {
	Path        string `json:"path"`
	VirtualPath string `json:"virtual_path"`
	Kind        string `json:"kind"`
	Mode        uint32 `json:"mode"`
	Data        []byte `json:"data,omitempty"`
}

type DatabaseObject struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Table string `json:"table"`
	SQL   string `json:"sql"`
}

type DatabaseState struct {
	Encoding      string `json:"encoding"`
	UserVersion   int64  `json:"user_version"`
	ApplicationID int64  `json:"application_id"`
}

// RootCapture是私密完整资源前像，包含配置密钥/上传/正文/DB bytes，禁止公开输出。
type RootCapture struct {
	Version          int              `json:"version"`
	Layout           RootLayout       `json:"layout"`
	Source           Snapshot         `json:"source"`
	TrackedSources   []string         `json:"tracked_sources"`
	Resources        []RootResource   `json:"resources"`
	Objects          []DatabaseObject `json:"database_objects"`
	DatabaseState    DatabaseState    `json:"database_state"`
	IgnoredLivePaths []string         `json:"ignored_live_paths"`
	Digest           string           `json:"digest"`
}

func (r RootCapture) ResourceDigest() string {
	r.Digest = ""
	return digest(r)
}
func (r RootCapture) Verify() error {
	if r.Version != RootCaptureVersion || r.Digest == "" || r.Digest != r.ResourceDigest() {
		return fmt.Errorf("migration root resource digest changed")
	}
	return r.Source.Verify()
}

func normalizeRootLayout(layout RootLayout) (RootLayout, error) {
	layout.ExtraRuntimePaths = append([]string(nil), layout.ExtraRuntimePaths...)
	original, err := filepath.Abs(layout.Root)
	if err != nil {
		return layout, err
	}
	info, err := os.Lstat(original)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return layout, fmt.Errorf("migration root must be an existing ordinary directory")
	}
	root, err := filepath.EvalSymlinks(original)
	if err != nil {
		return layout, err
	}
	layout.Root = root
	resolve := func(path, defaultPath string) (string, error) {
		if path == "" {
			path = defaultPath
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		} else if rel, e := filepath.Rel(original, path); e == nil && filepath.IsLocal(rel) {
			path = filepath.Join(root, rel)
		}
		path = filepath.Clean(path)
		if i, e := os.Lstat(path); e == nil && i.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("runtime resource anchor cannot be a symlink")
		}
		// 系统/祖先路径规范化；anchor以下随后逐组件拒绝符号链接。
		parent := filepath.Dir(path)
		if p, e := filepath.EvalSymlinks(parent); e == nil {
			path = filepath.Join(p, filepath.Base(path))
		}
		return path, nil
	}
	if layout.DBPath, err = resolve(layout.DBPath, "data/blog.db"); err != nil {
		return layout, err
	}
	if layout.UploadDir, err = resolve(layout.UploadDir, "data/uploads"); err != nil {
		return layout, err
	}
	if layout.AppPath, err = resolve(layout.AppPath, "config/app.yml"); err != nil {
		return layout, err
	}
	if layout.OperationLock, err = resolve(layout.OperationLock, filepath.Join(filepath.Dir(layout.DBPath), "maintenance.lock")); err != nil {
		return layout, err
	}
	for i, p := range layout.ExtraRuntimePaths {
		if layout.ExtraRuntimePaths[i], err = resolve(p, p); err != nil {
			return layout, err
		}
	}
	sort.Strings(layout.ExtraRuntimePaths)
	return layout, nil
}

func localTo(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	return relative, err == nil && (relative == "." || filepath.IsLocal(relative))
}
func resourceVirtualPath(layout RootLayout, path string) (string, error) {
	for _, anchor := range []struct{ path, prefix string }{{layout.Root, "root"}, {filepath.Dir(layout.DBPath), "database"}, {layout.UploadDir, "uploads"}} {
		if rel, ok := localTo(anchor.path, path); ok {
			return filepath.ToSlash(filepath.Join(anchor.prefix, rel)), nil
		}
	}
	if path == layout.AppPath {
		return "external-app/app.yml", nil
	}
	for i, extra := range layout.ExtraRuntimePaths {
		if rel, ok := localTo(extra, path); ok {
			return filepath.ToSlash(filepath.Join(fmt.Sprintf("extra/%d", i), rel)), nil
		}
	}
	return "", fmt.Errorf("runtime resource outside captured anchors")
}

func rootJournalPath(layout RootLayout) string {
	return filepath.Join(filepath.Dir(layout.DBPath), ".stellar-migration-journal.json")
}

// CaptureMigrationRoot仅mode=ro/query_only读取。双次完整资源扫描拒绝并发变动。
// SQLite主文件/WAL/journal bytes入前像；SHM和操作锁只列忽略路径，不删除/伪造零写。
func CaptureMigrationRoot(ctx context.Context, layout RootLayout, trackedSources []string) (RootCapture, error) {
	return captureMigrationRoot(ctx, layout, trackedSources, "")
}
func captureMigrationRoot(ctx context.Context, layout RootLayout, trackedSources []string, ownedTask string) (RootCapture, error) {
	layout, err := normalizeRootLayout(layout)
	if err != nil {
		return RootCapture{}, err
	}
	if ownedTask == "" {
		if _, e := os.Lstat(rootJournalPath(layout)); !os.IsNotExist(e) {
			return RootCapture{}, fmt.Errorf("migration journal exists; recovery required before capture")
		}
	}
	tracked := append([]string{}, trackedSources...)
	sort.Strings(tracked)
	for i, p := range tracked {
		if !filepath.IsLocal(p) || p != filepath.Clean(p) || !strings.HasPrefix(filepath.ToSlash(p), "content/") || (i > 0 && tracked[i-1] == p) {
			return RootCapture{}, fmt.Errorf("tracked migration source is invalid or duplicated")
		}
	}
	ignored := []string{layout.DBPath + "-shm", layout.OperationLock, rootJournalPath(layout)}
	resources, err := readRootResources(ctx, layout, ignored, ownedTask)
	if err != nil {
		return RootCapture{}, err
	}
	files := map[string]string{}
	byPath := map[string]RootResource{}
	for _, r := range resources {
		byPath[r.Path] = r
	}
	for _, source := range tracked {
		r, ok := byPath[filepath.Join(layout.Root, source)]
		if !ok || r.Kind != "file" {
			return RootCapture{}, fmt.Errorf("tracked source absent from full resource capture")
		}
		files[source] = string(r.Data)
	}
	snapshot, err := ReadSnapshot(ctx, layout.DBPath, files)
	if err != nil {
		return RootCapture{}, err
	}
	objects, state, err := readDatabaseObjects(ctx, layout.DBPath)
	if err != nil {
		return RootCapture{}, err
	}
	after, err := readRootResources(ctx, layout, ignored, ownedTask)
	if err != nil {
		return RootCapture{}, err
	}
	if !reflect.DeepEqual(resources, after) {
		beforeByPath := map[string]RootResource{}
		for _, resource := range resources {
			beforeByPath[resource.Path] = resource
		}
		for _, resource := range after {
			if previous, ok := beforeByPath[resource.Path]; !ok || !reflect.DeepEqual(previous, resource) {
				return RootCapture{}, fmt.Errorf("source runtime resources changed during capture: %s", resource.VirtualPath)
			}
			delete(beforeByPath, resource.Path)
		}
		return RootCapture{}, fmt.Errorf("source runtime resources disappeared during capture")
	}
	r := RootCapture{Version: RootCaptureVersion, Layout: layout, Source: snapshot, TrackedSources: tracked, Resources: resources, Objects: objects, DatabaseState: state, IgnoredLivePaths: ignored}
	r.Digest = r.ResourceDigest()
	return r, nil
}

func readRootResources(ctx context.Context, layout RootLayout, ignored []string, ownedTask string) ([]RootResource, error) {
	paths := []string{filepath.Join(layout.Root, "config"), filepath.Join(layout.Root, "content"), filepath.Join(layout.Root, "plugins"), filepath.Join(layout.Root, "runtime"), filepath.Join(layout.Root, "cache"), filepath.Dir(layout.DBPath), layout.UploadDir, layout.AppPath,
		filepath.Join(layout.Root, "internal/settings/data/icons.yml"), filepath.Join(layout.Root, "internal/og/fonts/LXGWWenKai-Regular.ttf")}
	paths = append(paths, layout.ExtraRuntimePaths...)
	seen := map[string]RootResource{}
	ignoredPath := func(path string) bool {
		for _, p := range ignored {
			if path == p {
				return true
			}
		}
		if ownedTask != "" {
			_, ok := localTo(ownedTask, path)
			return ok
		}
		return false
	}
	var walk func(string) error
	walk = func(path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ignoredPath(path) {
			return nil
		}
		if _, exists := seen[path]; exists {
			return nil
		}
		virtual, err := resourceVirtualPath(layout, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			seen[path] = RootResource{Path: path, VirtualPath: virtual, Kind: "absent"}
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("runtime capture requires ordinary files/directories")
		}
		r := RootResource{Path: path, VirtualPath: virtual, Mode: uint32(info.Mode().Perm()), Kind: "file"}
		if info.IsDir() {
			r.Kind = "directory"
			seen[path] = r
			if err := noSymlinkPath(path); err != nil {
				return err
			}
			directory, err := os.OpenRoot(path)
			if err != nil {
				return err
			}
			input, err := directory.Open(".")
			if err != nil {
				directory.Close()
				return err
			}
			actual, err := input.Stat()
			if err != nil || !os.SameFile(info, actual) {
				input.Close()
				directory.Close()
				return fmt.Errorf("runtime directory identity changed during capture")
			}
			entries, err := input.ReadDir(-1)
			input.Close()
			directory.Close()
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := walk(filepath.Join(path, entry.Name())); err != nil {
					return err
				}
			}
			return nil
		}
		// os.Root保证锚点内路径保护；不跟随文件/目录符号链接。
		if err := noSymlinkPath(filepath.Dir(path)); err != nil {
			return err
		}
		anchor, err := os.OpenRoot(filepath.Dir(path))
		if err != nil {
			return err
		}
		file, err := anchor.Open(filepath.Base(path))
		anchor.Close()
		if err != nil {
			return err
		}
		current, err := file.Stat()
		if err != nil {
			file.Close()
			return err
		}
		if !os.SameFile(info, current) || !current.Mode().IsRegular() {
			file.Close()
			return fmt.Errorf("runtime file identity changed during capture")
		}
		data, err := io.ReadAll(file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		r.Data = data
		seen[path] = r
		return nil
	}
	for _, path := range paths {
		if err := walk(path); err != nil {
			return nil, err
		}
	}
	keys := sortedKeys(seen)
	out := make([]RootResource, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out, nil
}

func noSymlinkPath(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("runtime resource path contains a symlink")
		}
		if current == filepath.Dir(current) {
			return nil
		}
	}
}

func readDatabaseObjects(ctx context.Context, path string) ([]DatabaseObject, DatabaseState, error) {
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_pragma", "query_only(1)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, DatabaseState{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, DatabaseState{}, err
	}
	defer tx.Rollback()
	var queryOnly int
	if err = tx.QueryRowContext(ctx, "PRAGMA query_only").Scan(&queryOnly); err != nil || queryOnly != 1 {
		return nil, DatabaseState{}, fmt.Errorf("root objects require query_only")
	}
	var state DatabaseState
	for _, p := range []struct {
		query string
		value any
	}{{"PRAGMA encoding", &state.Encoding}, {"PRAGMA user_version", &state.UserVersion}, {"PRAGMA application_id", &state.ApplicationID}} {
		if err = tx.QueryRowContext(ctx, p.query).Scan(p.value); err != nil {
			return nil, state, err
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT type,name,tbl_name,coalesce(sql,'') FROM sqlite_master WHERE type IN ('index','view','trigger') ORDER BY type,name")
	if err != nil {
		return nil, state, err
	}
	defer rows.Close()
	objects := []DatabaseObject{}
	for rows.Next() {
		var object DatabaseObject
		if err = rows.Scan(&object.Kind, &object.Name, &object.Table, &object.SQL); err != nil {
			return nil, state, err
		}
		objects = append(objects, object)
	}
	return objects, state, rows.Err()
}
