package settings

import "testing"

func TestBuildTagTreeHierarchyAndCounts(t *testing.T) {
	data := BuildTagTree([]TagCount{
		{Name: "go", Count: 2},
		{Name: "go/web", Count: 3},
		{Name: "rust", Count: 1},
	}, "notes", "go/web", false, true, true)
	if data.Root == nil || len(data.Children) != 2 {
		t.Fatalf("root children = %+v", data.Children)
	}
	goNode := data.Children[0]
	if goNode.ID != "go" || goNode.Count != 5 || goNode.Active || !goNode.Expanded {
		t.Fatalf("parent node = %+v", goNode)
	}
	if len(goNode.Children) != 1 || goNode.Children[0].ID != "go/web" || !goNode.Children[0].Active {
		t.Fatalf("child node = %+v", goNode.Children)
	}
	if goNode.Path != "/notebooks/notes?tag=go" || goNode.Children[0].Path != "/notebooks/notes?tag=go%2Fweb" {
		t.Fatalf("tag paths = %q, %q", goNode.Path, goNode.Children[0].Path)
	}
	if !data.HasTag("go") || !data.HasTag("go/web") || data.HasTag("go/mobile") {
		t.Fatal("HasTag should only report tags represented in the tree")
	}
	if !TagMatches("go", "go") || !TagMatches("go/web", "go") || TagMatches("golang", "go") {
		t.Fatal("TagMatches hierarchy semantics are incorrect")
	}

	data.ApplyTagcons(map[string]string{"go": "default:documents", "web": "default:link"})
	if data.Children[0].Tagcon != "default:documents" || data.Children[0].Children[0].Tagcon != "default:link" {
		t.Fatalf("tagcons = %+v", data.Children)
	}
}
