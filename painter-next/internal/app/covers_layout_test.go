package app

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCoverLinesBalancesCJKHeadingWithoutAnOrphan(t *testing.T) {
	const title = "把博客变成一间愿意停留的书房"
	got := coverLines(title, 13.5, 4)
	want := []string{"把博客变成一间", "愿意停留的书房"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unbalanced editorial heading: %#v", got)
	}
	if strings.Join(got, "") != title {
		t.Fatal("line breaks changed the article title", got)
	}
}

func TestCoverLinesFitsAllTemplateWidthsAndPreservesMixedWords(t *testing.T) {
	for _, width := range []float64{13.8, 14.4, 15.4} {
		for _, title := range []string{
			"把博客变成一间愿意停留的书房",
			"用Go与PostgreSQL写一篇关于中文排版的文章",
			"Writing with Go 和中文一起排版",
			strings.Repeat("长标题", 66) + "结尾", // The API's 200-character maximum.
		} {
			lines := coverLines(title, width, 4)
			if len(lines) < 1 || len(lines) > 4 {
				t.Fatalf("width %v: %d lines", width, len(lines))
			}
			for _, line := range lines {
				units := 0.0
				for _, r := range line {
					units += coverRuneWidth(r)
				}
				if units > width+1e-9 {
					t.Fatalf("width %v overflows with %q (%v)", width, line, units)
				}
			}
			if len(lines) > 1 && utf8.RuneCountInString(lines[len(lines)-1]) == 1 {
				t.Fatalf("width %v left a one-character last line: %#v", width, lines)
			}
			joined := strings.Join(lines, "")
			if utf8.RuneCountInString(title) < 100 {
				if strings.ReplaceAll(joined, " ", "") != strings.ReplaceAll(title, " ", "") {
					t.Fatalf("title text changed: %#v", lines)
				}
				if strings.Contains(title, "PostgreSQL") && !strings.Contains(strings.Join(lines, "\n"), "PostgreSQL") {
					t.Fatalf("split a fitting English word: %#v", lines)
				}
			} else if !strings.HasSuffix(joined, "…") {
				t.Fatal("long title did not use a bounded ellipsis", lines)
			}
		}
	}
}

func TestAutoCoverStylesUseFullBleedGradients(t *testing.T) {
	images := map[string]bool{}
	for _, template := range []string{"editorial", "geometric", "minimal"} {
		for _, color := range []string{"ink", "moss", "clay", "plum"} {
			svg := renderCoverSVG("把博客变成一间愿意停留的书房", CoverConfig{Mode: "auto", Template: template, Color: color})
			if images[svg] {
				t.Fatal("gradient options render the same image", template, color)
			}
			images[svg] = true
			decoder := xml.NewDecoder(strings.NewReader(svg))
			gradients, canvases := 0, 0
			for {
				token, e := decoder.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					t.Fatal("invalid gradient SVG", template, color, e)
				}
				start, ok := token.(xml.StartElement)
				if !ok {
					continue
				}
				switch start.Name.Local {
				case "linearGradient", "radialGradient":
					gradients++
				case "rect":
					attrs := map[string]string{}
					for _, attr := range start.Attr {
						attrs[attr.Name.Local] = attr.Value
					}
					if attrs["width"] != "1200" || attrs["height"] != "675" || attrs["stroke"] != "" || (attrs["x"] != "" && attrs["x"] != "0") || (attrs["y"] != "" && attrs["y"] != "0") {
						t.Fatal("cover contains a decorative shape instead of a full canvas", template, color, attrs)
					}
					canvases++
				case "circle", "ellipse", "path", "line", "polygon", "polyline", "image", "foreignObject", "script":
					t.Fatal("cover contains a decoration or external content", template, color, start.Name.Local)
				}
			}
			if gradients < 2 || canvases < 1 || !strings.Contains(svg, `data-renderer="`+coverRendererVersion+`"`) {
				t.Fatal("cover is missing its gradient or cache version", template, color)
			}
		}
	}
}

func TestAutoCoverHTTPRevalidationAndHEAD(t *testing.T) {
	a := &App{}
	c := CoverConfig{Mode: "auto", Template: "editorial", Color: "ink"}
	read := func(method, target, match string, public bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, target, nil)
		r.Header.Set("If-None-Match", match)
		w := httptest.NewRecorder()
		if e := a.writeCover(w, r, "渐变里的中文与 Go", c, public); e != nil {
			t.Fatal(e)
		}
		return w
	}
	get := read(http.MethodGet, "/cover", "", true)
	if get.Code != http.StatusOK || get.Header().Get("ETag") == "" || get.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
		t.Fatal("public cover cannot safely revalidate", get.Code, get.Header())
	}
	if get.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
		t.Fatal("cover content length does not describe its bytes", get.Header())
	}
	head := read(http.MethodHead, "/cover?download=1", "", true)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("ETag") != get.Header().Get("ETag") || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") || head.Header().Get("Content-Disposition") == "" {
		t.Fatal("HEAD differs from the downloadable GET metadata or contains a body", head.Code, head.Header(), head.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		cached := read(method, "/cover", get.Header().Get("ETag"), true)
		if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
			t.Fatal("matching cover ETag did not revalidate", method, cached.Code, cached.Body.String())
		}
	}
	c.Color = "moss"
	updated := read(http.MethodGet, "/cover", get.Header().Get("ETag"), true)
	if updated.Code != http.StatusOK || updated.Header().Get("ETag") == get.Header().Get("ETag") || updated.Body.String() == get.Body.String() {
		t.Fatal("a changed gradient reused an old cached cover", updated.Code, updated.Header())
	}
	private := read(http.MethodGet, "/cover", "", false)
	if private.Header().Get("Cache-Control") != "private, no-store" || private.Header().Get("ETag") != "" {
		t.Fatal("private cover became cacheable", private.Header())
	}
}

func TestBalancedCoverSVGKeepsFullEscapedAccessibleTitle(t *testing.T) {
	title := `中文封面 </text><script>alert("x")</script> & ` + strings.Repeat("标题", 70)
	for _, template := range []string{"editorial", "geometric", "minimal"} {
		svg := renderCoverSVG(title, CoverConfig{Mode: "auto", Template: template, Color: "ink"})
		if strings.Contains(svg, "<script>") || !strings.Contains(svg, `<title id="cover-title">`+xmlText(title)+`</title>`) {
			t.Fatal("layout changed or failed to escape the full title", template)
		}
		decoder := xml.NewDecoder(strings.NewReader(svg))
		textCount := 0
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal("invalid SVG", template, e)
			}
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "text" {
				textCount++
			}
		}
		if textCount > 4 {
			t.Fatal("template exceeded its title line limit", template, textCount)
		}
	}
}
