// Package plugin contains the deliberately narrow JavaScript plugin runtime.
// Scripts are hook handlers only: the Goja sandbox exposes no filesystem,
// process, module loader, or unrestricted network object.
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"

	"blog/internal/safefetch"
)

type Hook string

const (
	HookShortcode           Hook = "shortcode"
	HookWidget              Hook = "widget"
	HookContentFilter       Hook = "content_filter"
	HookNotificationWebhook Hook = "notification_webhook"
)

const (
	PermissionScript  = "plugin:script"
	PermissionNetwork = "network:fetch"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,63}$`)

// Manifest is the only capability declaration accepted by the runtime.
type Manifest struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	Kind             string   `json:"kind,omitempty"`
	Bundle           string   `json:"bundle,omitempty"`
	Script           string   `json:"script"`
	Hooks            []Hook   `json:"hooks"`
	Permissions      []string `json:"permissions"`
	NetworkAllowlist []string `json:"network_allowlist,omitempty"`
}

type loadedPlugin struct {
	manifest Manifest
	source   string
	program  *goja.Program
}

type Runtime struct {
	mu      sync.RWMutex
	plugins map[string]*loadedPlugin
	timeout time.Duration
	maxBody int64
}

func NewRuntime() *Runtime {
	return &Runtime{plugins: map[string]*loadedPlugin{}, timeout: 250 * time.Millisecond, maxBody: 1 << 20}
}

func (r *Runtime) Load(manifest Manifest, source string) error {
	if manifest.Kind == "" && strings.TrimSpace(source) != "" {
		manifest.Kind = "script"
	}
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	if manifest.Kind == "bundle" {
		r.mu.Lock()
		r.plugins[manifest.ID] = &loadedPlugin{manifest: manifest}
		r.mu.Unlock()
		return nil
	}
	if strings.TrimSpace(source) == "" {
		return errors.New("plugin script is empty")
	}
	if len(source) > 512<<10 {
		return errors.New("plugin script exceeds 512 KiB")
	}
	program, err := goja.Compile(manifest.ID, source, false)
	if err != nil {
		return fmt.Errorf("compile plugin %s: %w", manifest.ID, err)
	}
	r.mu.Lock()
	r.plugins[manifest.ID] = &loadedPlugin{manifest: manifest, source: source, program: program}
	r.mu.Unlock()
	return nil
}

func (r *Runtime) Remove(id string) {
	r.mu.Lock()
	delete(r.plugins, id)
	r.mu.Unlock()
}

func (r *Runtime) Manifests() []Manifest {
	r.mu.RLock()
	items := make([]Manifest, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		items = append(items, plugin.manifest)
	}
	r.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// Apply runs matching hook handlers in stable plugin-id order. A handler may
// transform the value; returning undefined leaves the current value unchanged.
func (r *Runtime) Apply(ctx context.Context, hook Hook, value any) (any, error) {
	if !validHook(hook) {
		return nil, fmt.Errorf("unknown plugin hook %q", hook)
	}
	r.mu.RLock()
	plugins := make([]*loadedPlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		if hasHook(plugin.manifest, hook) {
			plugins = append(plugins, plugin)
		}
	}
	r.mu.RUnlock()
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].manifest.ID < plugins[j].manifest.ID })

	current := value
	for _, plugin := range plugins {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, changed, err := r.runOne(ctx, plugin, hook, current)
		if err != nil {
			return nil, fmt.Errorf("plugin %s hook %s: %w", plugin.manifest.ID, hook, err)
		}
		if changed {
			current = result
		}
	}
	return current, nil
}

func (r *Runtime) runOne(ctx context.Context, plugin *loadedPlugin, hook Hook, input any) (result any, changed bool, err error) {
	runtime := goja.New()
	runtime.Set("plugin", map[string]any{"id": plugin.manifest.ID, "version": plugin.manifest.Version})
	if hasPermission(plugin.manifest, PermissionNetwork) {
		runtime.Set("fetch", r.fetchFunction(ctx, plugin.manifest))
	}
	timer := time.AfterFunc(r.timeout, func() { runtime.Interrupt("plugin execution timed out") })
	defer timer.Stop()
	if _, err := runtime.RunProgram(plugin.program); err != nil {
		return nil, false, err
	}
	functionValue := runtime.Get(functionName(hook))
	function, ok := goja.AssertFunction(functionValue)
	if !ok {
		return nil, false, fmt.Errorf("%s must be a function", functionName(hook))
	}
	value, err := function(goja.Undefined(), runtime.ToValue(input))
	if err != nil {
		return nil, false, err
	}
	if value == nil || goja.IsUndefined(value) {
		return input, false, nil
	}
	return value.Export(), true, nil
}

func (r *Runtime) fetchFunction(ctx context.Context, manifest Manifest) func(string) (string, error) {
	return func(raw string) (string, error) {
		u, err := safefetch.ValidateURL(raw)
		if err != nil {
			return "", err
		}
		if !hostAllowed(u.Hostname(), manifest.NetworkAllowlist) {
			return "", fmt.Errorf("network host %q is not in plugin allowlist", u.Hostname())
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}
		response, err := safefetch.New(safefetch.Options{
			Timeout: 10 * time.Second, MaxRedirects: 3,
			RedirectValidator: func(redirectURL *url.URL) error {
				if _, err := safefetch.ValidateURL(redirectURL.String()); err != nil {
					return err
				}
				if !hostAllowed(redirectURL.Hostname(), manifest.NetworkAllowlist) {
					return fmt.Errorf("network redirect host %q is not in plugin allowlist", redirectURL.Hostname())
				}
				return nil
			},
		}).Do(request)
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return "", fmt.Errorf("network request returned HTTP %d", response.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, r.maxBody+1))
		if err != nil {
			return "", err
		}
		if int64(len(body)) > r.maxBody {
			return "", errors.New("plugin network response is too large")
		}
		return string(body), nil
	}
}

func ValidateManifest(manifest Manifest) error {
	if !pluginIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("invalid plugin id %q", manifest.ID)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return errors.New("plugin version is required")
	}
	kind := manifest.Kind
	if kind == "" {
		kind = "script"
		if manifest.Script == "" {
			kind = "bundle"
		}
	}
	if kind != "script" && kind != "bundle" {
		return fmt.Errorf("plugin %s has unknown kind %q", manifest.ID, kind)
	}
	if kind == "bundle" {
		if !strings.HasPrefix(manifest.Bundle, "/plugins/") || len(manifest.Hooks) > 0 {
			return fmt.Errorf("bundle plugin %s must declare a same-origin bundle and no script hooks", manifest.ID)
		}
		return nil
	}
	if !hasPermission(manifest, PermissionScript) {
		return fmt.Errorf("plugin %s must declare %s", manifest.ID, PermissionScript)
	}
	seenHooks := map[Hook]bool{}
	for _, hook := range manifest.Hooks {
		if !validHook(hook) {
			return fmt.Errorf("plugin %s declares unknown hook %q", manifest.ID, hook)
		}
		if seenHooks[hook] {
			return fmt.Errorf("plugin %s declares duplicate hook %q", manifest.ID, hook)
		}
		seenHooks[hook] = true
		if !hasPermission(manifest, "hook:"+string(hook)) {
			return fmt.Errorf("plugin %s must declare hook:%s", manifest.ID, hook)
		}
	}
	if len(manifest.NetworkAllowlist) > 0 && !hasPermission(manifest, PermissionNetwork) {
		return fmt.Errorf("plugin %s declares a network allowlist without %s", manifest.ID, PermissionNetwork)
	}
	for _, host := range manifest.NetworkAllowlist {
		if strings.TrimSpace(host) == "" || strings.ContainsAny(host, "/:@") {
			return fmt.Errorf("plugin %s has invalid network host %q", manifest.ID, host)
		}
	}
	return nil
}

func (r *Runtime) LoadDir(root string) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(root, entry.Name())
		manifestBytes, err := os.ReadFile(filepath.Join(pluginDir, "manifest.json"))
		if err != nil {
			return fmt.Errorf("read plugin %s manifest: %w", entry.Name(), err)
		}
		var manifest Manifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return fmt.Errorf("decode plugin %s manifest: %w", entry.Name(), err)
		}
		if manifest.Kind == "" && manifest.Script == "" && manifest.Bundle != "" {
			manifest.Kind = "bundle"
		}
		if manifest.Kind == "bundle" {
			if err := r.Load(manifest, ""); err != nil {
				return err
			}
			continue
		}
		if manifest.Script == "" {
			manifest.Script = "main.js"
		}
		if filepath.Base(manifest.Script) != manifest.Script {
			return fmt.Errorf("plugin %s script must stay inside its directory", manifest.ID)
		}
		source, err := os.ReadFile(filepath.Join(pluginDir, manifest.Script))
		if err != nil {
			return fmt.Errorf("read plugin %s script: %w", manifest.ID, err)
		}
		if err := r.Load(manifest, string(source)); err != nil {
			return err
		}
	}
	return nil
}

func validHook(hook Hook) bool {
	switch hook {
	case HookShortcode, HookWidget, HookContentFilter, HookNotificationWebhook:
		return true
	default:
		return false
	}
}

func functionName(hook Hook) string {
	switch hook {
	case HookShortcode:
		return "onShortcode"
	case HookWidget:
		return "onWidget"
	case HookContentFilter:
		return "filterContent"
	case HookNotificationWebhook:
		return "onNotification"
	default:
		return ""
	}
}

func hasHook(manifest Manifest, hook Hook) bool {
	for _, declared := range manifest.Hooks {
		if declared == hook {
			return true
		}
	}
	return false
}

func hasPermission(manifest Manifest, permission string) bool {
	for _, declared := range manifest.Permissions {
		if declared == permission {
			return true
		}
	}
	return false
}

func hostAllowed(host string, allowlist []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, allowed := range allowlist {
		allowed = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(allowed), "."))
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}
