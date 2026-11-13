package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHomeV36CardNameCompatibleConfigWrites(t *testing.T) {
	old := HomeConfig{IntroCardName: "Saved card name", Sections: map[string]HomeSection{}, Activity: defaultHomeActivity()}
	for _, sample := range []struct {
		name, raw, want string
	}{
		{"legacy omission retains saved value", `{"sections":{}}`, "Saved card name"},
		{"explicit empty restores name fallback", `{"sections":{},"intro_card_name":""}`, ""},
		{"explicit replacement", `{"sections":{},"intro_card_name":"Independent card name"}`, "Independent card name"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			var incoming HomeConfig
			if err := json.Unmarshal([]byte(sample.raw), &incoming); err != nil {
				t.Fatal(err)
			}
			mergeHomeAdditions(&incoming, old)
			if incoming.IntroCardName != sample.want {
				t.Fatalf("card name = %q, want %q", incoming.IntroCardName, sample.want)
			}
			encoded, err := json.Marshal(incoming)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip HomeConfig
			if err := json.Unmarshal(encoded, &roundTrip); err != nil || roundTrip.IntroCardName != sample.want || !roundTrip.introCardNamePresent {
				t.Fatal("explicit saved value lost on JSON round trip", err)
			}
		})
	}
	for _, raw := range []string{`null`, `42`, `true`, `[]`, `{}`} {
		var candidate HomeConfig
		if err := json.Unmarshal([]byte(`{"intro_card_name":`+raw+`}`), &candidate); err == nil {
			t.Fatalf("non-string card name accepted: %s", raw)
		}
	}
}

func TestHomeV36CardNameValidation(t *testing.T) {
	cfg := HomeConfig{Order: append([]string{}, homeSectionKeys...), Sections: map[string]HomeSection{}, IntroRoles: []string{}, Activity: defaultHomeActivity()}
	for _, key := range homeSectionKeys {
		cfg.Sections[key] = HomeSection{}
	}
	for _, name := range []string{"", "A configured card", strings.Repeat("界", 80)} {
		cfg.IntroCardName = name
		if err := validateHome(cfg); err != nil {
			t.Fatal("valid card name rejected", err)
		}
	}
	for _, name := range []string{strings.Repeat("界", 81), "Name\nSecond line", "Name\x00"} {
		cfg.IntroCardName = name
		if validateHome(cfg) == nil {
			t.Fatal("unbounded or control-bearing card name accepted")
		}
	}
}

func TestHomeV36CopyTemplateKeepsIndependentNameAndAccessibleDates(t *testing.T) {
	templates := v31NavigationTemplates(t)
	render := func(name string, section homeWebSection) string {
		t.Helper()
		var output bytes.Buffer
		if err := templates.ExecuteTemplate(&output, name, section); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	intro := friendWebDocument(t, render("home-introduction", homeWebSection{Name: "Nickname", CardName: "Configured card", Title: "Hello", Description: "The large introduction", Brand: "Painter"}))
	for class, want := range map[string]string{"personal-name": "Nickname", "story-sheet-title": "Configured card", "story-sheet-signature": "Painter"} {
		nodes := friendWebClass(intro, class)
		if len(nodes) != 1 || strings.TrimSpace(friendWebText(nodes[0])) != want {
			t.Fatalf("%s must use its own unpunctuated configured copy", class)
		}
	}
	if len(friendWebClass(intro, "story-intro-line"))+len(friendWebClass(intro, "story-sheet-intro")) != 0 || !strings.Contains(friendWebText(intro), "The large introduction") {
		t.Fatal("duplicate small copy remained or large introduction was removed")
	}
	project := friendWebDocument(t, render("home-projects", homeWebSection{Items: []map[string]any{{"title": "Saved project", "source": "github", "url": "https://github.com/example/project"}}}))
	if len(friendWebClass(project, "project-deck-hint")) != 0 || !strings.Contains(friendWebText(project), "Saved project") {
		t.Fatal("project hint remained or actual content disappeared")
	}
	activity := friendWebDocument(t, render("home-activity", homeWebSection{Activity: HomeActivity{Weeks: [][]HomeActivityDay{{{Date: "2026-11-08", InRange: true}}}}}))
	detail := friendWebClass(activity, "sr-only")
	region := friendWebClass(activity, "activity-scroll")
	if len(detail) != 1 || !friendWebHasAttr(detail[0], "data-activity-detail") || friendWebAttr(detail[0], "aria-live") != "polite" || len(region) != 1 || friendWebAttr(region[0], "aria-describedby") != friendWebAttr(detail[0], "id") {
		t.Fatal("hidden live date instructions or their accessible relationship lost")
	}
	if len(friendWebClass(activity, "activity-day")) != 1 {
		t.Fatal("real keyboard-addressable dates disappeared")
	}
	closing := render("home-closing", homeWebSection{Title: "A configured conclusion", Name: "Must not appear"})
	if strings.Contains(closing, "Must not appear") || strings.Contains(closing, "<p>") {
		t.Fatal("empty conclusion description regained a name fallback")
	}
}

func TestHomeV36HeadingsOnlyDropTerminalChineseStop(t *testing.T) {
	for input, want := range map[string]string{"唱片墙。": "唱片墙", "写下来的日子。。": "写下来的日子", "第一句。\n第二句。": "第一句。\n第二句", "A title.": "A title.", "保留？": "保留？"} {
		if got := homeHeadingText(input); got != want {
			t.Fatalf("homepage heading %q became %q, want %q", input, got, want)
		}
	}
}
