package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/language"
	"painter-next/web"
)

const defaultLocale = "zh-CN"
const localeCookie = "painter_locale"

var supportedLocales = []string{defaultLocale, "en"}
var englishMessages struct {
	sync.Once
	messages map[string]string
}

// UI messages use their Chinese source as stable IDs. Content and profile fields
// are never passed through this catalogue, and missing translations stay usable.
func translateUI(locale, source string, args ...any) string {
	value := source
	if locale == "en" {
		englishMessages.Do(func() {
			englishMessages.messages = map[string]string{}
			if data, err := web.FS.ReadFile("locales/en.json"); err == nil {
				_ = json.Unmarshal(data, &englishMessages.messages)
			}
		})
		if translated := englishMessages.messages[source]; translated != "" {
			value = translated
		}
	}
	replacements := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		replacements = append(replacements, "{"+fmt.Sprint(args[i])+"}", fmt.Sprint(args[i+1]))
	}
	// One pass prevents user-supplied values containing another placeholder
	// from being interpreted as message syntax.
	if len(replacements) > 0 {
		value = strings.NewReplacer(replacements...).Replace(value)
	}
	return value
}

// Only application code may mark a nested UI phrase as translatable. Request
// values such as author names, property keys and supplied text stay opaque.
type translatableUIValue struct {
	source string
	args   []any
}

func translatableMessage(source string, args ...any) translatableUIValue {
	return translatableUIValue{source: source, args: append([]any(nil), args...)}
}

func localizedUIMessage(locale, source string, args ...any) string {
	values := append([]any(nil), args...)
	for i := 1; i < len(values); i += 2 {
		if message, ok := values[i].(translatableUIValue); ok {
			values[i] = localizedUIMessage(locale, message.source, message.args...)
		}
	}
	return translateUI(locale, source, values...)
}

func localeErrorMessage(err *APIError, locale string) string {
	if err.messageSource != "" {
		return localizedUIMessage(locale, err.messageSource, err.messageArgs...)
	}
	return translateUI(locale, err.Message)
}

func canonicalLocale(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "en") || strings.EqualFold(raw, "en-US") || strings.EqualFold(raw, "en-GB") {
		return "en"
	}
	if strings.EqualFold(raw, "zh") || strings.EqualFold(raw, "zh-CN") || strings.EqualFold(raw, "zh-Hans") {
		return defaultLocale
	}
	return ""
}

func requestLocale(r *http.Request) string {
	if locale := canonicalLocale(r.URL.Query().Get("lang")); locale != "" {
		return locale
	}
	if cookie, err := r.Cookie(localeCookie); err == nil {
		if locale := canonicalLocale(cookie.Value); locale != "" {
			return locale
		}
	}
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err == nil {
		for _, tag := range tags {
			base, _ := tag.Base()
			switch base.String() {
			case "en":
				return "en"
			case "zh", "mul": // ParseAcceptLanguage represents the wildcard as mul.
				return defaultLocale
			}
		}
	}
	return defaultLocale
}

func localeResponseHeaders(w http.ResponseWriter, locale string) {
	w.Header().Set("Content-Language", locale)
	// Responses depend on both a saved explicit choice and browser preferences.
	for _, field := range []string{"Accept-Language", "Cookie"} {
		present := false
		for _, vary := range w.Header().Values("Vary") {
			for _, token := range strings.Split(vary, ",") {
				if strings.EqualFold(strings.TrimSpace(token), field) {
					present = true
				}
			}
		}
		if !present {
			w.Header().Add("Vary", field)
		}
	}
}

func localeReturnPath(raw string) string {
	if raw == "" || len(raw) > 4096 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\\r\n") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Opaque != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || u.Path == "/locale" || strings.ContainsAny(u.Path, "\\\r\n") || strings.IndexFunc(u.Path, unicode.IsControl) >= 0 {
		return "/"
	}
	query := u.Query()
	query.Del("lang")
	u.RawQuery = query.Encode()
	return u.String()
}

func (a *App) persistLocaleChoice(w http.ResponseWriter, r *http.Request, locale string) {
	http.SetCookie(w, &http.Cookie{Name: localeCookie, Value: locale, Path: "/", MaxAge: 365 * 24 * 60 * 60, HttpOnly: true, Secure: a.Config.secureRequest(r), SameSite: http.SameSiteLaxMode})
}

func (a *App) registerLocale() {
	a.Mux.HandleFunc("GET /locale", func(w http.ResponseWriter, r *http.Request) {
		locale := canonicalLocale(r.URL.Query().Get("lang"))
		if locale == "" {
			http.Error(w, "Unsupported language", http.StatusBadRequest)
			return
		}
		localeResponseHeaders(w, locale)
		w.Header().Set("Cache-Control", "private, no-store")
		a.persistLocaleChoice(w, r, locale)
		http.Redirect(w, r, localeReturnPath(r.URL.Query().Get("next")), http.StatusSeeOther)
	})
}

func localizedDate(locale string, value any, clock bool) string {
	stamp, err := time.Parse(time.RFC3339Nano, webString(value))
	if err != nil {
		if clock {
			return ""
		}
		return webString(value)
	}
	stamp = stamp.In(time.FixedZone("CST", 8*3600))
	if locale == "en" {
		if clock {
			return stamp.Format("Jan 2, 2006 15:04")
		}
		return stamp.Format("Jan 2, 2006")
	}
	if clock {
		return stamp.Format("2006年1月2日 15:04")
	}
	return stamp.Format("2006年1月2日")
}

func localizedGameHours(locale string, value any) string {
	if locale != "en" {
		return gameHours(value)
	}
	if value == nil {
		return translateUI(locale, "时长未提供")
	}
	minutes, err := strconv.ParseInt(webString(value), 10, 64)
	if err != nil || minutes < 0 {
		return translateUI(locale, "时长未提供")
	}
	if minutes < 6 && minutes > 0 {
		return translateUI(locale, "不足 0.1 小时")
	}
	hours := strconv.FormatFloat(float64(minutes)/60, 'f', 1, 64)
	if minutes == 0 {
		hours = "0"
	}
	return translateUI(locale, "{count} 小时", "count", hours)
}

func localeTemplateFuncs(locale string) template.FuncMap {
	return template.FuncMap{"gamePlatform": func(value any) string {
		if webString(value) == "psn" {
			return "PlayStation"
		}
		return "Steam"
	}, "t": func(source string, args ...any) string { return translateUI(locale, source, args...) }, "date": func(value any) string { return localizedDate(locale, value, false) }, "dateTime": func(value any) string { return localizedDate(locale, value, true) }, "gameHours": func(value any) string { return localizedGameHours(locale, value) }}
}

func localizePageChrome(p *webPage) {
	switch p.Kind {
	case "posts", "series-list", "products", "games", "search", "about", "login", "register", "setup", "studio", "phrases", "moments", "error":
		p.Title = translateUI(p.Locale, p.Title)
	}
	if p.Kind == "series-list" || p.Kind == "games" || p.Kind == "moments" {
		p.Description = translateUI(p.Locale, p.Description)
	}
	p.Error = translateUI(p.Locale, p.Error)
}
