package checker

import (
	"net/url"
	"regexp"
	"strings"
)

// repoSegment is the set of characters allowed in a single path segment
// (owner, repo or namespace component). Restricting this prevents an entry
// from smuggling path traversal (e.g. "..") or other characters into the
// upstream API request URL.
var repoSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validSegment reports whether s is a safe single path segment.
func validSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return repoSegment.MatchString(s)
}

// Provider identifies the hosting service for a repository URL.
type Provider int

const (
	// ProviderUnknown means the URL is not a recognised repository host.
	ProviderUnknown Provider = iota
	// ProviderGitHub is github.com.
	ProviderGitHub
	// ProviderGitLab is gitlab.com or a self-hosted GitLab instance.
	ProviderGitLab
)

// RepoRef is a normalised reference to a repository on a known host.
type RepoRef struct {
	Provider Provider
	Host     string // host of the instance, e.g. gitlab.gnome.org
	Path     string // namespace/project, e.g. owner/repo
}

// ParseRepo extracts a repository reference from an arbitrary URL. The second
// return value is false when the URL does not point at a supported repository
// host (e.g. an App Store link or a plain website), in which case no API check
// should be attempted.
func ParseRepo(raw string) (RepoRef, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return RepoRef{}, false
	}

	// Only HTTP(S) URLs can be repository links; reject javascript:, file:,
	// data: and any other scheme outright.
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return RepoRef{}, false
	}

	host := strings.ToLower(u.Host)
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return RepoRef{}, false
	}

	switch {
	case host == "github.com" || host == "www.github.com":
		return parseGitHub(path)
	case host == "gitlab.com" || strings.HasPrefix(host, "gitlab."):
		return parseGitLab(host, path)
	default:
		return RepoRef{}, false
	}
}

func parseGitHub(path string) (RepoRef, bool) {
	// Keep only owner/repo, dropping /tree, /blob, /releases, .git, etc.
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return RepoRef{}, false
	}
	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	if !validSegment(owner) || !validSegment(repo) {
		return RepoRef{}, false
	}
	return RepoRef{Provider: ProviderGitHub, Host: "github.com", Path: owner + "/" + repo}, true
}

func parseGitLab(host, path string) (RepoRef, bool) {
	// GitLab supports nested groups, so the project path is everything before
	// the "/-/" separator that prefixes blob/tree/issues/etc.
	if i := strings.Index(path, "/-/"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if !strings.Contains(path, "/") {
		// A bare group with no project is not a repository.
		return RepoRef{}, false
	}
	// Validate every namespace/project segment so nothing but a clean path is
	// ever escaped into the upstream API URL.
	for _, seg := range strings.Split(path, "/") {
		if !validSegment(seg) {
			return RepoRef{}, false
		}
	}
	return RepoRef{Provider: ProviderGitLab, Host: host, Path: path}, true
}
