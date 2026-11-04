package app

import (
	"context"
	"net/http"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5"
)

type HomeActivityOptions struct {
	Source   string `json:"source"`
	Timezone string `json:"timezone"`
	Period   string `json:"period"`
	Year     int    `json:"year"`
}
type HomeActivityDay struct {
	Date    string `json:"date"`
	Count   int    `json:"count"`
	Level   int    `json:"level"`
	InRange bool   `json:"in_range"`
	Future  bool   `json:"future"`
}
type HomeActivity struct {
	Enabled     bool                `json:"enabled"`
	Days        []HomeActivityDay   `json:"days"`
	Weeks       [][]HomeActivityDay `json:"weeks"`
	Total       int                 `json:"total"`
	ActiveDays  int                 `json:"active_days"`
	Year        int                 `json:"year"`
	Start       string              `json:"start"`
	End         string              `json:"end"`
	Timezone    string              `json:"timezone"`
	Source      string              `json:"source"`
	SourceLabel string              `json:"source_label"`
	Period      string              `json:"period"`
}

func defaultHomeActivity() HomeActivityOptions {
	return HomeActivityOptions{Source: "posts_moments", Timezone: "Asia/Shanghai", Period: "rolling"}
}
func validateHomeActivity(o HomeActivityOptions) error {
	if o.Source != "posts_moments" && o.Source != "posts" && o.Source != "moments" {
		return E(400, "VALIDATION_FAILED", "请选择文章、瞬间或文章与瞬间作为统计来源")
	}
	if len(o.Timezone) > 100 || o.Timezone == "" || o.Timezone == "Local" {
		return E(400, "VALIDATION_FAILED", "请选择明确的统计时区")
	}
	if _, e := time.LoadLocation(o.Timezone); e != nil {
		return E(400, "VALIDATION_FAILED", "统计时区无效")
	}
	if (o.Period != "rolling" && o.Period != "year") || (o.Year != 0 && (o.Year < 2000 || o.Year > 2100)) {
		return E(400, "VALIDATION_FAILED", "统计范围或年份无效")
	}
	return nil
}

type homeRowsQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// Every currently-public item contributes once on its first publication date.
// Draft edits, republishing, deleted items and withdrawn items add no events.
// UTC bounds are derived from the chosen IANA zone, never the server's zone.
func homeActivityAt(ctx context.Context, q homeRowsQuery, c HomeConfig, now time.Time) (HomeActivity, error) {
	o := c.Activity
	if e := validateHomeActivity(o); e != nil {
		return HomeActivity{}, e
	}
	out := HomeActivity{Enabled: c.Sections["activity"].Enabled, Days: []HomeActivityDay{}, Weeks: [][]HomeActivityDay{}, Timezone: o.Timezone, Source: o.Source, Period: o.Period}
	out.SourceLabel = map[string]string{"posts_moments": "公开文章与瞬间", "posts": "公开文章", "moments": "公开瞬间"}[o.Source]
	if !out.Enabled {
		return out, nil
	}
	loc, _ := time.LoadLocation(o.Timezone)
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	start, end := today.AddDate(0, 0, -364), today.AddDate(0, 0, 1)
	if o.Period == "year" {
		year := o.Year
		if year == 0 {
			year = today.Year()
		}
		start = time.Date(year, 1, 1, 0, 0, 0, 0, loc)
		end = start.AddDate(1, 0, 0)
	}
	out.Start = start.Format("2006-01-02")
	out.End = end.AddDate(0, 0, -1).Format("2006-01-02")
	out.Year = start.Year()
	upper := end
	if tomorrow := today.AddDate(0, 0, 1); upper.After(tomorrow) {
		upper = tomorrow
	}
	counts := map[string]int{}
	if upper.After(start) {
		rows, e := q.Query(ctx, `SELECT (to_timestamp(ms::double precision/1000) AT TIME ZONE $3)::date::text,count(*)::int FROM (
   SELECT first_published_at_ms AS ms FROM public_posts WHERE $4::boolean AND first_published_at_ms >= $1 AND first_published_at_ms < $2
   UNION ALL SELECT first_published_at_ms FROM public_moments WHERE $5::boolean AND first_published_at_ms >= $1 AND first_published_at_ms < $2
  ) activity GROUP BY 1`, start.UnixMilli(), upper.UnixMilli(), o.Timezone, o.Source != "moments", o.Source != "posts")
		if e != nil {
			return out, e
		}
		for rows.Next() {
			var day string
			var n int
			if e = rows.Scan(&day, &n); e != nil {
				rows.Close()
				return out, e
			}
			counts[day] = n
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
	}
	gridStart := start.AddDate(0, 0, -int(start.Weekday()))
	gridEnd := end
	if gridEnd.Weekday() != time.Sunday {
		gridEnd = gridEnd.AddDate(0, 0, 7-int(gridEnd.Weekday()))
	}
	week := []HomeActivityDay{}
	for d := gridStart; d.Before(gridEnd); d = d.AddDate(0, 0, 1) {
		day := HomeActivityDay{Date: d.Format("2006-01-02"), InRange: !d.Before(start) && d.Before(end), Future: d.After(today)}
		if day.InRange && !day.Future {
			day.Count = counts[day.Date]
			if day.Count > 0 {
				out.Total += day.Count
				out.ActiveDays++
				day.Level = min(day.Count, 4)
			}
		}
		if day.InRange {
			out.Days = append(out.Days, day)
		}
		week = append(week, day)
		if len(week) == 7 {
			out.Weeks = append(out.Weeks, week)
			week = []HomeActivityDay{}
		}
	}
	return out, nil
}
func (a *App) PublicHomeActivity(ctx context.Context, c HomeConfig) (HomeActivity, error) {
	return homeActivityAt(ctx, a.Pool, c, time.Now())
}
func (a *App) homeActivityPreview(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	s, e := homeRead(r.Context(), a.Pool, false)
	if e != nil {
		return e
	}
	o := s.Config.Activity
	q := r.URL.Query()
	if q.Has("source") {
		o.Source = q.Get("source")
	}
	if q.Has("timezone") {
		o.Timezone = q.Get("timezone")
	}
	if q.Has("period") {
		o.Period = q.Get("period")
	}
	if q.Has("year") {
		year, e := strconv.Atoi(q.Get("year"))
		if e != nil {
			return E(400, "VALIDATION_FAILED", "年份无效")
		}
		o.Year = year
	}
	s.Config.Activity = o
	v := s.Config.Sections["activity"]
	v.Enabled = true
	s.Config.Sections["activity"] = v
	out, e := a.PublicHomeActivity(r.Context(), s.Config)
	if e == nil {
		JSON(w, 200, out)
	}
	return e
}
