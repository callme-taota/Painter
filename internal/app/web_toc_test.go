package app

import (
	"reflect"
	"testing"
)

func TestArticleTOCTreePreservesOrderAndRealHierarchy(t *testing.T) {
	entries := []TOCEntry{{"a", "Starts with h3", 3}, {"b", "Skipped h5", 5}, {"c", "Sibling h5", 5}, {"d", "Back to h4", 4}, {"e", "New h2 root", 2}, {"f", "Child h6", 6}, {"g", "Another h2", 2}}
	tree := articleTOCTree(entries)
	if len(tree) != 3 || len(tree[0].Children) != 3 || len(tree[1].Children) != 1 || len(tree[2].Children) != 0 {
		t.Fatal("incorrect parent relationships", tree)
	}
	var ids []string
	var levels, depths []int
	var walk func([]*ArticleTOCNode)
	walk = func(nodes []*ArticleTOCNode) {
		for _, node := range nodes {
			ids = append(ids, node.ID)
			levels = append(levels, node.Level)
			depths = append(depths, node.Depth)
			walk(node.Children)
		}
	}
	walk(tree)
	if !reflect.DeepEqual(ids, []string{"a", "b", "c", "d", "e", "f", "g"}) || !reflect.DeepEqual(levels, []int{3, 5, 5, 4, 2, 6, 2}) || !reflect.DeepEqual(depths, []int{0, 1, 1, 1, 0, 1, 0}) {
		t.Fatal(ids, levels, depths)
	}
	if tree[0].Text != entries[0].Text || entries[0].Level != 3 {
		t.Fatal("source text or stored metadata changed")
	}
}

func TestArticleTOCTreeFlatEmptyAndDeepHeadings(t *testing.T) {
	if len(articleTOCTree(nil)) != 0 {
		t.Fatal("empty article must not gain fake headings")
	}
	flat := articleTOCTree([]TOCEntry{{"a", "One", 2}, {"b", "Two", 2}})
	if len(flat) != 2 || flat[0].Depth != 0 || flat[1].Depth != 0 {
		t.Fatal("h2-only articles should have root entries")
	}
	deep := articleTOCTree([]TOCEntry{{"a", "1", 1}, {"b", "2", 2}, {"c", "3", 3}, {"d", "4", 4}, {"e", "5", 5}, {"f", "6", 6}})
	for depth := 0; depth < 6; depth++ {
		if len(deep) != 1 || deep[0].Depth != depth || deep[0].Level != depth+1 {
			t.Fatal("all six heading levels must remain navigable")
		}
		deep = deep[0].Children
	}
}
