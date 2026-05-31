package main

import (
	"testing"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/checker"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/curate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnresolvedRepoEntries(t *testing.T) {
	resolved := []curate.Resolved{
		// Repo, known: not a warning.
		{Item: catalog.Item{Name: "ok", URL: "https://github.com/a/b"}, Status: checker.Status{Known: true}},
		// Repo, unknown: warning.
		{Item: catalog.Item{Name: "broken", URL: "https://github.com/c/d"}, Status: checker.Status{Known: false}},
		// upstream_repo unknown: warning, reported by its check URL.
		{Item: catalog.Item{Name: "viaUpstream", URL: "https://site.example", UpstreamRepo: "https://github.com/e/f"}, Status: checker.Status{Known: false}},
		// Non-repo link, unknown: expected, not a warning.
		{Item: catalog.Item{Name: "website", URL: "https://example.com"}, Status: checker.Status{Known: false}},
		// SkipCheck: never a warning.
		{Item: catalog.Item{Name: "skipped", URL: "https://github.com/g/h", SkipCheck: true}, Status: checker.Status{Known: false}},
	}

	warns := unresolvedRepoEntries(resolved)
	require.Len(t, warns, 2)
	assert.Equal(t, "broken", warns[0].Item.Name)
	assert.Equal(t, "viaUpstream", warns[1].Item.Name)
	assert.Equal(t, "https://github.com/e/f", warns[1].Item.CheckURL())
}
