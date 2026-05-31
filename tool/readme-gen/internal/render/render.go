// Package render turns a catalog plus resolved statuses into the README
// markdown, applying the curation layout.
package render

import (
	"fmt"
	"strings"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/curate"
)

const (
	// ArchiveSection is the heading for moved items.
	ArchiveSection = "Archived / Inactive"
	// ArchivedIcon marks entries archived upstream.
	ArchivedIcon = "📦"

	header = "<!-- This file is generated from data.toml. Do not edit README.md directly. -->"

	policyNote = "> Freshness reflects the latest repository activity: " +
		"🟢 < 2 months · 🟡 < 4 months · 🟠 < 8 months · 🔴 older · ⚪ unknown · " + ArchivedIcon + " archived upstream.\n" +
		"> Projects archived upstream or inactive for over two years are listed under **" + ArchiveSection + "** at the bottom.\n" +
		"> This list is generated automatically; to add or update an entry see [CONTRIBUTORS.md](CONTRIBUTORS.md)."
)

// lineIcon is the leading marker for an entry: a dedicated icon when archived
// upstream, otherwise the freshness dot.
func lineIcon(r curate.Resolved) string {
	if r.Status.Known && r.Status.Archived {
		return ArchivedIcon
	}
	return freshnessIcon(r.Freshness)
}

// freshnessIcon maps a freshness level to its emoji.
func freshnessIcon(f curate.Freshness) string {
	switch f {
	case curate.FreshGreen:
		return "🟢"
	case curate.FreshYellow:
		return "🟡"
	case curate.FreshOrange:
		return "🟠"
	case curate.FreshRed:
		return "🔴"
	default:
		return "⚪"
	}
}

// Render produces the full README markdown from the catalog and resolved items.
func Render(c *catalog.Catalog, resolved []curate.Resolved) string {
	bySection := map[string][]curate.Resolved{}
	var archived []curate.Resolved
	for _, r := range resolved {
		if r.State == curate.StateArchived {
			archived = append(archived, r)
			continue
		}
		bySection[r.Section] = append(bySection[r.Section], r)
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "# %s\n\n", c.Title)
	if c.Intro != "" {
		b.WriteString(c.Intro)
		b.WriteString("\n\n")
	}
	b.WriteString(policyNote)
	b.WriteString("\n")

	for _, sec := range c.Sections {
		items := bySection[sec.Name]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n", sec.Name)
		for _, r := range items {
			b.WriteString(activeLine(r))
			b.WriteString("\n")
		}
	}

	if len(archived) > 0 {
		fmt.Fprintf(&b, "\n## %s\n\n", ArchiveSection)
		b.WriteString("These projects are archived upstream or have seen no activity for over two years.\n\n")
		for _, r := range archived {
			b.WriteString(archivedLine(r))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func link(r curate.Resolved) string {
	s := fmt.Sprintf("* %s [%s](%s)", lineIcon(r), r.Item.Name, r.Item.URL)
	if r.Item.Description != "" {
		s += " - " + r.Item.Description
	}
	return s
}

func activeLine(r curate.Resolved) string {
	return link(r)
}

func archivedLine(r curate.Resolved) string {
	s := link(r)
	var reason string
	switch {
	case r.Status.Archived:
		reason = "archived"
	case !r.Status.LastActivity.IsZero():
		reason = "last activity " + r.Status.LastActivity.Format("2006-01-02")
	default:
		reason = "inactive"
	}
	return fmt.Sprintf("%s _(%s · %s)_", s, r.Section, reason)
}
