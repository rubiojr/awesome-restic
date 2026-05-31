package curate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// LoadCache reads a cache file located within root, resolved relative to root
// so it can never escape the permitted directory tree. An empty name disables
// the cache; a missing file yields an empty cache.
func LoadCache(root *os.Root, name string) (*Cache, error) {
	c := &Cache{Entries: map[string]CacheEntry{}}
	if name == "" {
		return c, nil
	}
	b, err := root.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading cache %q: %w", name, err)
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parsing cache %q: %w", name, err)
	}
	if c.Entries == nil {
		c.Entries = map[string]CacheEntry{}
	}
	return c, nil
}

// Save writes the cache within root with stable, indented JSON. The name is
// resolved relative to root, confining writes to the permitted tree.
func (c *Cache) Save(root *os.Root, name string) error {
	if name == "" {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if dir := filepath.Dir(name); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating cache dir %q: %w", dir, err)
		}
	}
	if err := root.WriteFile(name, b, 0o644); err != nil {
		return fmt.Errorf("writing cache %q: %w", name, err)
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
