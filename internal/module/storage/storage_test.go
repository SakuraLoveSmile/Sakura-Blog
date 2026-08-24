package storage

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"blog/internal/module"
	attachmentstorage "blog/internal/storage"
	"blog/internal/store"
)

func TestStorageModuleConfigIsSchemaDrivenAndSecretIsMasked(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	mgr, err := NewManager(st, t.TempDir())
	if err != nil {
		t.Fatalf("new storage manager: %v", err)
	}
	mod := &Module{}
	meta := mod.Meta()
	if meta.ConfigSection != settingKey || len(meta.ConfigSchema) < 8 {
		t.Fatalf("storage config metadata = %+v", meta)
	}
	deps := module.Deps{Store: st, Storage: mgr}

	input := attachmentstorage.Config{
		Provider: attachmentstorage.ProviderLocal, Region: "us-east-1", SecretKey: "stored-secret", Prefix: "attachments",
	}
	raw, _ := json.Marshal(input)
	if err := mod.PutConfig(deps, raw); err != nil {
		t.Fatalf("put config: %v", err)
	}
	value, err := mod.GetConfig(deps)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	got := value.(attachmentstorage.Config)
	if got.SecretKey != attachmentstorage.MaskValue {
		t.Fatalf("secret key = %q, want masked", got.SecretKey)
	}
	if mgr.Config().SecretKey != "stored-secret" {
		t.Fatalf("runtime secret was not retained: %q", mgr.Config().SecretKey)
	}
	if raw, err := st.GetSetting(settingKey); err != nil || raw == "" {
		t.Fatalf("persisted config missing: raw=%q err=%v", raw, err)
	}

	// A form submission containing the masked value must preserve the real key.
	if err := mod.PutConfig(deps, []byte(`{"provider":"local","secret_key":"****","prefix":"next"}`)); err != nil {
		t.Fatalf("put masked config: %v", err)
	}
	if mgr.Config().SecretKey != "stored-secret" || mgr.Config().Prefix != "next" {
		t.Fatalf("masked update changed runtime secret: %+v", mgr.Config())
	}
}

func TestStorageModuleSatisfiesConfigModule(t *testing.T) {
	var _ module.Module = (*Module)(nil)
	var _ module.ConfigModule = (*Module)(nil)
}
