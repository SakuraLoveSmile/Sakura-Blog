package stellar

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"blog/modules/stellar/web/assets"
)

func TestRuntimeVersionIncludesImportedModules(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "modules", "stellar", "web", "assets", "js", "runtime")
	if err := os.MkdirAll(filepath.Join(dir, "extensions"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.js", "import './extensions/shell.js';")
	write("extensions/shell.js", "export const version = 1;")
	version := NewAssetVersioner(true, root)
	before := version("/assets/js/runtime/index.js")
	if before == "0" || before != version("/assets/js/runtime/index.js") {
		t.Fatal("runtime fingerprint must be stable and nonzero")
	}
	write("extensions/shell.js", "export const version = 2;")
	if before == version("/assets/js/runtime/index.js") {
		t.Fatal("an imported module change must invalidate cached runtime")
	}
	write("README.md", "provenance only")
	after := version("/assets/js/runtime/index.js")
	write("README.md", "updated provenance")
	if after != version("/assets/js/runtime/index.js") {
		t.Fatal("documentation must not invalidate runtime")
	}
}

func TestRuntimeVersionIncludesDynamicallyLoadedAssets(t *testing.T) {
	for _, name := range []string{
		"js/search/algolia-search.js",
		"js/services/siteinfo.js",
		"js/plugins/fancybox.js",
		"vendor/fancybox/fancybox.umd.js",
		"vendor/fancybox/fancybox.css",
		"css/plugins/fancybox.css",
		"css/comments/waline.css",
		"fonts/font.css",
		"js/icons/default.json",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "modules", "stellar", "web", "assets")
			write := func(name, content string) {
				t.Helper()
				path := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("js/runtime/index.js", "export const runtime = 1;")
			write(name, "first content")
			version := NewAssetVersioner(true, root)
			before := version("/assets/js/runtime/index.js")
			write(name, "changed content")
			if before == "0" || before == version("/assets/js/runtime/index.js") {
				t.Fatal("dynamic asset changes must invalidate the inherited runtime version")
			}
			before = version("/assets/js/runtime/index.js")
			write("vendor/README.md", "documentation")
			write("img/icon.svg", "<svg/>")
			if before != version("/assets/js/runtime/index.js") {
				t.Fatal("documentation and independently referenced images must not invalidate runtime")
			}
		})
	}
}

func TestRuntimeVersionMatchesDiskAndEmbed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "modules", "stellar", "web", "assets")
	if err := fs.WalkDir(assets.FS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(path, 0755)
		}
		data, err := assets.FS.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	dev := NewAssetVersioner(true, root)
	embedded := NewAssetVersioner(false, "")
	for _, path := range []string{"/assets/js/runtime/index.js", "assets/js/runtime/index.js", "/assets/js/search/algolia-search.js", "/assets/css/main.css"} {
		if got, want := dev(path), embedded(path); got == "0" || got != want {
			t.Errorf("disk and embed must agree for %s: got %s, want %s", path, got, want)
		}
	}
}
