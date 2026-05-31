package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// maxResponseBytes caps how much of a host response we will read. The status
// payloads are tiny; the limit defends against a hostile or compromised host
// streaming an unbounded body to exhaust memory.
const maxResponseBytes = 1 << 20 // 1 MiB

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

// New returns a Client with sensible defaults. The HTTP client refuses to
// connect to non-public IP addresses, mitigating SSRF from a hostile entry
// (e.g. a self-hosted "gitlab.*" host pointing at an internal service or the
// cloud metadata endpoint).
func New(token string) *Client {
	return &Client{
		HTTP:       safeHTTPClient(20 * time.Second),
		Token:      token,
		GitHubBase: "https://api.github.com",
	}
}

// safeHTTPClient builds an HTTP client whose dialer rejects connections to
// loopback, private, link-local, multicast and unspecified addresses.
func safeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   ssrfGuard,
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// ssrfGuard is a net.Dialer Control function. It runs after DNS resolution
// with the concrete IP being dialed, so it also defeats DNS rebinding to an
// internal address.
func ssrfGuard(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("ssrf guard: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("ssrf guard: unparseable address %q", address)
	}
	if !isPublicIP(ip) {
		return fmt.Errorf("ssrf guard: refusing to connect to non-public address %s", ip)
	}
	return nil
}

// isPublicIP reports whether ip is a globally routable unicast address.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	return true
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

	body := io.LimitReader(resp.Body, maxResponseBytes)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s", resp.Status, snippet(b))
	}
	return json.NewDecoder(body).Decode(out)
}

func snippet(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
