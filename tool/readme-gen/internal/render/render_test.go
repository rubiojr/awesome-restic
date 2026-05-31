package render

import (
	"testing"
	"time"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/checker"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/curate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	c := &catalog.Catalog{
		Title: "Awesome Restic",
		Intro: "Awesome stuff.",
		Sections: []catalog.Section{
			{Name: "Apps"},
			{Name: "Wrappers"},
		},
	}
	la := time.Date(2021, 3, 4, 0, 0, 0, 0, time.UTC)
	resolved := []curate.Resolved{
		{Section: "Apps", Item: catalog.Item{Name: "Active", URL: "u1", Description: "d1"}, State: curate.StateActive, Freshness: curate.FreshGreen},
		{Section: "Apps", Item: catalog.Item{Name: "Stale", URL: "u2", Description: "d2"}, State: curate.StateActive, Freshness: curate.FreshRed},
		{Section: "Apps", Item: catalog.Item{Name: "Unknown", URL: "u5", Description: "d5"}, State: curate.StateActive, Freshness: curate.FreshUnknown},
		{Section: "Apps", Item: catalog.Item{Name: "Old", URL: "u3", Description: "d3"}, State: curate.StateArchived, Freshness: curate.FreshRed, Status: checker.Status{Known: true, LastActivity: la}},
		{Section: "Wrappers", Item: catalog.Item{Name: "Gone", URL: "u4"}, State: curate.StateArchived, Freshness: curate.FreshUnknown, Status: checker.Status{Known: true, Archived: true}},
	}

	out := Render(c, resolved)

	assert.Contains(t, out, "Do not edit README.md directly")
	assert.Contains(t, out, "# Awesome Restic")
	assert.Contains(t, out, "* 🟢 [Active](u1) - d1\n")
	assert.Contains(t, out, "* 🔴 [Stale](u2) - d2\n")
	assert.Contains(t, out, "* ⚪ [Unknown](u5) - d5\n")
	// Archived items live only in the archive section, not their origin.
	assert.NotContains(t, out, "[Old](u3) - d3\n## ")
	assert.Contains(t, out, "## Archived / Inactive")
	assert.Contains(t, out, "* 🔴 [Old](u3) - d3 _(Apps · last activity 2021-03-04)_")
	assert.Contains(t, out, "* 📦 [Gone](u4) _(Wrappers · archived)_")
	// Wrappers had only an archived item, so its heading is omitted.
	assert.NotContains(t, out, "## Wrappers")
}

func TestRenderNoArchive(t *testing.T) {
	c := &catalog.Catalog{Title: "T", Sections: []catalog.Section{{Name: "Apps"}}}
	resolved := []curate.Resolved{
		{Section: "Apps", Item: catalog.Item{Name: "A", URL: "u"}, State: curate.StateActive},
	}
	out := Render(c, resolved)
	require.NotContains(t, out, "## Archived / Inactive")
}

func TestRenderSanitizesUntrustedFields(t *testing.T) {
	c := &catalog.Catalog{Title: "T", Sections: []catalog.Section{{Name: "Apps"}}}
	resolved := []curate.Resolved{
		// Markdown/HTML metacharacters and a newline in name + description.
		{Section: "Apps", Item: catalog.Item{
			Name:        "Evil] (x) <b>",
			URL:         "https://github.com/a/b",
			Description: "line1\nline2 <img src=x>",
		}, State: curate.StateActive, Freshness: curate.FreshGreen},
		// Dangerous URL scheme must not become a link destination.
		{Section: "Apps", Item: catalog.Item{
			Name: "JS",
			URL:  "javascript:alert(1)",
		}, State: curate.StateActive, Freshness: curate.FreshGreen},
		// Parentheses in an otherwise valid URL get encoded; stray
		// whitespace is stripped.
		{Section: "Apps", Item: catalog.Item{
			Name: "Parens",
			URL:  "https://example.com/a (b)",
		}, State: curate.StateActive, Freshness: curate.FreshGreen},
	}

	out := Render(c, resolved)

	// Name brackets/angles escaped; no raw newline injected into the list.
	assert.Contains(t, out, `Evil\] (x) &lt;b&gt;`)
	assert.NotContains(t, out, "line1\nline2")
	assert.Contains(t, out, "line1 line2 &lt;img src=x&gt;")
	// javascript: URL is dropped; the entry renders as plain text, no link.
	assert.NotContains(t, out, "javascript:")
	assert.Contains(t, out, "* 🟢 JS\n")
	// Parens in the destination are percent-encoded (whitespace stripped).
	assert.Contains(t, out, "https://example.com/a%28b%29")
}

func TestSafeURL(t *testing.T) {
	cases := []struct {
		in  string
		out string
		ok  bool
	}{
		{"https://github.com/a/b", "https://github.com/a/b", true},
		{"http://x.test", "http://x.test", true},
		{"mailto:a@b.test", "mailto:a@b.test", true},
		{"u1", "u1", true}, // relative is allowed
		{"javascript:alert(1)", "", false},
		{"data:text/html,x", "", false},
		{"file:///etc/passwd", "", false},
		{"", "", false},
		{"  ", "", false},
	}
	for _, tt := range cases {
		got, ok := safeURL(tt.in)
		assert.Equalf(t, tt.ok, ok, "ok for %q", tt.in)
		if tt.ok {
			assert.Equalf(t, tt.out, got, "out for %q", tt.in)
		}
	}
}
