package loader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/util/httpheaders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NotEmpty(t, r.Header.Get(httpheaders.HeaderUserAgent))
			assert.Empty(t, r.Header.Get(httpheaders.HeaderAuthorization))
			w.Header().Set(httpheaders.HeaderContentType, "application/yaml")
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		body, err := FetchURL(t.Context(), server.URL+"/config.yaml")
		require.NoError(t, err)
		assert.Equal(t, "portals: []\n", string(body))
	})

	t.Run("retries retryable status", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempt := attempts.Add(1)
			if attempt < 3 {
				http.Error(w, "try again", http.StatusInternalServerError)
				return
			}
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		body, err := fetchURL(t.Context(), server.URL, urlFetchConfig{backoff: time.Millisecond})
		require.NoError(t, err)
		assert.Equal(t, "portals: []\n", string(body))
		assert.Equal(t, int32(3), attempts.Load())
	})

	t.Run("does not retry non-retryable status", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			http.NotFound(w, nil)
		}))
		defer server.Close()

		_, err := fetchURL(t.Context(), server.URL, urlFetchConfig{backoff: time.Millisecond})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
		assert.Equal(t, int32(1), attempts.Load())
	})

	t.Run("rejects response over content length limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "4")
			_, err := w.Write([]byte("data"))
			require.NoError(t, err)
		}))
		defer server.Close()

		_, err := fetchURL(t.Context(), server.URL, urlFetchConfig{maxBytes: 3})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "response is too large")
	})

	t.Run("rejects response over streaming limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			_, err := w.Write([]byte("data"))
			require.NoError(t, err)
		}))
		defer server.Close()

		_, err := fetchURL(t.Context(), server.URL, urlFetchConfig{maxBytes: 3})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "response is too large")
	})

	t.Run("rejects redirect loops", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/loop", http.StatusFound)
		}))
		defer server.Close()

		_, err := fetchURL(t.Context(), server.URL+"/loop", urlFetchConfig{maxRedirects: 2})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stopped after 2 redirects")
	})

	t.Run("rejects https to http redirects", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer target.Close()

		source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}))
		defer source.Close()

		_, err := fetchURL(t.Context(), source.URL, urlFetchConfig{client: source.Client()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refusing to follow HTTPS to HTTP redirect")
	})

	t.Run("honors canceled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := FetchURL(ctx, server.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), context.Canceled.Error())
	})
}

func TestFetchURLAuth(t *testing.T) {
	t.Run("sends bearer token to allowed HTTPS host", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get(httpheaders.HeaderAuthorization))
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		body, err := fetchURL(t.Context(), server.URL, urlFetchConfig{
			client: server.Client(),
			options: URLFetchOptions{
				AuthPolicy:       URLFetchAuthAuto,
				AuthAllowedHosts: []string{mustURLHostname(t, server.URL)},
				TokenSource:      staticURLFetchTokenSource{token: "test-token"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "portals: []\n", string(body))
	})

	t.Run("does not send bearer token to non-allowed HTTPS host", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get(httpheaders.HeaderAuthorization))
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		_, err := fetchURL(t.Context(), server.URL, urlFetchConfig{
			client: server.Client(),
			options: URLFetchOptions{
				AuthPolicy:       URLFetchAuthAuto,
				AuthAllowedHosts: []string{"example.com"},
				TokenSource:      staticURLFetchTokenSource{token: "test-token"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("does not send bearer token over HTTP", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get(httpheaders.HeaderAuthorization))
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		_, err := fetchURL(t.Context(), server.URL, urlFetchConfig{
			options: URLFetchOptions{
				AuthPolicy:       URLFetchAuthAuto,
				AuthAllowedHosts: []string{mustURLHostname(t, server.URL)},
				TokenSource:      staticURLFetchTokenSource{token: "test-token"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("strips bearer token on redirect outside allowed hosts", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/config.yaml", nil)
		require.NoError(t, err)
		req.Header.Set(httpheaders.HeaderAuthorization, "Bearer test-token")

		via, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"https://us.cloud.konghq.com/config.yaml",
			nil,
		)
		require.NoError(t, err)

		err = checkURLFetchRedirect(10, URLFetchOptions{
			AuthPolicy:       URLFetchAuthAuto,
			AuthAllowedHosts: []string{"cloud.konghq.com"},
			TokenSource:      staticURLFetchTokenSource{token: "test-token"},
		})(req, []*http.Request{via})
		require.NoError(t, err)
		assert.Empty(t, req.Header.Get(httpheaders.HeaderAuthorization))
	})
}

func TestLoader_LoadFromSources_URL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`
portals:
  - ref: remote-portal
    name: Remote Portal
`))
		require.NoError(t, err)
	}))
	defer server.Close()

	rs, err := New().LoadFromSourcesWithContext(t.Context(), []Source{
		{Path: server.URL + "/config", Type: SourceTypeURL},
	}, false)
	require.NoError(t, err)
	require.Len(t, rs.Portals, 1)
	assert.Equal(t, "remote-portal", rs.Portals[0].Ref)
}

type staticURLFetchTokenSource struct {
	token string
}

func (s staticURLFetchTokenSource) Token(context.Context) (string, error) {
	return s.token, nil
}

func mustURLHostname(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	return parsed.Hostname()
}

func TestLoader_LoadFromSources_URLUsesBaseDirForFileTags(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "description.txt"), []byte("loaded from base dir"), 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`
apis:
  - ref: remote-api
    name: Remote API
    description: !file description.txt
`))
		require.NoError(t, err)
	}))
	defer server.Close()

	rs, err := NewWithBaseDir(dir).LoadFromSourcesWithContext(t.Context(), []Source{
		{Path: server.URL + "/config.yaml", Type: SourceTypeURL},
	}, false)
	require.NoError(t, err)
	require.Len(t, rs.APIs, 1)
	require.NotNil(t, rs.APIs[0].Description)
	assert.Equal(t, "loaded from base dir", *rs.APIs[0].Description)
}

func TestFetchURLRejectsInvalidURL(t *testing.T) {
	tests := []string{
		"file:///tmp/config.yaml",
		"https://",
		"://bad",
	}

	for _, rawURL := range tests {
		t.Run(strings.ReplaceAll(rawURL, "/", "_"), func(t *testing.T) {
			_, err := FetchURL(t.Context(), rawURL)
			require.Error(t, err)
			assert.Contains(t, fmt.Sprint(err), "URL")
		})
	}
}
