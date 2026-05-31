package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/foo/bar", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"archived": true, "pushed_at": "2020-01-02T03:04:05Z"}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.GitHubBase = srv.URL

	st := c.Check(context.Background(), "https://github.com/foo/bar")
	require.True(t, st.Known)
	assert.True(t, st.Archived)
	assert.Equal(t, "github", st.Source)
	assert.Equal(t, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC), st.LastActivity.UTC())
}

func TestCheckGitLab(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/World%2Fdeja-dup", r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"archived": false, "last_activity_at": "2024-06-07T08:09:10Z"}`))
	}))
	defer srv.Close()

	c := New("")
	c.GitLabBaseFor = func(host string) string { return srv.URL }

	st := c.Check(context.Background(), "https://gitlab.gnome.org/World/deja-dup")
	require.True(t, st.Known)
	assert.False(t, st.Archived)
	assert.Equal(t, "gitlab", st.Source)
	assert.Equal(t, time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC), st.LastActivity.UTC())
}

func TestCheckUnsupported(t *testing.T) {
	c := New("")
	st := c.Check(context.Background(), "https://relicabackup.com")
	assert.False(t, st.Known)
	assert.Equal(t, "unsupported", st.Source)
}

func TestCheckHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := New("")
	c.GitHubBase = srv.URL

	st := c.Check(context.Background(), "https://github.com/foo/bar")
	assert.False(t, st.Known)
	assert.Equal(t, "error", st.Source)
	assert.NotEmpty(t, st.Err)
}
