package curate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/checker"
)

// CacheEntry is a persisted check result for a single URL.
type CacheEntry struct {
	Status    checker.Status `json:"status"`
	CheckedAt time.Time      `json:"checked_at"`
}

// Cache is a durable map of URL to last known status. It lets generation fall
// back to previously known data when an API call fails, so a transient outage
// never produces destructive README changes.
type Cache struct {
	Entries map[string]CacheEntry `json:"entries"`
}

// LoadCache reads a cache file. A missing file yields an empty cache.
func LoadCache(path string) (*Cache, error) {
	c := &Cache{Entries: map[string]CacheEntry{}}
	if path == "" {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading cache %q: %w", path, err)
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parsing cache %q: %w", path, err)
	}
	if c.Entries == nil {
		c.Entries = map[string]CacheEntry{}
	}
	return c, nil
}

// Save writes the cache to a file with stable, indented JSON.
func (c *Cache) Save(path string) error {
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("writing cache %q: %w", path, err)
	}
	return nil
}

// Get returns a cached entry for a URL.
func (c *Cache) Get(u string) (CacheEntry, bool) {
	e, ok := c.Entries[u]
	return e, ok
}

// Put stores a status for a URL with the given check time.
func (c *Cache) Put(u string, st checker.Status, now time.Time) {
	c.Entries[u] = CacheEntry{Status: st, CheckedAt: now}
}
