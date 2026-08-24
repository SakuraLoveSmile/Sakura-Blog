package post

import (
	"testing"

	"blog/internal/plugin"
)

func TestRenderAppliesScriptShortcodeAndContentHooks(t *testing.T) {
	runtime := plugin.NewRuntime()
	manifest := plugin.Manifest{
		ID: "content-filter", Name: "Content filter", Version: "1.0.0",
		Hooks:       []plugin.Hook{plugin.HookShortcode, plugin.HookContentFilter},
		Permissions: []string{plugin.PermissionScript, "hook:shortcode", "hook:content_filter"},
	}
	source := `function onShortcode(value) { return value.replace("hello", "**hello**") }
function filterContent(value) { return value + "<!-- plugin-filter -->" }`
	if err := runtime.Load(manifest, source); err != nil {
		t.Fatalf("load runtime plugin: %v", err)
	}
	service := NewService(t.TempDir())
	service.SetPluginRuntime(runtime)
	html, err := service.Render("hello")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if html == "" || !contains(html, "<strong>hello</strong>") || !contains(html, "plugin-filter") {
		t.Fatalf("filtered HTML = %q", html)
	}
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
