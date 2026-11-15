package app

import (
	"regexp"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

var readingTimeURL = regexp.MustCompile(`(?i)\b(?:https?://|ftp://|www\.)[^\s<>"'，。！？；：、（）【】《》“”‘’]+`)

// Estimate the body that is actually public, including expanded internal cards.
// At 400 CJK characters or 200 other words per minute, mixed-language effort is
// added before rounding. This is a reading estimate, not a source-code byte count.
func articleReadingMinutes(markup string) int {
	cjk, words := articleReadingUnits(articleReadingText(markup))
	return max(1, (cjk+2*words+399)/400)
}

func articleReadingText(markup string) string {
	doc, err := xhtml.Parse(strings.NewReader(markup))
	if err != nil {
		return ""
	}
	var text strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			text.WriteString(n.Data)
			return
		}
		boundary := false
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "script", "style", "template", "noscript", "svg":
				return
			case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt", "figcaption", "figure", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "td", "th", "tr", "ul":
				boundary = true
			}
			for _, attr := range n.Attr {
				if attr.Key == "hidden" || attr.Key == "aria-hidden" && attr.Val == "true" {
					return
				}
				if attr.Key != "class" {
					continue
				}
				for _, class := range strings.Fields(attr.Val) {
					// Inline references display their anchor text in the article;
					// their duplicate snapshot/description is a hidden hover preview.
					if class == "content-link__preview" {
						return
					}
					// Chroma line numbers and card URL captions are presentation,
					// rather than article words. Syntax-highlight spans stay inline.
					if class == "ln" || class == "lnt" || class == "lntd" || class == "content-card__url" {
						return
					}
					// These spans are separate displayed card fields, even where
					// their serialized HTML has no whitespace between elements.
					if strings.HasPrefix(class, "content-card__") {
						boundary = true
					}
				}
			}
		}
		if boundary {
			text.WriteByte(' ')
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if boundary {
			text.WriteByte(' ')
		}
	}
	walk(doc)
	return text.String()
}

func articleReadingUnits(text string) (cjk, words int) {
	// Link destinations/metadata never enter this text. Bare visible URLs are
	// also excluded so long paths and query strings cannot inflate the estimate.
	text = readingTimeURL.ReplaceAllString(text, " ")
	inWord := false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) && unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
			cjk++
			inWord = false
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if !inWord {
				words++
			}
			inWord = true
		case inWord && (unicode.IsMark(r) || r == '\'' || r == '’' || r == '-'):
			// Keep accented words and internal apostrophes/hyphens together.
		default:
			inWord = false
		}
	}
	return cjk, words
}
