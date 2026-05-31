// Package render turns a catalog plus resolved statuses into the README
// markdown, applying the curation layout.
package render

import (
	"fmt"
	"net/url"
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
	name := mdText(r.Item.Name)
	var s string
	if dest, ok := safeURL(r.Item.URL); ok {
		s = fmt.Sprintf("* %s [%s](%s)", lineIcon(r), name, dest)
	} else {
		// Unsafe or empty URL: render the name as plain text rather than emit
		// a link with an untrusted destination.
		s = fmt.Sprintf("* %s %s", lineIcon(r), name)
	}
	if r.Item.Description != "" {
		s += " - " + mdText(r.Item.Description)
	}
	return s
}

// mdText sanitises an untrusted string for inclusion in a single Markdown list
// item. Names and descriptions are authored Markdown (inline code, links,
// emphasis), so that formatting is deliberately preserved; only control
// characters and line breaks are neutralised. Stripping line breaks is the
// meaningful defence: it stops a crafted field from breaking out of its list
// item into new lines, headings or code fences. (GitHub's renderer already
// sanitises raw HTML.)
func mdText(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', '\v', '\f':
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1 // drop other control characters
		}
		return r
	}, s)
}

// safeURL validates an untrusted URL for use as a Markdown link destination.
// It rejects dangerous schemes (e.g. javascript:, data:) and percent-encodes
// the few characters that would otherwise break the link syntax. The boolean
// is false when no safe link can be produced.
func safeURL(raw string) (string, bool) {
	s := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == ' ' {
			return -1 // strip control characters and whitespace
		}
		return r
	}, strings.TrimSpace(raw))
	if s == "" {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}
	if u.Scheme != "" {
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "mailto":
		default:
			return "", false
		}
	}
	// Keep the destination from terminating the Markdown link early.
	s = strings.NewReplacer("(", "%28", ")", "%29").Replace(s)
	return s, true
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
