package web

import "blog/internal/settings"

// Resource-facing aliases keep template tests and module callers concise while
// the durable configuration types remain owned by internal/settings.
type IconRegistry = settings.IconRegistry
type Site = settings.Site
type ViewData = settings.ViewData
type Page = settings.Page
type Paginator = settings.Paginator
type Footer = settings.Footer
type SocialItem = settings.SocialItem
type Collection = settings.Collection
type CollectionItem = settings.CollectionItem
type WidgetFrame = settings.WidgetFrame
type Author = settings.AuthorDef
type SEOConfig = settings.SEOConfig
type CanonicalConfig = settings.CanonicalConfig
type StructuredDataConfig = settings.StructuredDataConfig
type OpenGraphConfig = settings.OpenGraphConfig
type PluginsConfig = settings.PluginsConfig
type AdaptiveTextConfig = settings.AdaptiveTextConfig
type ArticleConfig = settings.ArticleConfig
type AILabelConfig = settings.AILabelConfig
type AILabelDef = settings.AILabelDef
type SwiperConfig = settings.SwiperConfig
type CopycodeConfig = settings.CopycodeConfig
type KaTeXConfig = settings.KaTeXConfig
type MermaidConfig = settings.MermaidConfig
type MathJaxConfig = settings.MathJaxConfig

func LoadIconsEmbed() (*IconRegistry, error) { return settings.LoadIconsEmbed() }
