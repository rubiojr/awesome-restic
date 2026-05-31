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

	// UpstreamRepo, when set, is the repository URL used for activity and
	// archival checks instead of URL. Useful when the displayed link points
	// somewhere other than the source repo (e.g. a website, a docs page, or a
	// tool that lives in a larger monorepo).
	UpstreamRepo string `toml:"upstream_repo,omitempty"`

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

// CheckURL is the URL used for activity/archival checks: UpstreamRepo when set,
// otherwise the displayed URL.
func (i Item) CheckURL() string {
	if i.UpstreamRepo != "" {
		return i.UpstreamRepo
	}
	return i.URL
}

// Load reads and parses a catalog from a TOML file located within root. The
// name is resolved relative to root, which prevents traversal outside the
// permitted directory tree.
func Load(root *os.Root, name string) (*Catalog, error) {
	b, err := root.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("loading catalog %q: %w", name, err)
	}
	var c Catalog
	if err := toml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parsing catalog %q: %w", name, err)
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
