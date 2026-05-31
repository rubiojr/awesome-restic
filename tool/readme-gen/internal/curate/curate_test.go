package curate

import (
	"context"
	"testing"
	"time"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/checker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var refNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestClassify(t *testing.T) {
	opts := Options{Now: refNow}.withDefaults()

	tests := []struct {
		name  string
		st    checker.Status
		state State
	}{
		{"unknown stays active", checker.Status{Known: false}, StateActive},
		{"archived flag moves", checker.Status{Known: true, Archived: true, LastActivity: refNow}, StateArchived},
		{"recent active", checker.Status{Known: true, LastActivity: refNow.AddDate(0, -3, 0)}, StateActive},
		{"one year stays active", checker.Status{Known: true, LastActivity: refNow.AddDate(-1, 0, -1)}, StateActive},
		{"two years archived", checker.Status{Known: true, LastActivity: refNow.AddDate(-2, 0, -1)}, StateArchived},
		{"known but no date stays active", checker.Status{Known: true}, StateActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.state, classify(tt.st, opts))
		})
	}
}

func TestFreshness(t *testing.T) {
	opts := Options{Now: refNow}.withDefaults()

	tests := []struct {
		name string
		st   checker.Status
		want Freshness
	}{
		{"unknown when not known", checker.Status{Known: false}, FreshUnknown},
		{"unknown when no date", checker.Status{Known: true}, FreshUnknown},
		{"archived upstream uses activity date", checker.Status{Known: true, Archived: true, LastActivity: refNow.AddDate(0, -1, 0)}, FreshGreen},
		{"green within 2mo", checker.Status{Known: true, LastActivity: refNow.AddDate(0, -1, 0)}, FreshGreen},
		{"yellow within 4mo", checker.Status{Known: true, LastActivity: refNow.AddDate(0, -3, 0)}, FreshYellow},
		{"orange within 8mo", checker.Status{Known: true, LastActivity: refNow.AddDate(0, -6, 0)}, FreshOrange},
		{"red beyond 8mo", checker.Status{Known: true, LastActivity: refNow.AddDate(0, -10, 0)}, FreshRed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, freshness(tt.st, opts))
		})
	}
}

func TestApplyManualOverrides(t *testing.T) {
	archived := true
	st := applyManual(checker.Status{Source: "unsupported"}, catalog.Item{
		Updated:  "2019-05-01",
		Archived: &archived,
	})
	require.True(t, st.Known)
	assert.True(t, st.Archived)
	assert.Equal(t, "manual", st.Source)
	assert.Equal(t, time.Date(2019, 5, 1, 0, 0, 0, 0, time.UTC), st.LastActivity)
}

func TestResolveFallsBackToCacheOnFailure(t *testing.T) {
	// Seed cache with a known-good status.
	cache := &Cache{Entries: map[string]CacheEntry{}}
	good := checker.Status{Known: true, LastActivity: refNow.AddDate(0, -1, 0), Source: "github"}
	cache.Put("https://github.com/foo/bar", good, refNow)

	// Client returns an error (unknown) for this run.
	client := checker.New("")
	client.GitHubBase = "http://127.0.0.1:0" // guaranteed connection failure

	r := &Resolver{Client: client, Cache: cache}
	c := &catalog.Catalog{Sections: []catalog.Section{{
		Name:  "Apps",
		Items: []catalog.Item{{Name: "Bar", URL: "https://github.com/foo/bar"}},
	}}}

	res := r.Resolve(context.Background(), c, Options{Now: refNow})
	require.Len(t, res, 1)
	assert.Equal(t, "cache", res[0].Status.Source)
	assert.True(t, res[0].Status.Known)
	assert.Equal(t, StateActive, res[0].State)
}

func TestResolveUsesUpstreamRepo(t *testing.T) {
	// Cache is keyed by the upstream repo URL, not the displayed URL.
	cache := &Cache{Entries: map[string]CacheEntry{}}
	good := checker.Status{Known: true, LastActivity: refNow.AddDate(0, -1, 0), Source: "github"}
	cache.Put("https://github.com/org/real-repo", good, refNow)

	r := &Resolver{Cache: cache}
	c := &catalog.Catalog{Sections: []catalog.Section{{
		Name: "Apps",
		Items: []catalog.Item{{
			Name:         "Tool",
			URL:          "https://example.com/tool",
			UpstreamRepo: "https://github.com/org/real-repo",
		}},
	}}}

	res := r.Resolve(context.Background(), c, Options{Now: refNow, Offline: true})
	require.Len(t, res, 1)
	assert.True(t, res[0].Status.Known)
	assert.Equal(t, FreshGreen, res[0].Freshness)
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cache.json"

	c := &Cache{Entries: map[string]CacheEntry{}}
	c.Put("https://github.com/a/b", checker.Status{Known: true, Source: "github"}, refNow)
	require.NoError(t, c.Save(path))

	loaded, err := LoadCache(path)
	require.NoError(t, err)
	e, ok := loaded.Get("https://github.com/a/b")
	require.True(t, ok)
	assert.Equal(t, "github", e.Status.Source)
}

func TestLoadCacheMissing(t *testing.T) {
	c, err := LoadCache(t.TempDir() + "/nope.json")
	require.NoError(t, err)
	assert.Empty(t, c.Entries)
}
