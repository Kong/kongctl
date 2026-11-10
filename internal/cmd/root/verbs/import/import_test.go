package importcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type orgChartImportSDK struct {
	helpers.SDKAPI
	directories helpers.IdentityDirectoriesAPI
	importer    helpers.IdentityOrgChartImportAPI
}

func (s orgChartImportSDK) GetIdentityDirectoriesAPI() helpers.IdentityDirectoriesAPI {
	return s.directories
}

func (s orgChartImportSDK) GetIdentityOrgChartImportAPI() helpers.IdentityOrgChartImportAPI {
	return s.importer
}

type orgChartImportAPI struct {
	directory string
	request   kkComps.ImportPrincipalsRequest
	calls     int
}

func (a *orgChartImportAPI) ImportOrgChart(_ context.Context, directory string,
	request kkComps.ImportPrincipalsRequest, _ ...kkOps.Option,
) (*kkOps.ImportOrgChartResponse, error) {
	a.directory, a.request = directory, request
	a.calls++
	const report = `{"counts":{"users":{"created":1,"updated":0,"unchanged":0,"inactive":0},
		"teams":{"created":1,"deleted":0},"memberships":{"added":0,"removed":0}},"warnings":[],"errors":[]}`
	return &kkOps.ImportOrgChartResponse{
		StatusCode:  http.StatusOK,
		RawResponse: &http.Response{Body: io.NopCloser(bytes.NewBufferString(report))},
	}, nil
}

func TestImportOrgChartRoutesPreserveScopeIntentAndReport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   []string
		dryRun bool
	}{
		{"implicit alias", []string{"id", "org-chart"}, true},
		{"implicit full name", []string{"identity", "org-charts"}, false},
		{"explicit Konnect", []string{"konnect", "identity", "org-chart"}, true},
		{"explicit aliases", []string{"k", "id", "org-charts"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, err := NewImportCmd()
			require.NoError(t, err)
			command.SilenceErrors, command.SilenceUsage = true, true
			leaf, remaining, err := command.Find(tc.path)
			require.NoError(t, err)
			require.Empty(t, remaining)
			require.Equal(t, "org-chart", leaf.Name())
			directory := fixture.Directory("default", "", false)
			api := &orgChartImportAPI{}
			sdk := orgChartImportSDK{
				directories: &fixture.DirectoryAPI{Current: directory, Directories: []kkComps.KongDirectory{directory}},
				importer:    api,
			}
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(cmdCommon.OutputConfigPath, "json")
			streams, _, output, _ := iostreams.NewTestIOStreams()
			ctx := context.WithValue(t.Context(), config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
			ctx = context.WithValue(ctx, logctx.LoggerKey, slog.New(slog.DiscardHandler))
			preRun := leaf.PreRunE
			leaf.PreRunE = func(cmd *cobra.Command, args []string) error {
				if preRun != nil {
					if err := preRun(cmd, args); err != nil {
						return err
					}
				}
				require.Equal(t, verbs.Import, cmd.Context().Value(verbs.Verb))
				require.Equal(t, products.ProductKonnect, cmd.Context().Value(products.Product))
				require.Equal(t, "https://us.api.konghq.tech", cfg.GetString(common.BaseURLConfigPath))
				require.Equal(t, "us", cfg.GetString(common.RegionConfigPath))
				require.Equal(t, "fixture", cfg.GetString(common.PATConfigPath))
				cmd.SetContext(context.WithValue(cmd.Context(), helpers.SDKAPIFactoryKey,
					helpers.SDKAPIFactory(func(config.Hook, *slog.Logger) (helpers.SDKAPI, error) { return sdk, nil })))
				return nil
			}
			const csv = "external_id,email,given_name,family_name,manager_external_id\none,person@example.test,First,Last,\n"
			source := filepath.Join(t.TempDir(), "roster.csv")
			report := filepath.Join(t.TempDir(), "reports")
			require.NoError(t, os.WriteFile(source, []byte(csv), 0o600))
			mode := "--execute"
			if tc.dryRun {
				mode = "--dry-run"
			}
			command.SetArgs(append(tc.path, "--file", source, "--report-output-dir", report, mode,
				"--base-url", "https://us.api.konghq.tech", "--region", "us", "--pat", "fixture"))
			require.NoError(t, command.ExecuteContext(ctx))
			require.Equal(t, 1, api.calls)
			require.Equal(t, directory.ID, api.directory)
			require.Equal(t, []byte(csv), api.request.File.Content)
			require.Equal(t, new(tc.dryRun), api.request.DryRun)
			var summary helpers.IdentityOrgChartImportSummary
			require.NoError(t, json.Unmarshal(output.Bytes(), &summary))
			require.Equal(t, tc.dryRun, summary.DryRun)
			require.Equal(t, directory.ID, summary.DirectoryID)
			require.Equal(t, 1, summary.Rows)
			require.Zero(t, summary.ErrorCount)
			require.Equal(t, report, filepath.Dir(summary.ReportOutputFile))
			if tc.dryRun {
				require.Contains(t, filepath.Base(summary.ReportOutputFile), "org-chart-preview-")
			} else {
				require.Contains(t, filepath.Base(summary.ReportOutputFile), "org-chart-execute-")
			}
			report = summary.ReportOutputFile
			info, err := os.Stat(report)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			contents, err := os.ReadFile(report)
			require.NoError(t, err)
			var capture struct {
				DryRun      bool   `json:"dry_run"`
				DirectoryID string `json:"directory_id"`
			}
			require.NoError(t, json.Unmarshal(contents, &capture))
			require.Equal(t, tc.dryRun, capture.DryRun)
			require.Equal(t, directory.ID, capture.DirectoryID)
		})
	}
}

func TestImportCommandGroupsRequireSubcommands(t *testing.T) {
	for _, path := range [][]string{nil, {"id"}, {"konnect"}, {"konnect", "identity"}} {
		command, err := NewImportCmd()
		require.NoError(t, err)
		leaf, remaining, err := command.Find(path)
		require.NoError(t, err)
		require.Empty(t, remaining)
		require.Error(t, leaf.RunE(leaf, nil))
	}
}
