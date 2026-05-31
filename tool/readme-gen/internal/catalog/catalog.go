// Package catalog defines the TOML source-of-truth model for the awesome-restic
// list and helpers to load and save it.
package catalog

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Catalog is the full source of truth for the generated README.
type Catalog struct {
	Title    string    `toml:"title"`
	Intro    string    `toml:"intro"`
	Sections []Section `toml:"sections"`
}

// Section is a named group of items, rendered as a README heading.
type Section struct {
	Name  string `toml:"name"`
	Items []Item `toml:"items"`
}

// Item is a single list entry.
type Item struct {
	Name        string `toml:"name"`
	URL         string `toml:"url"`
	Description string `toml:"description,omitempty"`

	// Optional manual overrides, mainly for non-repo entries (websites, app
	// stores) or to pin a status regardless of API results.
	//
	// Archived, when set, forces the archived flag instead of querying an API.
	Archived *bool `toml:"archived,omitempty"`
	// Updated is a manual "last activity" date in YYYY-MM-DD form.
	Updated string `toml:"updated,omitempty"`
	// SkipCheck disables any API lookup for this item.
	SkipCheck bool `toml:"skip_check,omitempty"`
}

// Load reads and parses a catalog from a TOML file.
func Load(path string) (*Catalog, error) {
	var c Catalog
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("loading catalog %q: %w", path, err)
	}
	return &c, nil
}

// Save writes the catalog to a TOML file.
func Save(path string, c *Catalog) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating catalog %q: %w", path, err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	enc.Indent = "  "
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encoding catalog %q: %w", path, err)
	}
	return nil
}
