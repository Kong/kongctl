package organization

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/log"
	configtest "github.com/kong/kongctl/test/config"
	"github.com/stretchr/testify/require"
)

func TestOrganizationPATsOutput(t *testing.T) {
	for _, format := range []string{"json", "yaml", "text"} {
		t.Run(format, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v3/organizations/me" {
					_, _ = fmt.Fprint(w, `{"id":"current-org","owner_id":"owner-id"}`)
					return
				}
				require.Equal(t, "/v3/organizations/current-org/personal-access-tokens", r.URL.Path)
				_, _ = fmt.Fprint(w, `{"data":[{"id":"token-id","name":"audit-token","user_id":"owner-id","state":"ACTIVE"}]}`)
			}))
			defer server.Close()
			cfg := &configtest.MockConfigHook{
				GetStringMock: func(key string) string {
					switch key {
					case common.BaseURLConfigPath:
						return server.URL
					case common.PATConfigPath:
						return "test-token"
					case cmdcommon.OutputConfigPath:
						return format
					default:
						return ""
					}
				},
				GetIntMock:     func(string) int { return 2 },
				GetBoolMock:    func(string) bool { return false },
				GetProfileMock: func() string { return "test" },
			}
			c := newGetOrganizationPATsCmd(verbs.Get, nil, nil)
			streams, _, out, _ := iostreams.NewTestIOStreams()
			ctx := context.WithValue(t.Context(), config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.New(slog.DiscardHandler))
			c.SetContext(ctx)
			c.SetArgs(nil)
			require.NoError(t, c.Execute())
			require.Contains(t, out.String(), "audit-token")
			require.Contains(t, out.String(), "owner-id")
			if format == "json" {
				require.Contains(t, out.String(), `"user_id"`)
			}
		})
	}
}

func TestFetchOrganizationPATs(t *testing.T) {
	for _, shape := range []string{"flat", "nested", "cursor"} {
		t.Run(shape, func(t *testing.T) {
			lookups, pages := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				require.Equal(t, http.MethodGet, r.Method)
				if r.URL.Path == "/v3/organizations/me" {
					lookups++
					_, _ = fmt.Fprint(w, `{"id":"current-org"}`)
					return
				}
				require.Equal(t, "/v3/organizations/current-org/personal-access-tokens", r.URL.Path)
				require.Equal(t, "2", r.URL.Query().Get("page[size]"))
				pages++
				if pages == 1 {
					require.Empty(t, r.URL.Query().Get("page[after]"))
					_, _ = fmt.Fprint(w, `{"data":[{"id":"token-1","name":"first","user_id":"user-1","state":"ACTIVE"}],`+
						paginationMeta(shape))
				} else {
					require.Equal(t, "cursor", r.URL.Query().Get("page[after]"))
					_, _ = fmt.Fprint(w, `{"data":[{"id":"token-2","name":"second","user_id":"user-2","state":"REVOKED"}],`+
						`"meta":{"page":{"next":null}}}`)
				}
			}))
			defer server.Close()
			tokens, err := fetchOrganizationPATs(t.Context(), server.Client(), server.URL,
				apiutil.NewStaticTokenSource("test-token"), 2)
			require.NoError(t, err)
			require.Len(t, tokens, 2)
			require.Equal(t, "token-2", tokens[1].ID)
			require.Equal(t, 2, pages)
			require.Equal(t, 1, lookups)
		})
	}
}

func TestFetchOrganizationPATsErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		lookup           bool
		status           int
	}{
		{name: "forbidden", status: 403, body: `{"message":"Forbidden"}`, want: "status 403"},
		{name: "lookup unauthorized", lookup: true, status: 401, body: `{}`, want: "status 401"},
		{name: "missing org ID", lookup: true, status: 200, body: `{}`, want: "has no ID"},
		{name: "malformed response", status: 200, body: `invalid`, want: "decode"},
		{
			name: "invalid cursor", status: 200, body: `{"meta":{"page":{"next":"/no-cursor?size=10"}}}`,
			want: "pagination",
		},
		{
			name: "repeated cursor", status: 200, body: `{"meta":{"page":{"next":"?page%5Bafter%5D=same"}}}`,
			want: "pagination",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v3/organizations/me" && !tc.lookup {
					_, _ = fmt.Fprint(w, `{"id":"current-org"}`)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			_, err := fetchOrganizationPATs(t.Context(), server.Client(), server.URL,
				apiutil.NewStaticTokenSource("test-token"), 10)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestOrganizationPATsCommand(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		c, err := NewOrganizationCmd(verb, nil, nil)
		require.NoError(t, err)
		pats, _, err := c.Find([]string{"pats"})
		require.NoError(t, err)
		require.Equal(t, "pats", pats.Name())
		require.NotNil(t, pats.RunE)
		require.Nil(t, pats.Flags().Lookup("org-id"))
		c.SetArgs([]string{"pats", "--org-id", "other-org"})
		require.ErrorContains(t, c.Execute(), "unknown flag: --org-id")
		require.Error(t, pats.Args(pats, []string{"unexpected"}))
	}
}

func paginationMeta(shape string) string {
	if shape == "cursor" {
		return `"meta":{"next":"cursor"}}`
	}
	next := `"next":"/v3/organizations/current-org/personal-access-tokens?page%5Bafter%5D=cursor"`
	if shape == "nested" {
		return `"meta":{"page":{` + next + `}}}`
	}
	return `"meta":{` + next + `}}`
}
