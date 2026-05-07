package search

import (
	_ "embed"
	"strings"
	"unicode"
)

//go:embed dict_zh.txt
var embeddedDict []byte

// Tokenizer 离线分词：嵌入词典 + Trie 最长正向匹配；拉丁数字按连续片段切分。
type Tokenizer struct {
	trie      *Trie
	stopWords map[string]struct{}
}

func NewTokenizer() (*Tokenizer, error) {
	tr := NewTrie()
	lines := strings.Split(string(embeddedDict), "\n")
	for _, line := range lines {
		w := strings.TrimSpace(line)
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		tr.Insert(w)
		if lw := strings.ToLower(w); lw != w {
			tr.Insert(lw)
		}
	}
	stop := map[string]struct{}{
		"的": {}, "了": {}, "和": {}, "与": {}, "或": {}, "在": {}, "是": {}, "有": {}, "为": {}, "及": {}, "等": {}, "啊": {}, "吧": {}, "吗": {}, "呀": {}, "呢": {},
	}
	return &Tokenizer{trie: tr, stopWords: stop}, nil
}

func (z *Tokenizer) Segment(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	rs := []rune(strings.ToLower(raw))
	var out []string
	i := 0
	for i < len(rs) {
		r := rs[i]
		switch {
		case unicode.Is(unicode.Han, r):
			w, l := z.trie.MatchLongest(rs, i)
			w = normalizeToken(w)
			if z.keep(w) {
				out = append(out, w)
			}
			i += max(l, 1)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j])) {
				j++
			}
			w := normalizeToken(string(rs[i:j]))
			if z.keep(w) {
				out = append(out, w)
			}
			i = j
		default:
			i++
		}
	}
	return dedupeOrdered(out)
}

func (z *Tokenizer) keep(tok string) bool {
	if tok == "" {
		return false
	}
	if _, ok := z.stopWords[tok]; ok {
		return false
	}
	rs := []rune(tok)
	if len(rs) == 1 && unicode.IsPunct(rs[0]) {
		return false
	}
	return true
}

func normalizeToken(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func dedupeOrdered(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, x := range in {
		if x == "" {
			continue
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
