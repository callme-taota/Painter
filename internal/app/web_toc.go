package app

// ArticleTOCNode preserves the actual heading level while nesting under the
// closest preceding shallower heading. Skipped levels do not create empty
// headings or unnecessarily indent an article that starts at h2/h3.
type ArticleTOCNode struct {
	ID, Text     string
	Level, Depth int
	Children     []*ArticleTOCNode
}

func articleTOCTree(entries []TOCEntry) []*ArticleTOCNode {
	roots := []*ArticleTOCNode{}
	stack := []*ArticleTOCNode{}
	for _, entry := range entries {
		level := entry.Level
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		for len(stack) > 0 && stack[len(stack)-1].Level >= level {
			stack = stack[:len(stack)-1]
		}
		node := &ArticleTOCNode{ID: entry.ID, Text: entry.Text, Level: level, Depth: len(stack)}
		if len(stack) == 0 {
			roots = append(roots, node)
		} else {
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, node)
		}
		stack = append(stack, node)
	}
	return roots
}
