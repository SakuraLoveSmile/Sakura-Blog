// Package viewmodel exposes the public Stellar rendering models. The current
// concrete definitions remain in settings while the migration keeps all
// consumers on this module-facing package.
package viewmodel

import (
	"html/template"
	"time"

	"blog/internal/settings"
	"blog/internal/store"
)

type AdjacentPost = settings.AdjacentPost
type ArticleConfig = settings.ArticleConfig
type AdaptiveTextConfig = settings.AdaptiveTextConfig
type ArchiveItem = settings.ArchiveItem
type ArchiveYear = settings.ArchiveYear
type AuthorDef = settings.AuthorDef
type AuthorLink = settings.AuthorLink
type AuthorWidgetData = settings.AuthorWidgetData
type CanonicalConfig = settings.CanonicalConfig
type CatItem = settings.CatItem
type Collection = settings.Collection
type CollectionItem = settings.CollectionItem
type CommentsConfig = settings.CommentsConfig
type DefaultAssets = settings.DefaultAssets
type EditPageLink = settings.EditPageLink
type Footer = settings.Footer

type HeroAction = settings.HeroAction
type HeroCommand = settings.HeroCommand
type HeroConfig = settings.HeroConfig
type HeroAnimation = settings.HeroAnimation
type HeroPreview = settings.HeroPreview

type IconRegistry = settings.IconRegistry
type LatestComment = settings.LatestComment
type LocalSearchConfig = settings.LocalSearchConfig
type LinkGroup = settings.LinkGroup
type LinkItem = settings.LinkItem
type LinkListConfig = settings.LinkListConfig
type LinkListItem = settings.LinkListItem
type MenuItem = settings.MenuItem
type Menubar = settings.Menubar
type MomentItem = settings.MomentItem
type Page = settings.Page
type PageBars = settings.PageBars
type Paginator = settings.Paginator
type PostCard = settings.PostCard
type PosterCard = settings.PosterCard
type RecentPost = settings.RecentPost
type SEOConfig = settings.SEOConfig
type SearchConfig = settings.SearchConfig
type Site = settings.Site
type SiteStats = settings.SiteStats
type TagCloudOptions = settings.TagCloudOptions
type TagCount = settings.TagCount
type TagItem = settings.TagItem
type TopicDetail = settings.TopicDetail
type TopicItem = settings.TopicItem
type TopicSeries = settings.TopicSeries
type TopicSeriesPost = settings.TopicSeriesPost
type WidgetFrame = settings.WidgetFrame
type ViewData = settings.ViewData
type WidgetDef = settings.WidgetDef
type WidgetInstance = settings.WidgetInstance
type WidgetsConfig = settings.WidgetsConfig
type WikiDocNav = settings.WikiDocNav
type WikiDocView = settings.WikiDocView
type WikiProject = settings.WikiProject
type WikiTreeNode = settings.WikiTreeNode
type WikiTreeWidgetData = settings.WikiTreeWidgetData

func NewPostCard(p *store.Post, categoryColors, categoryIcons map[string]string) *PostCard {
	return settings.NewPostCard(p, categoryColors, categoryIcons)
}

func ParseTags(raw string) []string { return settings.ParseTags(raw) }

func BuildTagCloud(counts map[string]int, opt TagCloudOptions) []*TagItem {
	return settings.BuildTagCloud(counts, opt)
}
func ExpiredDays(created, updated string, threshold int, now time.Time) int {
	return settings.ExpiredDays(created, updated, threshold, now)
}
func PageTypeOf(layout string) string        { return settings.PageTypeOf(layout) }
func ReadingStats(content string) (int, int) { return settings.ReadingStats(content) }
func ResolveEditPage(source string, mapping map[string]string) *EditPageLink {
	return settings.ResolveEditPage(source, mapping)
}
func TagCloudFontSize(count, min, max int) int          { return settings.TagCloudFontSize(count, min, max) }
func TagCloudOptionsFrom(def WidgetDef) TagCloudOptions { return settings.TagCloudOptionsFrom(def) }
func CategoryColorStyle(category string, colors map[string]string) template.CSS {
	return settings.CategoryColorStyle(category, colors)
}
func CategoryIconKey(category string, icons map[string]string) string {
	return settings.CategoryIconKey(category, icons)
}
func NormalizeSiteURL(raw string) string         { return settings.NormalizeSiteURL(raw) }
func ValidateSiteURL(raw string) error           { return settings.ValidateSiteURL(raw) }
func JoinSiteURL(base, requestPath string) string { return settings.JoinSiteURL(base, requestPath) }
