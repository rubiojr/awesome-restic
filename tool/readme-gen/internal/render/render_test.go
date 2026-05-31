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
