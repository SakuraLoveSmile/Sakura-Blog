package stellarmigration

import (
	"fmt"
	"strings"
)

type ConsumerCapability string

const (
	ConsumerPoster          ConsumerCapability = "poster"
	ConsumerCollectionOrder ConsumerCapability = "collection_order"
	ConsumerAuthors         ConsumerCapability = "authors"
	ConsumerAuthorWidget    ConsumerCapability = "author_widget"
	ConsumerWikiExternalURL ConsumerCapability = "wiki_external_url"
)

// A true value is the caller's explicit assertion that the corresponding
// consumer has been implemented and verified. No capability is enabled by default.
type ConsumerCapabilities map[ConsumerCapability]bool

var consumerCatalog = map[string]ConsumerCapability{
	"poster_consumer":            ConsumerPoster,
	"topic_order_consumer":       ConsumerCollectionOrder,
	"multiple_authors":           ConsumerAuthors,
	"widget_author_removed":      ConsumerAuthorWidget,
	"wiki_external_url_consumer": ConsumerWikiExternalURL,
}

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
