// Package curate resolves the activity/archival status of catalog items and
// classifies them according to the curation policy.
package curate

import (
	"context"
	"time"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/checker"
)

// State is the placement classification of an item.
type State int

const (
	// StateActive means the item stays in its section.
	StateActive State = iota
	// StateArchived means archived upstream or inactive for >= the archived
	// threshold; it is moved to the archive section.
	StateArchived
)

// Freshness is a coarse indication of how recently a project saw activity.
type Freshness int

const (
	// FreshUnknown means activity could not be determined (non-repo link or a
	// failed check with no cached data).
	FreshUnknown Freshness = iota
	// FreshGreen: active within the green threshold.
	FreshGreen
	// FreshYellow: active within the yellow threshold.
	FreshYellow
	// FreshOrange: active within the orange threshold.
	FreshOrange
	// FreshRed: older than the orange threshold.
	FreshRed
)

// Resolved is an item paired with its resolved status and classification.
type Resolved struct {
	Section   string
	Item      catalog.Item
	Status    checker.Status
	State     State
	Freshness Freshness
}

// Options controls resolution and classification.
type Options struct {
	// ArchiveAfter is the inactivity duration after which an item is archived
	// (default 2 years).
	ArchiveAfter time.Duration
	// GreenMonths, YellowMonths and OrangeMonths are the freshness thresholds
	// in months (defaults 2, 4, 8). Activity older than OrangeMonths is red.
	GreenMonths  int
	YellowMonths int
	OrangeMonths int
	// Now is the reference time (default time.Now).
	Now time.Time
	// Offline skips live API calls and relies solely on cache and manual data.
	Offline bool
}

func (o Options) withDefaults() Options {
	if o.ArchiveAfter == 0 {
		o.ArchiveAfter = 2 * 365 * 24 * time.Hour
	}
	if o.GreenMonths == 0 {
		o.GreenMonths = 2
	}
	if o.YellowMonths == 0 {
		o.YellowMonths = 4
	}
	if o.OrangeMonths == 0 {
		o.OrangeMonths = 8
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	return o
}

// Resolver coordinates checks, caching and manual overrides.
type Resolver struct {
	Client *checker.Client
	Cache  *Cache
}

// Resolve produces classified items for every entry in the catalog, in order.
func (r *Resolver) Resolve(ctx context.Context, c *catalog.Catalog, opts Options) []Resolved {
	opts = opts.withDefaults()
	var out []Resolved
	for _, sec := range c.Sections {
		for _, item := range sec.Items {
			st := r.status(ctx, item, opts)
			res := Resolved{Section: sec.Name, Item: item, Status: st}
			res.State = classify(st, opts)
			res.Freshness = freshness(st, opts)
			out = append(out, res)
		}
	}
	return out
}

// status determines the effective status of an item, applying manual overrides
// and falling back to cache when a live check fails.
func (r *Resolver) status(ctx context.Context, item catalog.Item, opts Options) checker.Status {
	_, isRepo := checker.ParseRepo(item.URL)

	var st checker.Status
	if !item.SkipCheck && isRepo {
		if !opts.Offline && r.Client != nil {
			st = r.Client.Check(ctx, item.URL)
			if st.Known && r.Cache != nil {
				r.Cache.Put(item.URL, st, opts.Now)
			}
		}
		if !st.Known && r.Cache != nil {
			if e, ok := r.Cache.Get(item.URL); ok {
				st = e.Status
				st.Source = "cache"
			}
		}
	}

	st = applyManual(st, item)
	return st
}

// applyManual overlays manual TOML overrides on top of an API/cache status.
func applyManual(st checker.Status, item catalog.Item) checker.Status {
	if item.Updated != "" {
		if t, err := time.Parse("2006-01-02", item.Updated); err == nil {
			st.LastActivity = t
			st.Known = true
			if st.Source == "" || st.Source == "unsupported" || st.Source == "error" {
				st.Source = "manual"
			}
			st.Err = ""
		}
	}
	if item.Archived != nil {
		st.Archived = *item.Archived
		st.Known = true
		if st.Source == "" || st.Source == "unsupported" || st.Source == "error" {
			st.Source = "manual"
		}
		st.Err = ""
	}
	return st
}

// classify decides whether an item stays in its section or moves to the
// archive section. An unknown status never moves an item.
func classify(st checker.Status, opts Options) State {
	if !st.Known {
		return StateActive
	}
	if st.Archived {
		return StateArchived
	}
	if st.LastActivity.IsZero() {
		return StateActive
	}
	if opts.Now.Sub(st.LastActivity) >= opts.ArchiveAfter {
		return StateArchived
	}
	return StateActive
}

// freshness maps the last activity date onto the colour scale. Repositories
// archived upstream report unknown freshness, since recent pre-archival pushes
// would otherwise be a misleading "maintained" signal.
func freshness(st checker.Status, opts Options) Freshness {
	if !st.Known || st.Archived || st.LastActivity.IsZero() {
		return FreshUnknown
	}
	now := opts.Now
	switch {
	case !st.LastActivity.Before(now.AddDate(0, -opts.GreenMonths, 0)):
		return FreshGreen
	case !st.LastActivity.Before(now.AddDate(0, -opts.YellowMonths, 0)):
		return FreshYellow
	case !st.LastActivity.Before(now.AddDate(0, -opts.OrangeMonths, 0)):
		return FreshOrange
	default:
		return FreshRed
	}
}
