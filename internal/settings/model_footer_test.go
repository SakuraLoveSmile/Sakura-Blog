package settings

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestFooterSocialYAMLListAndMappingPreserveOrder(t *testing.T) {
	for name, input := range map[string]string{
		"list": `footer:
  social:
    - title: GitHub
      url: https://github.com/example
      icon: default:github
    - type: spacer
    - title: 本站
      url: /about/
      icon_key: default:documents
    - type: dropdown
      title: 更多
      icon: default:more
      items:
        - title: 内链
          url: /links/
        - title: 外链
          url: https://example.com
`,
		"mapping": `footer:
  social:
    github:
      title: GitHub
      url: https://github.com/example
      icon: default:github
    spacer:
    about:
      title: 本站
      url: /about/
      icon_key: default:documents
    more:
      type: dropdown
      title: 更多
      icon: default:more
      items:
        - title: 内链
          url: /links/
`,
	} {
		t.Run(name, func(t *testing.T) {
			var cfg struct {
				Footer Footer `yaml:"footer"`
			}
			if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
				t.Fatalf("decode social: %v", err)
			}
			if len(cfg.Footer.Social) != 4 {
				t.Fatalf("social length = %d, want 4: %+v", len(cfg.Footer.Social), cfg.Footer.Social)
			}
			if cfg.Footer.Social[0].Title != "GitHub" || !cfg.Footer.Social[1].IsSpacer() || cfg.Footer.Social[2].Title != "本站" {
				t.Fatalf("social order = %+v", cfg.Footer.Social)
			}
			more := cfg.Footer.Social[3]
			if !more.IsDropdown() || len(more.Items) == 0 || more.Items[0].URL == "" {
				t.Fatalf("dropdown = %+v", more)
			}
			if !cfg.Footer.Social[0].External() || cfg.Footer.Social[0].Target() != "_blank" || cfg.Footer.Social[0].Rel() != "external nofollow noopener noreferrer" {
				t.Fatalf("external link attrs = %+v", cfg.Footer.Social[0])
			}
			if cfg.Footer.Social[2].External() || cfg.Footer.Social[2].Rel() != "noopener noreferrer" {
				t.Fatalf("internal link attrs = %+v", cfg.Footer.Social[2])
			}
		})
	}
}

func TestFooterSocialJSONMappingAndRawIcon(t *testing.T) {
	var footer Footer
	if err := footer.UnmarshalJSON([]byte(`{"social":{"one":{"title":"One","url":"/one","icon":"<svg></svg>"},"spacer":{},"drop":{"type":"dropdown","items":[{"title":"Two","url":"/two"}]}}}`)); err != nil {
		t.Fatalf("decode JSON social: %v", err)
	}
	if len(footer.Social) != 3 || !footer.Social[1].IsSpacer() || footer.Social[0].Icon != "<svg></svg>" {
		t.Fatalf("JSON social = %+v", footer.Social)
	}
	if !footer.Social[2].IsDropdown() || len(footer.Social[2].Items) != 1 {
		t.Fatalf("JSON nested items = %+v", footer.Social[2])
	}
}
