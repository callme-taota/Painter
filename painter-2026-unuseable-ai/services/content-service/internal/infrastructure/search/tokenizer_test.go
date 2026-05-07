package search

import "testing"

func TestTokenizerSegmentHanEnglish(t *testing.T) {
	z, err := NewTokenizer()
	if err != nil {
		t.Fatal(err)
	}
	parts := z.Segment("Painter 博客 支持离线分词与搜索")
	m := map[string]struct{}{}
	for _, p := range parts {
		m[p] = struct{}{}
	}
	for _, need := range []string{"painter", "博客", "分词"} {
		if _, ok := m[need]; !ok {
			t.Fatalf("missing token %q in %#v", need, parts)
		}
	}
}
