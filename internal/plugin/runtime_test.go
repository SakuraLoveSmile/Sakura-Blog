package plugin

import (
	"context"
	"testing"
)

func validManifest(hooks ...Hook) Manifest {
	permissions := []string{PermissionScript}
	for _, hook := range hooks {
		permissions = append(permissions, "hook:"+string(hook))
	}
	return Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", Hooks: hooks, Permissions: permissions}
}

func TestRuntimeExecutesOnlyDeclaredHooks(t *testing.T) {
	runtime := NewRuntime()
	manifest := validManifest(HookContentFilter)
	if err := runtime.Load(manifest, `function filterContent(value) { return value + "!" }`); err != nil {
		t.Fatalf("load: %v", err)
	}
	value, err := runtime.Apply(context.Background(), HookContentFilter, "hello")
	if err != nil || value != "hello!" {
		t.Fatalf("apply = %#v, err=%v", value, err)
	}
	if _, err := runtime.Apply(context.Background(), HookShortcode, "x"); err != nil {
		t.Fatalf("undeclared hook should be a no-op: %v", err)
	}
}

func TestManifestRequiresHookAndNetworkPermissions(t *testing.T) {
	if err := ValidateManifest(Manifest{ID: "demo", Version: "1", Hooks: []Hook{HookWidget}, Permissions: []string{PermissionScript}}); err == nil {
		t.Fatal("missing hook permission should be rejected")
	}
	manifest := validManifest(HookWidget)
	manifest.NetworkAllowlist = []string{"example.com"}
	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("network allowlist without network permission should be rejected")
	}
}

func TestBundleManifestIsListedWithoutEnteringGoja(t *testing.T) {
	runtime := NewRuntime()
	manifest := Manifest{ID: "bundle", Name: "Bundle", Version: "1.0.0", Kind: "bundle", Bundle: "/plugins/bundle/dist/index.js"}
	if err := runtime.Load(manifest, ""); err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	items := runtime.Manifests()
	if len(items) != 1 || items[0].Bundle != manifest.Bundle {
		t.Fatalf("bundle manifests = %#v", items)
	}
}

func TestRuntimeHasNoFilesystemOrUnrestrictedNetworkGlobals(t *testing.T) {
	runtime := NewRuntime()
	manifest := validManifest(HookContentFilter)
	if err := runtime.Load(manifest, `function filterContent(value) { return typeof require + ":" + typeof process + ":" + typeof fetch }`); err != nil {
		t.Fatalf("load: %v", err)
	}
	value, err := runtime.Apply(context.Background(), HookContentFilter, "x")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if value != "undefined:undefined:undefined" {
		t.Fatalf("sandbox globals = %#v", value)
	}
}
