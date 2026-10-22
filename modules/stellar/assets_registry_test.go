package stellar

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/store"
	"blog/modules/stellar/web"

	"github.com/gin-gonic/gin"
)

func TestMountedRuntimeVersionIncludesGeneratedIconRegistry(t *testing.T) {
	t.Cleanup(func() { web.SetAssetVersioner(nil) })
	var versions []string
	for _, source := range []string{
		"default:search: <svg><path id=\"first\"/></svg>\nother:icon: <svg/>\n",
		"other:icon: <svg/>\ndefault:search: <svg><path id=\"first\"/></svg>\n",
		"default:search: <svg><path id=\"changed\"/></svg>\nother:icon: <svg/>\n",
	} {
		root := t.TempDir()
		iconPath := filepath.Join(root, "icons.yml")
		if err := os.WriteFile(iconPath, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		icons, err := settings.LoadIcons(iconPath)
		if err != nil {
			t.Fatal(err)
		}
		st, err := store.New(filepath.Join(root, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		for _, ensure := range []func() error{st.EnsureSchema, st.EnsureWikiProjectsTable, st.EnsureWikidocsTable, st.EnsureTopicsTable, st.EnsureNotebooksTable, st.EnsureNotesTable, st.EnsureMomentsTable} {
			if err := ensure(); err != nil {
				t.Fatal(err)
			}
		}
		canonical, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		site := new(atomic.Pointer[settings.Site])
		site.Store(&settings.Site{Title: "Fixture", URL: "https://fixture.test/", StellarV2: canonical})
		router := gin.New()
		server, err := Mount(router, Dependencies{Root: root, App: &config.App{Root: root, DBPath: filepath.Join(root, "test.db")}, Site: site, Store: st, Posts: post.NewService(""), Icons: icons})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(server.Close)
		version := web.AssetVersion("/assets/js/runtime/index.js")
		versions = append(versions, version)
		generated, err := web.GeneratedStellarAssets(icons)
		if err != nil {
			t.Fatal(err)
		}
		for _, asset := range generated {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+asset.Path+"?v="+version, nil))
			if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), asset.Body) {
				t.Fatalf("generated %s does not match the mounted registry: status=%d", asset.Path, response.Code)
			}
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "js/runtime/index.js?v="+version) {
			t.Fatalf("SSR did not emit the mounted runtime version: status=%d", response.Code)
		}
		server.Close()
		st.Close()
	}
	if versions[0] == "0" || versions[0] != versions[1] {
		t.Fatalf("registry declaration order changed runtime fingerprint: %v", versions)
	}
	if versions[0] == versions[2] {
		t.Fatal("changed production icon registry must invalidate the runtime and generated JSON query")
	}
}

func TestIconAssetVersionerPropagatesGenerationErrors(t *testing.T) {
	if _, err := newIconAssetVersioner(false, "", nil); err == nil {
		t.Fatal("missing registry must not be replaced with an inferred default")
	}
	path := filepath.Join(t.TempDir(), "icons.yml")
	if err := os.WriteFile(path, []byte(".:bad: <svg/>\n"), 0600); err != nil {
		t.Fatal(err)
	}
	icons, err := settings.LoadIcons(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = newIconAssetVersioner(false, "", icons); err == nil {
		t.Fatal("invalid generated namespace must propagate its error")
	}
}
