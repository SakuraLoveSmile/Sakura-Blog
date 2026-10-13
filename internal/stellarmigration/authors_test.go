package stellarmigration

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"blog/internal/store"
)

func authorSnapshot(t *testing.T, ddl string) Snapshot {
	t.Helper()
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);
INSERT INTO settings VALUES('authors','{"alice":{"name":"Public Alice"},"writer":{"name":"Public Writer"},"42":{"name":"Public numeric ID"},"blank":{"name":""}}');
CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,author TEXT);
INSERT INTO posts VALUES(1,'fixture','Fixture','body','');
CREATE TABLE users(id INTEGER PRIMARY KEY,username TEXT,name TEXT,password_hash TEXT);
INSERT INTO users VALUES(42,'alice','Public Writer','PRIVATE_PASSWORD_NEVER_PUBLIC');`+ddl)
	return snapshot(t, path, nil)
}

func TestOwnershipRowsProducePublicAuthorIDsWithoutPublishingPermissions(t *testing.T) {
	s := authorSnapshot(t, `CREATE TABLE post_authors(post_id INTEGER,author_id INTEGER,sort_order INTEGER,private_note TEXT);
INSERT INTO post_authors VALUES(1,42,1,'PRIVATE_LINK_NEVER_PUBLIC');
INSERT INTO users VALUES(7,'writer','Alice','PRIVATE_SECOND_PASSWORD');
INSERT INTO post_authors VALUES(1,7,0,'PRIVATE_LINK_NEVER_PUBLIC');`)
	c := candidate(t, s)
	p := c.Records[0]
	var blog struct {
		Content struct {
			Authors []string `json:"authors"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(p.BlogJSON), &blog); err != nil || !reflect.DeepEqual(blog.Content.Authors, []string{"writer", "alice"}) {
		t.Fatal("public authors are not ordered catalog ID strings")
	}
	if strings.Contains(p.BlogJSON, "PRIVATE_") || strings.Contains(p.BlogJSON, "author_id") || strings.Contains(p.BlogJSON, "sort_order") {
		t.Fatal("ownership row leaked into public extension")
	}
	if encode(c.Source) != encode(s) || !hasCode(p.Diagnostics, "multiple_authors") {
		t.Fatal("source or consumer gate lost")
	}
	closed, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerAuthors: true})
	if err != nil || ValidateCandidate(closed, s) != nil {
		t.Fatal("determined catalog candidate fails strict migration validation")
	}
	// 正式 Store 消费同一候选，而不是测试中另造一份扩展数组。
	st, err := store.New(filepath.Join(t.TempDir(), "strict.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err = st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting("stellar_v2", c.Theme.DocumentJSON); err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting("blog_extensions", c.Theme.BlogJSON); err != nil {
		t.Fatal(err)
	}
	post := &store.Post{StellarVersion: 2, StellarConfig: p.ConfigJSON, BlogExtensions: p.BlogJSON, Slug: "fixture"}
	if err = st.ValidateStellarPostPublicAuthors(post); err != nil || !reflect.DeepEqual(post.Authors, []string{"writer", "alice"}) || len(post.AuthorIDs) != 0 {
		t.Fatal("formal Store cannot consume candidate or candidate assigned ownership")
	}
}

func TestLegacyTextAuthorsUseExactExistingCatalogIDs(t *testing.T) {
	s := authorSnapshot(t, `CREATE TABLE post_authors(post_id INTEGER,author_id TEXT,sort_order INTEGER);
INSERT INTO post_authors VALUES(1,'writer',0),(1,'writer',1),(1,'42',2);
CREATE TABLE post_authors_legacy(post_id INTEGER,author_ref TEXT,sort_order INTEGER,reason TEXT);
INSERT INTO post_authors_legacy VALUES(1,'alice',3,'unresolved_user');`)
	c := candidate(t, s)
	v, _ := decode(c.Records[0].BlogJSON)
	ids, _ := get(object(v), "content.authors")
	if encode(ids) != `["writer","42","alice"]` {
		t.Fatal("TEXT public IDs were guessed as user IDs or duplicates reordered")
	}
}

func TestUnmappedAuthorsRemainBlockingEvenWithConsumerCapability(t *testing.T) {
	for name, ddl := range map[string]string{
		"missing user":        `CREATE TABLE post_authors(post_id INTEGER,author_id INTEGER,sort_order INTEGER);INSERT INTO post_authors VALUES(1,99,0);`,
		"display name":        `CREATE TABLE post_authors(post_id INTEGER,author_id TEXT,sort_order INTEGER);INSERT INTO post_authors VALUES(1,'Public Writer',0);`,
		"blank catalog":       `CREATE TABLE post_authors(post_id INTEGER,author_id TEXT,sort_order INTEGER);INSERT INTO post_authors VALUES(1,'blank',0);`,
		"partial unresolved":  `CREATE TABLE post_authors(post_id INTEGER,author_id TEXT,sort_order INTEGER);INSERT INTO post_authors VALUES(1,'alice',0),(1,'PRIVATE_UNMAPPED',1);`,
		"legacy unresolved":   `CREATE TABLE post_authors_legacy(post_id INTEGER,author_ref TEXT,sort_order INTEGER,reason TEXT);INSERT INTO post_authors_legacy VALUES(1,'PRIVATE_UNMAPPED',0,'unresolved');`,
		"no catalog username": `UPDATE users SET username='PRIVATE_UNMAPPED';CREATE TABLE post_authors(post_id INTEGER,author_id INTEGER,sort_order INTEGER);INSERT INTO post_authors VALUES(1,42,0);`,
		"scalar display name": `UPDATE posts SET author='Public Writer';`,
	} {
		t.Run(name, func(t *testing.T) {
			s := authorSnapshot(t, ddl)
			c, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerAuthors: true})
			if err != nil {
				t.Fatal(err)
			}
			if !hasCode(c.Records[0].Diagnostics, "public_author_unmapped") || ValidateCandidate(c, s) == nil {
				t.Fatal("consumer capability erased identity mapping decision")
			}
			blog, _ := decode(c.Records[0].BlogJSON)
			if _, exists := get(object(blog), "content.authors"); exists {
				t.Fatal("partial unresolved authors silently truncated")
			}
			if encode(c.Source) != encode(s) || strings.Contains(encode(c.PublicReport()), "PRIVATE_UNMAPPED") {
				t.Fatal("private association changed or leaked")
			}
			for _, d := range c.Records[0].Diagnostics {
				if d.Code == "public_author_unmapped" && (!d.Blocking || d.Kind != "policy") {
					t.Fatal("identity policy gate misclassified")
				}
			}
		})
	}
}

func TestFriendsShowPostsNullPreservesInheritedEnabledState(t *testing.T) {
	for _, value := range []any{nil, false, true} {
		s := settingsSnapshot(t, map[string]any{"pages": map[string]any{"friends": map[string]any{"show_posts": value}}})
		c, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerPages: true})
		if err != nil || ValidateCandidate(c, s) != nil {
			t.Fatal("valid nullable pointer falsely blocked")
		}
		blog, _ := decode(c.Theme.BlogJSON)
		got, exists := get(object(blog), "pages.friends.show_posts")
		if !exists || got != value {
			t.Fatal("null/false/true changed or discarded")
		}
	}
}
