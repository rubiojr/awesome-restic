package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		ok       bool
		provider Provider
		host     string
		path     string
	}{
		{"github simple", "https://github.com/Mebus/restatic", true, ProviderGitHub, "github.com", "Mebus/restatic"},
		{"github trailing slash", "https://github.com/drdo/redu/", true, ProviderGitHub, "github.com", "drdo/redu"},
		{"github blob path", "https://github.com/foo/bar/blob/main/README.md", true, ProviderGitHub, "github.com", "foo/bar"},
		{"github tree path", "https://github.com/foo/bar/tree/main", true, ProviderGitHub, "github.com", "foo/bar"},
		{"github dot git", "https://github.com/foo/bar.git", true, ProviderGitHub, "github.com", "foo/bar"},
		{"github www", "https://www.github.com/foo/bar", true, ProviderGitHub, "github.com", "foo/bar"},
		{"github owner only", "https://github.com/foo", false, ProviderUnknown, "", ""},
		{"gitlab.com nested", "https://gitlab.com/stormking/resticguigx/-/blob/master/README.md", true, ProviderGitLab, "gitlab.com", "stormking/resticguigx"},
		{"gitlab self hosted", "https://gitlab.gnome.org/World/deja-dup", true, ProviderGitLab, "gitlab.gnome.org", "World/deja-dup"},
		{"gitlab deep namespace", "https://gitlab.com/a/b/c/-/tree/main", true, ProviderGitLab, "gitlab.com", "a/b/c"},
		{"gitlab group only", "https://gitlab.com/World", false, ProviderUnknown, "", ""},
		{"app store", "https://apps.apple.com/app/restique/id6744624567", false, ProviderUnknown, "", ""},
		{"plain website", "https://relicabackup.com", false, ProviderUnknown, "", ""},
		{"rclone docs", "https://rclone.org/commands/rclone_serve_restic/", false, ProviderUnknown, "", ""},
		{"empty", "", false, ProviderUnknown, "", ""},
		// Security: dangerous schemes are rejected outright.
		{"javascript scheme", "javascript:alert(1)//github.com/a/b", false, ProviderUnknown, "", ""},
		{"file scheme", "file:///etc/passwd", false, ProviderUnknown, "", ""},
		// Security: path traversal in owner/repo is rejected, not forwarded.
		{"github dotdot owner", "https://github.com/../../search", false, ProviderUnknown, "", ""},
		{"github dotdot repo", "https://github.com/foo/..", false, ProviderUnknown, "", ""},
		// Security: look-alike GitLab hostnames are not treated as GitLab.
		{"evil gitlab suffix", "https://evil-gitlab.com/a/b", false, ProviderUnknown, "", ""},
		{"my gitlab", "https://mygitlab.com/a/b", false, ProviderUnknown, "", ""},
		// Legitimate self-hosted GitLab (gitlab.* subdomain) still works.
		{"gitlab subdomain host", "https://gitlab.example.org/group/project", true, ProviderGitLab, "gitlab.example.org", "group/project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, ok := ParseRepo(tt.url)
			assert.Equal(t, tt.ok, ok)
			if !tt.ok {
				return
			}
			assert.Equal(t, tt.provider, ref.Provider)
			assert.Equal(t, tt.host, ref.Host)
			assert.Equal(t, tt.path, ref.Path)
		})
	}
}
