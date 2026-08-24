package post

import "testing"

func TestShortcodeCatalogCoversRegisteredParsers(t *testing.T) {
	if len(shortcodeRegistry) < 50 {
		t.Fatalf("shortcode registry unexpectedly small: %d", len(shortcodeRegistry))
	}

	seen := make(map[string]bool, len(shortcodeRegistry))
	for _, registration := range shortcodeRegistry {
		if registration.entry.Name == "" {
			t.Error("catalog entry has empty name")
		}
		if seen[registration.entry.Name] {
			t.Errorf("duplicate catalog entry: %s", registration.entry.Name)
		}
		seen[registration.entry.Name] = true
		if registration.parser == nil {
			t.Errorf("catalog entry %s has no parser regexp", registration.entry.Name)
		}
		if registration.entry.Params == nil {
			t.Errorf("catalog entry %s has nil params; API should emit []", registration.entry.Name)
		}
		for _, field := range registration.entry.Params {
			if field.Key == "" || field.Type == "" || field.Role == "" {
				t.Errorf("catalog entry %s has incomplete field: %+v", registration.entry.Name, field)
			}
		}
	}

	for _, name := range []string{"note", "table", "tip", "links", "navbar", "frame", "ghrepo"} {
		if !seen[name] {
			t.Errorf("required parser %q missing from catalog", name)
		}
	}
	items := ShortcodeCatalog()
	for _, item := range items {
		if item.Name == "note" {
			if item.Title == "" || item.Description == "" || item.Signature == "" || item.Example == "" || len(item.ParamDocs) == 0 {
				t.Fatalf("note display metadata incomplete: %+v", item)
			}
			if len(item.Params) == 0 || item.Params[0].Label != "配色" {
				t.Fatalf("note form label missing from catalog: %+v", item.Params)
			}
			return
		}
	}
	t.Fatal("catalog does not contain note display metadata")
}

func TestShortcodeCatalogReturnsDefensiveCopy(t *testing.T) {
	first := ShortcodeCatalog()
	if len(first) == 0 || len(first[0].Params) == 0 {
		t.Fatal("catalog should contain fields")
	}
	first[0].Name = "changed"
	first[0].Params[0].Key = "changed"
	for _, item := range first {
		for _, field := range item.Params {
			if len(field.Options) > 0 {
				field.Options[0] = "changed"
			}
		}
	}

	second := ShortcodeCatalog()
	if second[0].Name == "changed" || second[0].Params[0].Key == "changed" {
		t.Fatal("ShortcodeCatalog must return a defensive copy")
	}
	for _, field := range second[0].Params {
		if len(field.Options) > 0 && field.Options[0] == "changed" {
			t.Fatal("ShortcodeCatalog must copy field options")
		}
	}
}
