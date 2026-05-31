// Command import-readme bootstraps data.toml from the existing README.md.
// It is a one-off helper; data.toml is the source of truth afterwards.
package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/rubiojr/awesome-restic/tool/readme-gen/internal/catalog"
)

var (
	headingRE = regexp.MustCompile(`^##\s+(.*)$`)
	itemRE    = regexp.MustCompile(`^\*\s+\[([^\]]+)\]\(([^)]+)\)\s*(?:-\s*(.*))?$`)
)

func main() {
	in := "README.md"
	out := "data.toml"
	if len(os.Args) > 1 {
		in = os.Args[1]
	}
	if len(os.Args) > 2 {
		out = os.Args[2]
	}

	f, err := os.Open(in)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	c := &catalog.Catalog{
		Title: "Awesome Restic",
		Intro: "Awesome [Restic](https://restic.net) related projects.",
	}

	var cur *catalog.Section
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t")
		if m := headingRE.FindStringSubmatch(line); m != nil {
			c.Sections = append(c.Sections, catalog.Section{Name: strings.TrimSpace(m[1])})
			cur = &c.Sections[len(c.Sections)-1]
			continue
		}
		if m := itemRE.FindStringSubmatch(line); m != nil && cur != nil {
			cur.Items = append(cur.Items, catalog.Item{
				Name:        strings.TrimSpace(m[1]),
				URL:         strings.TrimSpace(m[2]),
				Description: strings.TrimSpace(m[3]),
			})
		}
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}

	if err := catalog.Save(out, c); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d sections)", out, len(c.Sections))
}
