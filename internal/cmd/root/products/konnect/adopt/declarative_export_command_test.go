package adopt

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	logpkg "github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

func TestAdoptionExportCommandUsesSelectionAndChecksDestinationBeforeWrites(t *testing.T) {
	for _, mode := range []string{"dry-run", "adopt", "existing-file"} {
		t.Run(mode, func(t *testing.T) {
			current := aiAdoptionPayload(t, "models")
			sibling := aiAdoptionPayload(t, "models")
			sibling["id"] = "6078f559-063b-4f10-9098-9e6553b21d9b"
			sibling["name"] = "leave-alone"
			requests, writes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "/models") {
					if r.Method == http.MethodPut {
						writes++
						var request map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
						maps.Copy(current, request)
					} else {
						require.Equal(t, http.MethodGet, r.Method)
					}
					if strings.HasSuffix(r.URL.Path, "/"+adoptionChildID) {
						require.NoError(t, json.NewEncoder(w).Encode(current))
					} else {
						require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
							"data": []any{current, sibling},
							"meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
						}))
					}
				} else {
					require.Equal(t, http.MethodGet, r.Method)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"id": adoptionGatewayID, "name": "shared", "display_name": "Shared",
						"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
					}))
				}
			}))
			defer server.Close()
			sdk := &helpers.KonnectSDK{
				SDK:     kkSDK.New(kkSDK.WithServerURL(server.URL), kkSDK.WithClient(server.Client())),
				BaseURL: server.URL, HTTPClient: server.Client(),
			}
			command, err := NewAIGatewayCmd(verbs.Adopt, nil, nil, nil)
			require.NoError(t, err)
			require.NoError(t, adoptCommon.AddAdoptFlags(command))
			out := new(bytes.Buffer)
			ctx := context.WithValue(t.Context(), config.ConfigKey, stubConfig{pageSize: 100})
			ctx = context.WithValue(ctx, logpkg.LoggerKey, slog.Default())
			ctx = context.WithValue(ctx, iostreams.StreamsKey, &iostreams.IOStreams{Out: out, ErrOut: out})
			ctx = context.WithValue(ctx,
				helpers.SDKAPIFactoryKey,
				helpers.SDKAPIFactory(func(config.Hook,
					*slog.Logger) (helpers.SDKAPI,
					error,
				) {
					return sdk,
						nil
				}))
			command.SetContext(ctx)
			command.SilenceErrors = true
			command.SilenceUsage = true
			path := filepath.Join(t.TempDir(), "exported.yaml")
			args := []string{
				"models", "--name-pattern", "team-*", "--gateway-id", adoptionGatewayID, "--namespace", "team-a",
				"--declarative-output", path, "--parent-ref", "shared-gateway",
			}
			if mode == "dry-run" {
				args = append(args, "--dry-run")
			}
			if mode == "existing-file" {
				require.NoError(t, os.WriteFile(path, []byte("keep"), 0o600))
			}
			command.SetArgs(args)
			err = command.Execute()
			if mode == "existing-file" {
				require.ErrorContains(t, err, "create declarative output")
				require.Zero(t, requests)
				return
			}
			require.NoError(t, err)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Contains(t, string(data), "!ref shared-gateway#id")
			require.Contains(t, string(data), "team-child")
			require.NotContains(t, string(data), "leave-alone")
			require.NotContains(t, string(data), "ai_gateways:")
			if mode == "dry-run" {
				require.Zero(t, writes)
				require.Contains(t, string(data), "# DRY RUN:")
				require.Contains(t, out.String(), "Would adopt")
			} else {
				require.Equal(t, 1, writes)
				require.Contains(t, out.String(), "Adopted")
			}
		})
	}
}
