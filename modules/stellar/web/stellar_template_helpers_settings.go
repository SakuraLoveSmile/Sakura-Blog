package web

import (
	"fmt"
	"html/template"
	"strings"
)

func (r stellarTemplateRenderer) settingsPage(d *StellarRenderData) template.HTML {
	provider := stellarText(stellarGet(d.Stellar.Theme, "comments.provider"))
	supported := provider == "artalk" || provider == "waline" || provider == "twikoo"
	fallback := stellarURL(d.Stellar.Root, stellarGet(d.Stellar.Theme, "fallbacks.avatar"))
	var b strings.Builder
	fmt.Fprintf(&b, `<article class="md-text content settings-page" data-provider="%s" data-fallback-avatar="%s"`, stellarEscape(provider), stellarEscape(fallback))
	for _, key := range []string{"success", "partial", "failure"} {
		fmt.Fprintf(&b, ` data-message-%s="%s"`, key, stellarEscape(stellarTranslate(d, "settings.result."+key)))
	}
	for _, key := range []string{"name", "email", "url"} {
		fmt.Fprintf(&b, ` data-validation-%s="%s"`, key, stellarEscape(stellarTranslate(d, "settings.validation."+key)))
	}
	fmt.Fprintf(&b, `><header class="settings-page__header"><h1>%s</h1></header>`, stellarEscape(stellarTranslate(d, "settings.title")))
	if supported {
		fmt.Fprintf(&b, `<section class="settings-section" aria-labelledby="settings-profile"><h2 id="settings-profile">%s</h2><div class="settings-section__body"><div class="settings-profile"><div class="settings-profile__avatar" aria-label="%s"><img data-profile-avatar src="%s" alt=""><span data-profile-avatar-fallback hidden>%s</span></div><div class="settings-profile__content"><form data-profile-form class="settings-form" novalidate>`, stellarEscape(stellarTranslate(d, "settings.profile.title")), stellarEscape(stellarTranslate(d, "settings.profile.avatar")), stellarEscape(fallback), r.icon("default:profile"))
		for _, field := range []struct{ name, kind, label, attrs string }{{"name", "text", "nickname", ` maxlength="80" required autocomplete="nickname"`}, {"email", "email", "email", ` maxlength="254" autocomplete="email"`}, {"url", "url", "url", ` inputmode="url" autocomplete="url" placeholder="https://"`}} {
			fmt.Fprintf(&b, `<label><span>%s</span><input name="%s" type="%s"%s></label>`, stellarEscape(stellarTranslate(d, "settings.profile."+field.label)), field.name, field.kind, field.attrs)
		}
		fmt.Fprintf(&b, `<div class="settings-actions"><button type="submit" class="settings-button settings-button--primary">%s</button><button type="button" class="settings-button" data-profile-logout>%s</button></div></form><p class="settings-result" data-profile-status role="status" aria-live="polite"></p></div></div></div></section>`, stellarEscape(stellarTranslate(d, "settings.profile.save")), stellarEscape(stellarTranslate(d, "settings.profile.logout")))
	}
	fmt.Fprintf(&b, `<section class="settings-section" aria-labelledby="settings-cache"><h2 id="settings-cache">%s</h2><div class="settings-section__body"><dl class="settings-kv settings-cache-list">`, stellarEscape(stellarTranslate(d, "settings.cache.title")))
	for _, id := range []string{"search", "dynamic", "all"} {
		label := stellarTranslate(d, "settings.cache."+id)
		fmt.Fprintf(&b, `<div class="settings-kv__row"><dt>%s</dt><dd><span class="settings-kv__value" data-cache-size="%s">0 B</span><button type="button" class="settings-kv__action" data-cache-action="%s" aria-label="%s"><span class="ui-icon">%s</span></button></dd></div>`, stellarEscape(label), id, id, stellarEscape(stellarTranslate(d, "settings.cache.clear", label)), r.icon("default:trash"))
	}
	fmt.Fprintf(&b, `</dl><p class="settings-result" data-cache-status role="status" aria-live="polite"></p></div></section><section class="settings-section" aria-labelledby="settings-about"><h2 id="settings-about">%s</h2><div class="settings-section__body">`, stellarEscape(stellarTranslate(d, "settings.about.title")))
	items := stellarList(stellarGet(d.Stellar.Theme, "profiles.settings.about.items"))
	if len(items) == 0 {
		fmt.Fprintf(&b, `<p class="settings-empty">%s</p>`, stellarEscape(stellarTranslate(d, "settings.about.empty")))
	} else {
		b.WriteString(`<dl class="settings-kv">`)
		for _, raw := range items {
			item := stellarMap(raw)
			value := stellarSettingsValue(d, item["value"])
			target := stellarSettingsValue(d, item["url"])
			fmt.Fprintf(&b, `<div class="settings-kv__row"><dt>%s</dt><dd><span class="settings-kv__value">%s</span>`, stellarEscape(item["key"]), stellarEscape(value))
			if target != "" {
				attrs := ` rel="noopener noreferrer"`
				if strings.Contains(target, "://") {
					attrs = ` target="_blank" rel="external nofollow noopener noreferrer"`
				}
				fmt.Fprintf(&b, `<a class="settings-kv__link" href="%s"%s aria-label="%s"><span class="ui-icon">%s</span></a>`, stellarEscape(stellarURL(d.Stellar.Root, target)), attrs, stellarEscape(stellarTranslate(d, "settings.about.open_link", item["key"])), r.icon("default:link"))
			}
			b.WriteString(`</dd></div>`)
		}
		b.WriteString(`</dl>`)
	}
	b.WriteString(`</div></section></article>`)
	return template.HTML(b.String())
}
func stellarSettingsValue(d *StellarRenderData, value any) string {
	s := stellarText(value)
	return strings.NewReplacer("{hexo.name}", "Blog", "{hexo.version}", stellarText(d.Stellar.Data["blogVersion"]), "{hexo.homepage}", "https://hexo.io/", "{theme.name}", "Stellar", "{theme.version}", "2.0.0", "{theme.tree}", "https://github.com/xaoxuu/hexo-theme-stellar/tree/2.0.0").Replace(s)
}
