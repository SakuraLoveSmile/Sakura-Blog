package stellarmigration

import (
	"fmt"
	"strings"
)

type ConsumerCapability string

const (
	ConsumerPoster           ConsumerCapability = "poster"
	ConsumerCollectionOrder  ConsumerCapability = "collection_order"
	ConsumerAuthors          ConsumerCapability = "authors"
	ConsumerAuthorWidget     ConsumerCapability = "author_widget"
	ConsumerWikiExternalURL  ConsumerCapability = "wiki_external_url"
	ConsumerAvatarBackground ConsumerCapability = "avatar_background"
	ConsumerRightbarIntro    ConsumerCapability = "rightbar_intro"
	ConsumerWelcomeLinklist  ConsumerCapability = "welcome_linklist"
	ConsumerNavTabs          ConsumerCapability = "nav_tabs"
	ConsumerAILabel          ConsumerCapability = "ai_label"
	ConsumerCategoryIcons    ConsumerCapability = "category_icons"
	ConsumerTagIcons         ConsumerCapability = "tag_icons"
	ConsumerReadNext         ConsumerCapability = "read_next"
	ConsumerExpireDays       ConsumerCapability = "expire_days"
	ConsumerBannerRatio      ConsumerCapability = "banner_ratio"
	ConsumerEditThisPage     ConsumerCapability = "edit_this_page"
	ConsumerNotebookLicense  ConsumerCapability = "notebook_license"
	ConsumerNotebookShare    ConsumerCapability = "notebook_share"
	ConsumerPages            ConsumerCapability = "pages"
	ConsumerSEO              ConsumerCapability = "seo"
)

// A true value is the caller's explicit assertion that the corresponding
// consumer has been implemented and verified. No capability is enabled by default.
type ConsumerCapabilities map[ConsumerCapability]bool

var globalConsumerTargets = map[string]ConsumerCapability{
	"blog.content.avatar_background": ConsumerAvatarBackground,
	"blog.content.rightbar_intro":    ConsumerRightbarIntro,
	"blog.content.welcome_linklist":  ConsumerWelcomeLinklist,
	"blog.content.nav_tabs":          ConsumerNavTabs,
	"blog.content.ai_label":          ConsumerAILabel,
	"blog.content.category_icons":    ConsumerCategoryIcons,
	"blog.content.tag_icons":         ConsumerTagIcons,
	"blog.content.read_next":         ConsumerReadNext,
	"blog.content.expire_days":       ConsumerExpireDays,
	"blog.content.banner_ratio":      ConsumerBannerRatio,
	"blog.content.edit_this_page":    ConsumerEditThisPage,
	"blog.content.notebook.license":  ConsumerNotebookLicense,
	"blog.content.notebook.share":    ConsumerNotebookShare,
	"blog.pages":                     ConsumerPages,
	"blog.seo":                       ConsumerSEO,
}

func globalConsumerCode(capability ConsumerCapability) string {
	return "global_" + string(capability) + "_consumer"
}

var consumerCatalog = func() map[string]ConsumerCapability {
	catalog := map[string]ConsumerCapability{
		"poster_consumer":            ConsumerPoster,
		"topic_order_consumer":       ConsumerCollectionOrder,
		"multiple_authors":           ConsumerAuthors,
		"widget_author_removed":      ConsumerAuthorWidget,
		"wiki_external_url_consumer": ConsumerWikiExternalURL,
	}
	for _, capability := range globalConsumerTargets {
		catalog[globalConsumerCode(capability)] = capability
	}
	return catalog
}()

func validateCapabilities(capabilities ConsumerCapabilities) error {
	for capability := range capabilities {
		known := false
		for _, allowed := range consumerCatalog {
			if capability == allowed {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("unsupported migration consumer capability")
		}
	}
	return nil
}

func classifyDiagnostics(ds []Diagnostic, capabilities ConsumerCapabilities) []Diagnostic {
	for i := range ds {
		d := &ds[i]
		d.Kind = "policy"
		d.Severity = "error"
		if !d.Blocking {
			d.Severity = "warning"
		}
		if capability, known := consumerCatalog[d.Code]; known {
			d.Kind = "consumer"
			d.Consumer = capability
			if capabilities[capability] {
				d.Blocking = false
				d.Severity = "info"
			}
			continue
		}
		switch d.Code {
		case "legacy_notebook_schema", "note_missing_slug_status", "file_host_fields", "unversioned_metadata", "unsupported_content_version", "duplicate_collection_id", "topic_ambiguous", "topic_column_conflict", "topic_not_found", "category_relationship":
			d.Kind = "structure"
		default:
			if strings.HasPrefix(d.Code, "invalid_") || strings.HasPrefix(d.Code, "unknown_") || strings.HasPrefix(d.Code, "collection_") || d.Code == "removed_field" || d.Code == "unmapped_setting" || d.Code == "strict_validation" {
				d.Kind = "schema"
			}
		}
	}
	return ds
}
