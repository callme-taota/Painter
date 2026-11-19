package app

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"painter-next/web"
)

// CoverConfig is revision data: changing a working cover never changes the
// image for an already published revision.
type CoverConfig struct {
	Mode     string `json:"mode"`
	Template string `json:"template"`
	Color    string `json:"color"`
	MediaID  string `json:"media_id,omitempty"`
}

func normalizeCover(c *CoverConfig) error {
	if c.Mode == "" {
		c.Mode = "auto"
	}
	if c.Template == "" {
		c.Template = "editorial"
	}
	if c.Color == "" {
		c.Color = "ink"
	}
	if c.Mode != "auto" && c.Mode != "upload" && c.Mode != "none" {
		return E(400, "VALIDATION_FAILED", "封面模式无效")
	}
	if c.Template != "editorial" && c.Template != "geometric" && c.Template != "minimal" {
		return E(400, "VALIDATION_FAILED", "封面版式无效")
	}
	if c.Color != "ink" && c.Color != "moss" && c.Color != "clay" && c.Color != "plum" {
		return E(400, "VALIDATION_FAILED", "封面配色无效")
	}
	if c.Mode == "upload" {
		if len(c.MediaID) != 48 || strings.Trim(c.MediaID, "0123456789abcdef") != "" {
			return E(400, "VALIDATION_FAILED", "请选择已上传的封面图片")
		}
	} else {
		c.MediaID = ""
	}
	return nil
}

func coverJSON(c CoverConfig) []byte { b, _ := json.Marshal(c); return b }

// Article media is the union of inline images and the uploaded cover. The
// public draft contract retains media_ids for inline images only.
func draftMediaIDs(d draftInput) []string {
	ids := append([]string{}, d.MediaIDs...)
	if d.Cover.Mode == "upload" {
		found := false
		for _, id := range ids {
			if id == d.Cover.MediaID {
				found = true
				break
			}
		}
		if !found {
			ids = append(ids, d.Cover.MediaID)
		}
	}
	// All media locks use a stable order, including a cover reused inline.
	sort.Strings(ids)
	return ids
}

func (a *App) registerCovers() {
	a.Handle("POST /api/v1/editor/cover-preview", a.previewCover)
	a.Handle("GET /api/v1/studio/posts/{id}/cover", a.privateCover)
	a.Handle("GET /api/v1/posts/{id}/cover", a.publicCover)
}

func (a *App) previewCover(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "author", "owner"); e != nil {
		return e
	}
	if e := a.checkCSRF(r); e != nil {
		return e
	}
	var in struct {
		Title string      `json:"title"`
		Cover CoverConfig `json:"cover"`
	}
	if e := Decode(r, &in); e != nil {
		return e
	}
	if utf8.RuneCountInString(in.Title) > 200 {
		return E(400, "VALIDATION_FAILED", "标题最多200字")
	}
	if e := normalizeCover(&in.Cover); e != nil {
		return e
	}
	return a.writeCover(w, r, in.Title, in.Cover, false)
}

func (a *App) privateCover(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return e
	}
	v, e := a.privatePost(r.Context(), a.Pool, p, r.PathValue("id"))
	if e != nil {
		return e
	}
	d, e := draftFromMap(v)
	if e != nil {
		return e
	}
	return a.writeCover(w, r, d.Title, d.Cover, false)
}

func (a *App) publicCover(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if contentID(id) != nil {
		return NotFound()
	}
	var title, revision string
	var raw []byte
	e := a.Pool.QueryRow(r.Context(), `SELECT p.title,p.revision_id,r.cover FROM public_posts p JOIN post_revisions r ON r.id=p.revision_id WHERE p.post_id=$1`, id).Scan(&title, &revision, &raw)
	if e == pgx.ErrNoRows {
		return NotFound()
	}
	if e != nil {
		return e
	}
	// An old revision URL must never expose a withdrawn or replaced cover.
	if requested := r.URL.Query().Get("revision"); requested != "" && requested != revision {
		return NotFound()
	}
	var c CoverConfig
	if e = json.Unmarshal(raw, &c); e != nil {
		return e
	}
	return a.writeCover(w, r, title, c, true)
}

