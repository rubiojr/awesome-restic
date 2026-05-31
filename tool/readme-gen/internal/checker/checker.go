package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Status is the result of checking a single item.
type Status struct {
	// Known is true only when activity/archival was confidently determined.
	// When false, callers must fall back to cached or manual data and must
	// never treat the item as inactive or archived.
	Known        bool      `json:"known"`
	Archived     bool      `json:"archived"`
	LastActivity time.Time `json:"last_activity"`
	Source       string    `json:"source"` // github, gitlab, manual, cache, unsupported, error
	Err          string    `json:"err,omitempty"`
}

// Client queries repository hosts for activity and archival status.
type Client struct {
	HTTP  *http.Client
	Token string // GitHub token, optional

	// GitHubBase overrides the GitHub API base URL (default
	// https://api.github.com). Used in tests.
	GitHubBase string
	// GitLabBaseFor returns the GitLab API base URL for a given host. When nil,
	// it defaults to https://<host>/api/v4.
	GitLabBaseFor func(host string) string
}

// New returns a Client with sensible defaults.
func New(token string) *Client {
	return &Client{
		HTTP:       &http.Client{Timeout: 20 * time.Second},
		Token:      token,
		GitHubBase: "https://api.github.com",
	}
}

// Check determines the status of a repository URL. Non-repository URLs return a
// status with Known=false and Source="unsupported".
func (c *Client) Check(ctx context.Context, raw string) Status {
	ref, ok := ParseRepo(raw)
	if !ok {
		return Status{Source: "unsupported"}
	}
	switch ref.Provider {
	case ProviderGitHub:
		return c.checkGitHub(ctx, ref)
	case ProviderGitLab:
		return c.checkGitLab(ctx, ref)
	default:
		return Status{Source: "unsupported"}
	}
}

func (c *Client) checkGitHub(ctx context.Context, ref RepoRef) Status {
	base := c.GitHubBase
	if base == "" {
		base = "https://api.github.com"
	}
	endpoint := fmt.Sprintf("%s/repos/%s", base, ref.Path)

	var body struct {
		Archived bool      `json:"archived"`
		PushedAt time.Time `json:"pushed_at"`
	}
	if err := c.getJSON(ctx, endpoint, true, &body); err != nil {
		return Status{Source: "error", Err: err.Error()}
	}
	return Status{
		Known:        true,
		Archived:     body.Archived,
		LastActivity: body.PushedAt,
		Source:       "github",
	}
}

func (c *Client) checkGitLab(ctx context.Context, ref RepoRef) Status {
	base := fmt.Sprintf("https://%s/api/v4", ref.Host)
	if c.GitLabBaseFor != nil {
		base = c.GitLabBaseFor(ref.Host)
	}
	endpoint := fmt.Sprintf("%s/projects/%s", base, url.PathEscape(ref.Path))

	var body struct {
		Archived       bool      `json:"archived"`
		LastActivityAt time.Time `json:"last_activity_at"`
	}
	if err := c.getJSON(ctx, endpoint, false, &body); err != nil {
		return Status{Source: "error", Err: err.Error()}
	}
	return Status{
		Known:        true,
		Archived:     body.Archived,
		LastActivity: body.LastActivityAt,
		Source:       "gitlab",
	}
}

func (c *Client) getJSON(ctx context.Context, endpoint string, github bool, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if github {
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s", resp.Status, snippet(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func snippet(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
