# Contributing

`README.md` is **generated** — do not edit it directly. The source of truth is
[`data.toml`](data.toml) in the repository root. The generator lives in
[`tool/readme-gen`](tool/readme-gen).

## Adding or editing an entry

Add your project to the relevant `[[sections]]` block in `data.toml`:

```toml
[[sections.items]]
name = "My Project"
url = "https://github.com/me/my-project"
description = "Short description"
```

Open a pull request with just the `data.toml` change — the README is refreshed
automatically. Keep entries grouped under the right section heading.

### Optional per-item overrides

Mainly useful for non-repository links (websites, app stores) that can't be
checked automatically:

```toml
archived = true          # force the archived flag
updated = "2025-04-01"   # manual "last activity" date (YYYY-MM-DD)
skip_check = true        # never query an API for this entry
```

## How curation works

The README is curated automatically:

- Each entry shows a freshness dot based on the latest repository activity:
  🟢 < 2 months · 🟡 < 4 months · 🟠 < 8 months · 🔴 older · ⚪ unknown
  (non-repository links, or projects archived upstream).
- Projects archived upstream or inactive for over two years are moved to the
  **Archived / Inactive** section at the bottom.

Activity and archival status are read from the GitHub and GitLab APIs. A failed
or unknown check never moves a project — the tool falls back to the last known
status cached in `tool/readme-gen/.cache/status.json`.

## Regenerating the README locally

Run the generator from its directory:

```sh
cd tool/readme-gen

# Set a token to avoid GitHub's low unauthenticated rate limit.
export GITHUB_TOKEN=...

# Generate ../../README.md and refresh the status cache.
go run .

# Preview without writing:
go run . -dry-run

# Regenerate from the cache only, without hitting any API:
go run . -offline
```

Run the tests with `go test ./...` from `tool/readme-gen`.

A scheduled GitHub Actions workflow
([`.github/workflows/curate.yml`](.github/workflows/curate.yml)) runs the
generator weekly and opens a pull request when the README changes.
