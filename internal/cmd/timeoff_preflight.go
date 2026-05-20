package cmd

import (
	"context"
	"net/url"
	"sort"
	"time"

	"github.com/supervisible/supervisible-cli/internal/api"
)

// preflightTimeOff scans rawBody["items"] (upsert envelope) and emits one stderr
// warning per (user, time-off) overlap. Best-effort: a fetch error becomes a
// single soft warning, never a hard fail. Only runs when --dry-run is set.
func preflightTimeOff(ctx context.Context, app *App, rawBody map[string]any) {
	if !app.DryRun() {
		return
	}
	itemsRaw, ok := rawBody["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return
	}

	// Bucket items per user with min/max date.
	type bucket struct {
		minDate, maxDate string
		dates            []string
	}
	users := map[string]*bucket{}
	for _, raw := range itemsRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid, _ := item["userId"].(string)
		date, _ := item["date"].(string)
		if uid == "" || date == "" {
			continue
		}
		b, ok := users[uid]
		if !ok {
			b = &bucket{minDate: date, maxDate: date}
			users[uid] = b
		}
		if date < b.minDate {
			b.minDate = date
		}
		if date > b.maxDate {
			b.maxDate = date
		}
		b.dates = append(b.dates, date)
	}
	if len(users) == 0 {
		return
	}

	client, err := app.RequireClient()
	if err != nil {
		app.Printer().Aux("warning: could not verify time-off (no API key); proceeding without check")
		return
	}

	// Determinism for tests + readable output.
	uids := make([]string, 0, len(users))
	for uid := range users {
		uids = append(uids, uid)
	}
	sort.Strings(uids)

	for _, uid := range uids {
		b := users[uid]
		q := url.Values{}
		q.Set("user_id", uid)
		q.Set("status", "approved")
		q.Set("start_date", b.minDate)
		q.Set("end_date", b.maxDate)
		q.Set("limit", "50")
		q.Set("expand", "timeOffType,user")

		var timeOff []api.TimeOffRequest
		if err := client.Do(ctx, "GET", "/time-off", q, nil, &timeOff); err != nil {
			app.Printer().Aux("warning: could not verify time-off for user %s (%v); proceeding without check", uid, err)
			continue
		}

		// Collapse: one warning per (user, time-off entry) regardless of how many
		// items overlap it.
		warned := map[string]struct{}{}
		for _, date := range b.dates {
			d, err := time.Parse("2006-01-02", date)
			if err != nil {
				continue
			}
			for _, to := range timeOff {
				if to.Status != "approved" {
					continue
				}
				toStart, err1 := time.Parse("2006-01-02", to.StartDate)
				toEnd, err2 := time.Parse("2006-01-02", to.EndDate)
				if err1 != nil || err2 != nil {
					continue
				}
				if d.Before(toStart) || d.After(toEnd) {
					continue
				}
				if _, seen := warned[to.ID]; seen {
					continue
				}
				warned[to.ID] = struct{}{}

				name := uid
				if to.User != nil && to.User.Name != nil && *to.User.Name != "" {
					name = *to.User.Name
				}
				typeName := to.TimeOffTypeID
				if to.TimeOffType != nil && to.TimeOffType.Name != "" {
					typeName = to.TimeOffType.Name
				}
				app.Printer().Aux(
					"warning: time-off overlap — %s has approved %s %s → %s",
					name, typeName, to.StartDate, to.EndDate,
				)
			}
		}
	}
}
