package stellarmigration

import (
	"strings"
	"testing"
)

func TestWikiExternalURLRemainsExtensionAndCanonicalRouteLocal(t *testing.T) {
	path := fixture(t, `CREATE TABLE wiki_projects(id TEXT PRIMARY KEY,name TEXT,url TEXT,sort INTEGER,pin INTEGER,visible INTEGER);INSERT INTO wiki_projects VALUES('docs','Docs','https://external.example/docs',1,0,1);`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	p := c.Records[0]
	raw, _ := decode(p.ConfigJSON)
	blog, _ := decode(p.BlogJSON)
	if route, _ := get(object(raw), "route.path"); route != "wiki/docs" {
		t.Fatal("external URL replaced canonical route")
	}
	if url, _ := get(object(blog), "content.wiki_external_url"); url != "https://external.example/docs" {
		t.Fatal("old external URL not preserved in independent extension")
	}
	if !hasCode(p.Diagnostics, "wiki_external_url_consumer") || c.PublicReport().Blocking != 1 {
		t.Fatal("unverified consumer silently closed")
	}
	closed, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerWikiExternalURL: true})
	if err != nil {
		t.Fatal(err)
	}
	d := closed.Records[0].Diagnostics[0]
	if d.Kind != "consumer" || d.Consumer != ConsumerWikiExternalURL || d.Severity != "info" || d.Blocking {
		t.Fatal("explicit consumer declaration not retained as information")
	}
	if closed.Records[0].ConfigJSON != p.ConfigJSON || closed.Records[0].BlogJSON != p.BlogJSON || encode(closed.Records[0].Mappings) != encode(p.Mappings) {
		t.Fatal("consumer declaration altered candidates or mappings")
	}
	if err := ValidateCandidate(closed, s); err != nil {
		t.Fatal(err)
	}
}

func TestCapabilitiesNeverClosePolicyOrStructureConflicts(t *testing.T) {
	path := fixture(t, `CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,poster TEXT,banner TEXT,cover TEXT);INSERT INTO posts VALUES(1,'a','Title','body','{"headline":"Poster"}','/banner.png','/cover.png');`)
	s := snapshot(t, path, nil)
	open := candidate(t, s)
	closed, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerPoster: true, ConsumerCollectionOrder: true, ConsumerAuthors: true, ConsumerAuthorWidget: true, ConsumerWikiExternalURL: true})
	if err != nil {
		t.Fatal(err)
	}
	if closed.PublicReport().Blocking != 1 || len(closed.Records[0].Diagnostics) != len(open.Records[0].Diagnostics) {
		t.Fatal("policy removed or diagnostics discarded")
	}
	for _, d := range closed.Records[0].Diagnostics {
		if d.Code == "banner_cover" && (!d.Blocking || d.Kind != "policy" || d.Severity != "error") {
			t.Fatal("banner source choice resolved by consumer capability")
		}
	}
	if ValidateCandidate(closed, s) == nil {
		t.Fatal("unresolved policy accepted")
	}
	if _, err := PreviewWithCapabilities(s, ConsumerCapabilities{"banner_cover": true}); err == nil {
		t.Fatal("arbitrary policy key accepted as capability")
	}
}

func TestShelfURLAlternativesUseSameExtensionKey(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);INSERT INTO settings VALUES('wiki','[{"id":"docs","url":"https://shelf.example/"}]');CREATE TABLE wiki_projects(id TEXT PRIMARY KEY,name TEXT,url TEXT,sort INTEGER,pin INTEGER,visible INTEGER);INSERT INTO wiki_projects VALUES('docs','Docs','https://project.example/',1,0,1);`)
	c := candidate(t, snapshot(t, path, nil))
	found := false
	for _, d := range c.Theme.Diagnostics {
		if d.Code != "wiki_shelf_definition_conflict" {
			continue
		}
		found = true
		if len(d.Alternatives) != 2 {
			t.Fatal("missing source alternatives")
		}
		for _, a := range d.Alternatives {
			v, _ := decode(a.ConfigJSON)
			if route, _ := get(object(v), "route.path"); strings.HasPrefix(stringValue(route), "http") {
				t.Fatal("external URL in alternative canonical route")
			}
			blog, _ := decode(a.BlogJSON)
			if _, exists := get(object(blog), "content.wiki_external_url"); !exists {
				t.Fatal("alternative uses missing or parallel extension key")
			}
		}
	}
	if !found {
		t.Fatal("different external sources silently merged")
	}
}
