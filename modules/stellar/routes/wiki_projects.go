package routes

import (
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	"strings"
)

// wikiProjectTemplate converts the database model into the rendering model.
// Keeping this conversion at the module boundary prevents the store package
// from importing template/settings packages.
func wikiProjectTemplate(p *store.WikiProject) *tmpl.WikiProject {
	if p == nil {
		return nil
	}
	out := &tmpl.WikiProject{
		ID: p.ID, Name: p.Name, Title: p.Title, Headline: p.Headline,
		Subtitle: p.Subtitle, Description: p.Description, Icon: p.Icon,
		Cover: p.Cover, Available: p.Available, URL: p.URL, Repo: p.Repo,
		Branch: p.Branch, Sort: p.Sort, Pin: p.Pin, HasDocs: p.HasDocs,
		Tags: append([]string(nil), p.Tags...),
	}
	visible := p.Visible
	out.Visible = &visible
	if p.Hero != nil {
		h := &tmpl.HeroConfig{
			Background: p.Hero.Background, Title: p.Hero.Title,
			Description: p.Hero.Description, Start: p.Hero.Start,
		}
		if p.Hero.Animation != nil {
			h.Animation = &tmpl.HeroAnimation{Type: p.Hero.Animation.Type, Params: p.Hero.Animation.Params}
		}
		if p.Hero.Preview != nil {
			h.Preview = &tmpl.HeroPreview{Type: p.Hero.Preview.Type, Image: p.Hero.Preview.Image}
			for _, command := range p.Hero.Preview.Commands {
				h.Preview.Commands = append(h.Preview.Commands, tmpl.HeroCommand{Codes: append([]string(nil), command.Codes...)})
			}
		}
		for _, action := range p.Hero.Actions {
			h.Actions = append(h.Actions, tmpl.HeroAction{Title: action.Title, URL: action.URL, Icon: action.Icon})
		}
		out.Hero = normalizeLegacyWikiHero(h)
	}
	return out
}

func (s *webServer) wikiProjectsFromStore() ([]*tmpl.WikiProject, error) {
	projects, err := s.st.ListWikiProjectsWithCounts()
	if err != nil {
		return nil, err
	}
	out := make([]*tmpl.WikiProject, 0, len(projects))
	for _, p := range projects {
		if !p.Visible {
			continue
		}
		out = append(out, wikiProjectTemplate(p))
	}
	return out, nil
}

// wikiProjectsForRuntime includes a compatibility fallback for focused tests
// and old embedders that construct webServer directly without Bootstrap. A
// normal app has the migration marker and therefore never reads site.Wiki.
func (s *webServer) wikiProjectsForRuntime() []*tmpl.WikiProject {
	projects, err := s.wikiProjectsFromStore()
	if err == nil {
		if marker, markerErr := s.st.GetSetting("wiki_projects_migrated"); markerErr == nil && marker == "1" {
			return projects
		}
		if len(projects) > 0 {
			return projects
		}
	}
	if s.site == nil || s.site.Load() == nil {
		return nil
	}
	legacy := func() []*tmpl.WikiProject {
		out := make([]*tmpl.WikiProject, 0, len(s.site.Load().Wiki))
		for i := range s.site.Load().Wiki {
			p := s.site.Load().Wiki[i]
			p.Hero = normalizeLegacyWikiHero(p.Hero)
			out = append(out, &p)
		}
		return out
	}()
	ids := make([]string, 0, len(legacy))
	for _, p := range legacy {
		ids = append(ids, p.ID)
	}
	if counts, err := s.st.CountPublishedWikidocsByProjects(ids); err == nil {
		for _, p := range legacy {
			p.HasDocs = counts[p.ID] > 0
		}
	}
	filtered := legacy[:0]
	for _, p := range legacy {
		if p.Visible != nil && !*p.Visible {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

// normalizeLegacyWikiHero keeps the old background: galaxy shorthand working
// for direct embedders that bypass the Store migration path.
func normalizeLegacyWikiHero(hero *tmpl.HeroConfig) *tmpl.HeroConfig {
	if hero == nil {
		return nil
	}
	out := *hero
	if strings.EqualFold(strings.TrimSpace(out.Background), "galaxy") {
		out.Background = ""
		if out.Animation == nil || strings.TrimSpace(out.Animation.Type) == "" {
			out.Animation = &tmpl.HeroAnimation{Type: "galaxy"}
		}
	}
	return &out
}

func wikiProjectHasTag(p *tmpl.WikiProject, tag string) bool {
	for _, candidate := range p.Tags {
		if strings.TrimSpace(candidate) == tag {
			return true
		}
	}
	return false
}
