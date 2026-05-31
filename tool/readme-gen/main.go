// Command awesome-restic generates README.md from data.toml, checking each
// project's activity and archival status and applying the curation policy.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/checker"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/curate"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		input       = flag.String("input", "../../data.toml", "path to the TOML source of truth")
		output      = flag.String("output", "../../README.md", "path to the generated README")
		cachePath   = flag.String("cache", ".cache/status.json", "path to the status cache (empty to disable)")
		dryRun      = flag.Bool("dry-run", false, "print the README to stdout instead of writing it")
		offline     = flag.Bool("offline", false, "skip live API calls and use cache/manual data only")
		checkMode   = flag.Bool("check", false, "exit non-zero if the output file is out of date")
		archiveYrs  = flag.Float64("archive-years", 2, "years of inactivity before archiving an item")
		maxFailRate = flag.Float64("max-fail-rate", 0.5, "fail if more than this fraction of repo checks error")
	)
	flag.Parse()

	cat, err := catalog.Load(*input)
	if err != nil {
		return err
	}

	cache, err := curate.LoadCache(*cachePath)
	if err != nil {
		return err
	}

	client := checker.New(os.Getenv("GITHUB_TOKEN"))
	resolver := &curate.Resolver{Client: client, Cache: cache}

	now := time.Now().UTC()
	year := 365 * 24 * time.Hour
	opts := curate.Options{
		Now:          now,
		Offline:      *offline,
		ArchiveAfter: time.Duration(*archiveYrs * float64(year)),
	}

	ctx := context.Background()
	resolved := resolver.Resolve(ctx, cat, opts)

	if err := guardFailRate(resolved, *offline, *maxFailRate); err != nil {
		return err
	}

	if !*offline {
		if err := cache.Save(*cachePath); err != nil {
			return err
		}
	}

	out := render.Render(cat, resolved)
	printReport(resolved, *offline)

	if *dryRun {
		fmt.Print(out)
		return nil
	}

	if *checkMode {
		return checkUpToDate(*output, out)
	}

	if err := os.WriteFile(*output, []byte(out), 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", *output, err)
	}
	return nil
}

// guardFailRate aborts if too many repository checks failed without cache
// fallback, so a broad outage never silently produces a degraded README.
func guardFailRate(resolved []curate.Resolved, offline bool, max float64) error {
	if offline {
		return nil
	}
	var repos, failed int
	for _, r := range resolved {
		if _, ok := checker.ParseRepo(r.Item.CheckURL()); !ok || r.Item.SkipCheck {
			continue
		}
		repos++
		if r.Status.Source == "error" {
			failed++
		}
	}
	if repos == 0 {
		return nil
	}
	if rate := float64(failed) / float64(repos); rate > max {
		return fmt.Errorf("too many repo checks failed: %d/%d (%.0f%%) > %.0f%%; aborting to avoid a degraded README",
			failed, repos, rate*100, max*100)
	}
	return nil
}

func checkUpToDate(path, want string) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(got) != want {
		return fmt.Errorf("%s is out of date; run the generator and commit the result", path)
	}
	return nil
}

func printReport(resolved []curate.Resolved, offline bool) {
	var counts [5]int
	var archived []curate.Resolved
	for _, r := range resolved {
		counts[r.Freshness]++
		if r.State == curate.StateArchived {
			archived = append(archived, r)
		}
	}
	sort.SliceStable(archived, func(i, j int) bool { return archived[i].Item.Name < archived[j].Item.Name })

	fmt.Fprintf(os.Stderr, "Freshness: 🟢 %d  🟡 %d  🟠 %d  🔴 %d  ⚪ %d\n",
		counts[curate.FreshGreen], counts[curate.FreshYellow], counts[curate.FreshOrange],
		counts[curate.FreshRed], counts[curate.FreshUnknown])
	fmt.Fprintf(os.Stderr, "Archived / inactive: %d\n", len(archived))
	for _, r := range archived {
		reason := "inactive"
		if r.Status.Archived {
			reason = "archived upstream"
		} else if !r.Status.LastActivity.IsZero() {
			reason = "last activity " + r.Status.LastActivity.Format("2006-01-02")
		}
		fmt.Fprintf(os.Stderr, "  📦 %s (%s, %s)\n", r.Item.Name, reason, r.Status.Source)
	}

	printUnresolvedWarnings(resolved, offline)
}

// printUnresolvedWarnings flags entries that point at a checkable repository but
// could not be resolved, so a silent ⚪ on a repo-backed entry is never missed.
func printUnresolvedWarnings(resolved []curate.Resolved, offline bool) {
	warns := unresolvedRepoEntries(resolved)
	if len(warns) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "Warning: %d repo entr%s could not be checked and show as ⚪ unknown:\n",
		len(warns), plural(len(warns), "y", "ies"))
	for _, r := range warns {
		reason := "no data"
		switch {
		case r.Status.Err != "":
			reason = r.Status.Err
		case offline:
			reason = "offline and not in cache"
		}
		fmt.Fprintf(os.Stderr, "  ⚠️  %s (%s) — %s\n", r.Item.Name, r.Item.CheckURL(), reason)
	}
}

// unresolvedRepoEntries returns entries whose check URL is a recognised
// repository but whose status could not be determined.
func unresolvedRepoEntries(resolved []curate.Resolved) []curate.Resolved {
	var warns []curate.Resolved
	for _, r := range resolved {
		if r.Item.SkipCheck {
			continue
		}
		if _, ok := checker.ParseRepo(r.Item.CheckURL()); !ok {
			continue // genuinely a non-repo link; ⚪ is expected
		}
		if !r.Status.Known {
			warns = append(warns, r)
		}
	}
	sort.SliceStable(warns, func(i, j int) bool { return warns[i].Item.Name < warns[j].Item.Name })
	return warns
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
