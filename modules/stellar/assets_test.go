package stellar

import (
	"os"
	"path/filepath"
	"testing"
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
