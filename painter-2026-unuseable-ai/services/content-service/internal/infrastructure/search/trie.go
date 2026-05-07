package search

import "unicode"

type trieNode struct {
	child map[rune]*trieNode
	end   bool
}

func newTrieNode() *trieNode {
	return &trieNode{child: make(map[rune]*trieNode)}
}

// Trie 用于离线词典最长正向匹配（CM dict）。
type Trie struct {
	root *trieNode
}

func NewTrie() *Trie {
	return &Trie{root: newTrieNode()}
}

func (t *Trie) Insert(word string) {
	if word == "" {
		return
	}
	n := t.root
	for _, r := range word {
		next, ok := n.child[r]
		if !ok {
			next = newTrieNode()
			n.child[r] = next
		}
		n = next
	}
	n.end = true
}

// MatchLongest 从 text[start] 开始匹配最长的词典词（适用于汉字片段）。
func (t *Trie) MatchLongest(text []rune, start int) (word string, length int) {
	if start >= len(text) || !unicode.Is(unicode.Han, text[start]) {
		return "", 0
	}
	n := t.root
	bestEnd := -1
	cur := n
	for i := start; i < len(text); i++ {
		r := text[i]
		if !unicode.Is(unicode.Han, r) {
			break
		}
		next, ok := cur.child[r]
		if !ok {
			break
		}
		cur = next
		if cur.end {
			bestEnd = i
		}
	}
	if bestEnd >= start {
		return string(text[start : bestEnd+1]), bestEnd - start + 1
	}
	// 未命中词典：退回单字，避免阻断后续匹配
	return string(text[start]), 1
}
