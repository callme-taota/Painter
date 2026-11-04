package app

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

type homeWebSection struct {
	Key, Title, Description, LinkLabel, LinkURL                string
	Name, WritingTitle, WritingDescription, ScrollLabel, Brand string
	Roles                                                      []string
	Items                                                      []map[string]any
	MusicBadgeURL                                              string
	MusicPlaylist                                              map[string]any
	FollowingMusic                                             *homeWebSection
	FollowingGames                                             *homeWebSection
	GroupedWithGames                                           bool
	GroupedWithMusic                                           bool
	WorkJourneyStart, WorkJourneyEnd                           bool
	Activity                                                   HomeActivity
}

// Provider imports and synchronization happen separately. Rendering only reads
// the saved homepage and current public articles, never external services.
func (a *App) homepagePage(ctx context.Context, site map[string]any) (webPage, error) {
	data, err := a.HomepageData(ctx)
	if err != nil {
		return webPage{}, err
	}
	cfg := data["config"].(HomeConfig)
	walls := data["walls"].(map[string][]map[string]any)
	name, description, roles := cfg.IntroName, cfg.Sections["intro"].Description, cfg.IntroRoles
	if cfg.UseSiteIntro {
		name = webString(site["intro_name"])
		if name == "" {
			name = webString(site["display_name"])
		}
		description = webString(site["tagline"])
		roles, _ = site["intro_roles"].([]string)
	}
	p := webPage{Kind: "home", Site: site, HomeConfig: &cfg}
	for _, key := range cfg.Order {
		s := cfg.Sections[key]
		if !s.Enabled {
			continue
		}
		section := homeWebSection{Key: key, Title: s.Title, Description: s.Description, LinkLabel: s.LinkLabel, LinkURL: s.LinkURL,
			Name: name, Roles: roles, WritingTitle: cfg.WritingTitle, WritingDescription: cfg.WritingDescription,
			ScrollLabel: cfg.ScrollLabel, Brand: webString(site["display_name"])}
		switch key {
		case "intro":
			section.Description = description
			p.HomeHasIntro = section.Title != "" || name != ""
			// The default article jump remains useful if its destination is hidden.
			if (section.LinkURL == "/#home-writing" || section.LinkURL == "#home-writing") && (!cfg.Sections["articles"].Enabled || cfg.Sections["articles"].Limit == 0) {
				section.LinkURL = "/posts"
			}
		case "articles":
			if s.Limit == 0 {
				continue
			}
			posts, e := a.PublicPosts(ctx, url.Values{"limit": {strconv.Itoa(s.Limit)}})
			if e != nil {
				return webPage{}, e
			}
			section.Items = webMaps(posts["items"])
		case "activity":
			section.Activity, _ = data["activity"].(HomeActivity)
		case "projects":
			if s.Limit == 0 {
				continue
			}
			section.Items = walls["projects"]
		case "steam", "music", "photos":
			if s.Limit == 0 || len(walls[key]) == 0 {
				continue
			}
			section.Items = walls[key]
			if key == "steam" && len(section.Items) > 5 {
				section.Items = section.Items[:5]
			}
			if key == "music" {
				section.MusicPlaylist, err = a.PublicMusicPlaylist(ctx)
				if err != nil {
					return webPage{}, err
				}
				for _, item := range section.Items {
					u, e := url.Parse(webString(item["url"]))
					if e == nil && u.Scheme == "https" && strings.EqualFold(u.Host, "music.apple.com") {
						section.MusicBadgeURL = u.String()
						break
					}
				}
			}
		case "closing":
			if section.Description == "" && cfg.UseSiteIntro {
				section.Description = name
			}
		}
		if p.HomeFirst == "" {
			p.HomeFirst = key
		}
		p.HomeSections = append(p.HomeSections, section)
	}
	groupHomeSections(p.HomeSections)
	return p, nil
}

// Group only rendered neighbors, preserving each owner's configured order.
// The trailing section remains in the slice for ordering but renders via its
// preceding section, so templates skip its standalone output.
func groupHomeSections(sections []homeWebSection) {
	for i := 0; i+2 < len(sections); i++ {
		if sections[i].Key == "intro" && sections[i+1].Key == "projects" && sections[i+2].Key == "activity" {
			sections[i].WorkJourneyStart = true
			sections[i+2].WorkJourneyEnd = true
		}
	}
	for i := 0; i+1 < len(sections); i++ {
		first, next := &sections[i], &sections[i+1]
		switch {
		case first.Key == "steam" && next.Key == "music":
			first.FollowingMusic = next
			next.GroupedWithGames = true
		case first.Key == "music" && next.Key == "steam":
			first.FollowingGames = next
			next.GroupedWithMusic = true
		}
	}
}

func gameHours(value any) string {
	if value == nil {
		return "时长未公开"
	}
	minutes, err := strconv.ParseInt(webString(value), 10, 64)
	if err != nil || minutes < 0 {
		return "时长未公开"
	}
	if minutes == 0 {
		return "0 小时"
	}
	if minutes < 6 {
		return "不足 0.1 小时"
	}
	return strconv.FormatFloat(float64(minutes)/60, 'f', 1, 64) + " 小时"
}