func (a *App) writeCover(w http.ResponseWriter, r *http.Request, title string, c CoverConfig, public bool) error {
	if e := normalizeCover(&c); e != nil {
		return e
	}
	if c.Mode == "none" {
		return NotFound()
	}
	if c.Mode == "upload" {
		r.SetPathValue("id", c.MediaID)
		return a.mediaRead(w, r)
	}
	b := renderCoverSVG(title, c)
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, no-store")
	if public {
		// Revalidate against the publication pointer before every cached reuse.
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		etag := `"` + hashString(b) + `"`
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="article-cover.svg"`)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return nil
	}
	_, e := w.Write([]byte(b))
	return e
}

func xmlText(s string) string {
	return html.EscapeString(strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r <= 0xd7ff) || (r >= 0xe000 && r <= 0xfffd) || (r >= 0x10000 && r <= 0x10ffff) {
			return r
		}
		return -1
	}, s))
}

func coverLines(title string, width float64, max int) []string {
	if width <= 0 || max <= 0 {
		return []string{}
	}
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		title = "无题手稿"
	}
	runes := []rune(title)
	// Find the minimum line count and the prefix that fits this template. Reserve
	// room for an ellipsis before balancing, rather than adding it past the edge.
	count, used, end := 1, 0.0, len(runes)
	for i, r := range runes {
		units := coverRuneWidth(r)
		if used+units > width && used > 0 {
			if count == max {
				end = i
				break
			}
			count++
			used = 0
		}
		used += units
	}
	if end < len(runes) {
		for end > 0 && used+coverRuneWidth('…') > width {
			end--
			used -= coverRuneWidth(runes[end])
		}
		runes = append(append([]rune{}, runes[:end]...), '…')
	}
	// A global line break choice keeps a 14-character heading at 7 + 7 rather
	// than greedily filling 13 characters and leaving a one-character last line.
	n := len(runes)
	prefix := make([]float64, n+1)
	for i, r := range runes {
		prefix[i+1] = prefix[i] + coverRuneWidth(r)
	}
	target := prefix[n] / float64(count)
	cost := make([][]float64, count+1)
	previous := make([][]int, count+1)
	for line := 0; line <= count; line++ {
		cost[line] = make([]float64, n+1)
		previous[line] = make([]int, n+1)
		for i := range cost[line] {
			cost[line][i] = math.Inf(1)
			previous[line][i] = -1
		}
	}
	cost[0][0] = 0
	for line := 1; line <= count; line++ {
		for finish := 1; finish <= n; finish++ {
			for start := finish - 1; start >= 0; start-- {
				units := prefix[finish] - prefix[start]
				if units > width+1e-9 {
					break
				}
				if math.IsInf(cost[line-1][start], 1) || strings.TrimSpace(string(runes[start:finish])) == "" {
					continue
				}
				candidate := cost[line-1][start] + (units-target)*(units-target)
				if finish < n {
					if coverWordRune(runes[finish-1]) && coverWordRune(runes[finish]) {
						candidate += width * width
					}
					if strings.ContainsRune("，。！？、；：）》】」』,.!?;:)]}", runes[finish]) || strings.ContainsRune("（《【「『([{", runes[finish-1]) {
						candidate += width * width
					}
					if coverRuneWidth(runes[finish]) == 0 || runes[finish-1] == '\u200d' {
						candidate += width * width * 4
					}
				}
				if line == count && finish == n && utf8.RuneCountInString(strings.TrimSpace(string(runes[start:finish]))) == 1 && n > 1 {
					candidate += width * width * 4
				}
				if candidate < cost[line][finish] {
					cost[line][finish] = candidate
					previous[line][finish] = start
				}
			}
		}
	}
	lines := make([]string, count)
	for line, finish := count, n; line > 0; line-- {
		start := previous[line][finish]
		if start < 0 {
			return []string{string(runes)}
		} // Defensive: configured widths always fit a full-width glyph.
		lines[line-1] = strings.TrimSpace(string(runes[start:finish]))
		finish = start
	}
	return lines
}

func coverWordRune(r rune) bool { return unicode.Is(unicode.Latin, r) || unicode.IsDigit(r) }

func coverRuneWidth(r rune) float64 {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if unicode.IsSpace(r) {
		return .32
	}
	if r == 'W' || r == 'M' {
		return .9
	}
	if r < 128 || unicode.Is(unicode.Latin, r) {
		if unicode.IsUpper(r) {
			return .65
		}
		return .56
	}
	return 1
}

// The version is part of the SVG and its content-derived ETag, so changing the
// renderer invalidates old covers without changing saved revision metadata.
const coverRendererVersion = "gradient-v1"

func renderCoverSVG(title string, c CoverConfig) string {
	type palette struct{ deep, middle, light, paper, wash, ink string }
	colors := map[string]palette{
		"ink":  {web.Color(web.ColorCoverInkDeep, web.Light), web.Color(web.ColorCoverInkMiddle, web.Light), web.Color(web.ColorCoverInkLight, web.Light), web.Color(web.ColorCoverInkPaper, web.Light), web.Color(web.ColorCoverInkWash, web.Light), web.Color(web.ColorCoverInkInk, web.Light)},
		"moss": {web.Color(web.ColorCoverMossDeep, web.Light), web.Color(web.ColorCoverMossMiddle, web.Light), web.Color(web.ColorCoverMossLight, web.Light), web.Color(web.ColorCoverMossPaper, web.Light), web.Color(web.ColorCoverMossWash, web.Light), web.Color(web.ColorCoverMossInk, web.Light)},
		"clay": {web.Color(web.ColorCoverClayDeep, web.Light), web.Color(web.ColorCoverClayMiddle, web.Light), web.Color(web.ColorCoverClayLight, web.Light), web.Color(web.ColorCoverClayPaper, web.Light), web.Color(web.ColorCoverClayWash, web.Light), web.Color(web.ColorCoverClayInk, web.Light)},
		"plum": {web.Color(web.ColorCoverPlumDeep, web.Light), web.Color(web.ColorCoverPlumMiddle, web.Light), web.Color(web.ColorCoverPlumLight, web.Light), web.Color(web.ColorCoverPlumPaper, web.Light), web.Color(web.ColorCoverPlumWash, web.Light), web.Color(web.ColorCoverPlumInk, web.Light)},
	}
	p, ok := colors[c.Color]
	if !ok {
		p = colors["ink"]
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="675" viewBox="0 0 1200 675" role="img" aria-labelledby="cover-title" data-renderer="%s"><title id="cover-title">%s</title><defs>`, coverRendererVersion, xmlText(title))
	// Broad light fields give the entire image color and depth. All visible
	// geometry is the full canvas; the title is the only foreground element.
	x, size, width, lineHeight, family := 92, 72, 13.8, 90, "'Noto Serif CJK SC','Source Han Serif SC','Songti SC',serif"
	ink, anchor := web.Color(web.ColorCoverTitle, web.Light), "start"
	switch c.Template {
	case "geometric":
		fmt.Fprintf(&out, `<linearGradient id="cover-base" x1="0" y1="1" x2="1" y2="0"><stop stop-color="%s"/><stop offset=".62" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient><radialGradient id="cover-light" cx="1" cy="0" r="1.1"><stop stop-color="%s" stop-opacity=".5"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`, p.deep, p.middle, p.light, p.light, p.middle)
		size, width, lineHeight, family = 70, 14.4, 88, "'Noto Sans CJK SC','Source Han Sans SC','PingFang SC','Microsoft YaHei',sans-serif"
	case "minimal":
		fmt.Fprintf(&out, `<linearGradient id="cover-base" x1="0" y1="0" x2="1" y2="1"><stop stop-color="%s"/><stop offset=".5" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient><radialGradient id="cover-light" cx=".9" cy=".05" r="1"><stop stop-color="%s" stop-opacity=".75"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`, p.paper, p.paper, p.wash, p.wash, p.paper)
		x, size, width, lineHeight, ink, anchor = 600, 66, 15.4, 84, p.ink, "middle"
	default:
		fmt.Fprintf(&out, `<linearGradient id="cover-base" x1="0" y1="1" x2="1" y2="0"><stop stop-color="%s"/><stop offset=".65" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient><radialGradient id="cover-light" cx=".92" cy=".08" r=".9"><stop stop-color="%s" stop-opacity=".4"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`, p.deep, p.deep, p.middle, p.light, p.middle)
	}
	out.WriteString(`</defs><rect width="1200" height="675" fill="url(#cover-base)"/><rect width="1200" height="675" fill="url(#cover-light)"/>`)
	lines := coverLines(title, width, 4)
	y := 363 - ((len(lines)-1)*lineHeight)/2
	if c.Template == "geometric" {
		y = 545 - (len(lines)-1)*lineHeight
	}
	fmt.Fprintf(&out, `<g fill="%s" font-family="%s" font-size="%d" font-weight="600" text-anchor="%s">`, ink, family, size, anchor)
	for i, line := range lines {
		fmt.Fprintf(&out, `<text x="%d" y="%d">%s</text>`, x, y+i*lineHeight, xmlText(line))
	}
	out.WriteString(`</g></svg>`)
	return out.String()
}
