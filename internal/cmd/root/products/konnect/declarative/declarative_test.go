package declarative

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	utilviper "github.com/kong/kongctl/internal/util/viper"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDeclarativeConfig() *config.ProfiledConfig {
	return config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))
}

func testDeclarativeLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMaxConcurrencyFromCmd(t *testing.T) {
	t.Run("uses default value when flag is not set", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))

		got, err := maxConcurrencyFromCmd(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, executor.DefaultMaxConcurrency, got)
	})

	t.Run("uses config value when flag is not set", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))
		cfg.Set(maxConcurrencyConfigPath, 17)

		got, err := maxConcurrencyFromCmd(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, 17, got)
	})

	t.Run("accepts value within range", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)
		require.NoError(t, cmd.Flags().Set("max-concurrency", "42"))

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))

		got, err := maxConcurrencyFromCmd(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, 42, got)
	})

	t.Run("prefers flag value over config value", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)
		require.NoError(t, cmd.Flags().Set("max-concurrency", "42"))

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))
		cfg.Set(maxConcurrencyConfigPath, 17)

		got, err := maxConcurrencyFromCmd(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, 42, got)
	})

	t.Run("rejects value below minimum", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)
		require.NoError(t, cmd.Flags().Set("max-concurrency", "0"))

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))

		_, err := maxConcurrencyFromCmd(cmd, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--max-concurrency must be between")
	})

	t.Run("rejects value above maximum", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)
		require.NoError(t, cmd.Flags().Set("max-concurrency", "1000"))

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))

		_, err := maxConcurrencyFromCmd(cmd, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--max-concurrency must be between")
	})

	t.Run("rejects out-of-range value from config (below minimum)", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))
		cfg.Set(maxConcurrencyConfigPath, 0)

		_, err := maxConcurrencyFromCmd(cmd, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--max-concurrency must be between")
	})

	t.Run("rejects out-of-range value from config (above maximum)", func(t *testing.T) {
		cmd := &cobra.Command{}
		addMaxConcurrencyFlag(cmd)

		cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", utilviper.NewViper("nonexistent.yaml"))
		cfg.Set(maxConcurrencyConfigPath, 1000)

		_, err := maxConcurrencyFromCmd(cmd, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--max-concurrency must be between")
	})
}

func Test_validateDeletePlan(t *testing.T) {
	tests := []struct {
		name    string
		mode    planner.PlanMode
		wantErr bool
		errMsg  string
	}{
		{
			name:    "delete mode is accepted",
			mode:    planner.PlanModeDelete,
			wantErr: false,
		},
		{
			name:    "apply mode is rejected",
			mode:    planner.PlanModeApply,
			wantErr: true,
			errMsg:  `delete command requires a plan generated in delete mode, got "apply" mode`,
		},
		{
			name:    "sync mode is rejected",
			mode:    planner.PlanModeSync,
			wantErr: true,
			errMsg:  `delete command requires a plan generated in delete mode, got "sync" mode`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &planner.Plan{
				Metadata: planner.PlanMetadata{Mode: tt.mode},
			}
			err := validateDeletePlan(plan)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_parsePlanMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected planner.PlanMode
		errMsg   string
	}{
		{
			name:     "sync mode",
			mode:     "sync",
			expected: planner.PlanModeSync,
		},
		{
			name:     "apply mode",
			mode:     "apply",
			expected: planner.PlanModeApply,
		},
		{
			name:     "delete mode",
			mode:     "delete",
			expected: planner.PlanModeDelete,
		},
		{
			name:   "invalid mode",
			mode:   "invalid",
			errMsg: `invalid mode "invalid": must be 'sync', 'apply', or 'delete'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := parsePlanMode(tt.mode)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, mode)
		})
	}
}

func TestDeclarativeCommandsRequireExplicitFilename(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "plan", cmd: newDeclarativePlanCmd()},
		{name: "apply", cmd: newDeclarativeApplyCmd()},
		{name: "sync", cmd: newDeclarativeSyncCmd()},
		{name: "diff", cmd: newDeclarativeDiffCmd()},
		{name: "delete", cmd: newDeclarativeDeleteCmd()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.RunE(tt.cmd, nil)
			require.Error(t, err)
			assert.True(t, cmdpkg.IsUsageError(err))
			assert.Equal(
				t,
				"no configuration sources specified; use -f to specify files, directories, or URLs",
				err.Error(),
			)
		})
	}
}

func TestDeclarativeCommandsExposeRemoteSourceFlags(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "plan", cmd: newDeclarativePlanCmd()},
		{name: "apply", cmd: newDeclarativeApplyCmd()},
		{name: "sync", cmd: newDeclarativeSyncCmd()},
		{name: "diff", cmd: newDeclarativeDiffCmd()},
		{name: "delete", cmd: newDeclarativeDeleteCmd()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filenameFlag := tt.cmd.Flags().Lookup("filename")
			require.NotNil(t, filenameFlag)
			assert.Contains(t, filenameFlag.Usage, "URL")

			saveAsFlag := tt.cmd.Flags().Lookup(saveAsFlagName)
			require.NotNil(t, saveAsFlag)
			assert.Contains(t, saveAsFlag.Usage, "remote")

			remoteAuthFlag := tt.cmd.Flags().Lookup(remoteFileAuthFlagName)
			require.NotNil(t, remoteAuthFlag)
			assert.Contains(t, remoteAuthFlag.Usage, "auto|none")
		})
	}
}

func TestSourcesForCommand_SaveAs(t *testing.T) {
	t.Run("saves URL source and returns saved file source", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		cmd := newDeclarativeApplyCmd()
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)

		savePath := filepath.Join(t.TempDir(), "remote.yaml")
		require.NoError(t, cmd.Flags().Set(saveAsFlagName, savePath))

		sources, _, err := sourcesForCommand(
			cmd,
			"",
			[]string{server.URL + "/config.yaml"},
			testDeclarativeConfig(),
			testDeclarativeLogger(),
		)
		require.NoError(t, err)
		require.Len(t, sources, 1)
		assert.Equal(t, savePath, sources[0].Path)
		assert.Equal(t, loader.SourceTypeFile, sources[0].Type)

		content, err := os.ReadFile(savePath)
		require.NoError(t, err)
		assert.Equal(t, "portals: []\n", string(content))
		assert.Contains(t, stderr.String(), "Saved remote source to: "+savePath)
	})

	t.Run("rejects plan input", func(t *testing.T) {
		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(saveAsFlagName, filepath.Join(t.TempDir(), "remote.yaml")))

		_, _, err := sourcesForCommand(cmd, "plan.json", nil, testDeclarativeConfig(), testDeclarativeLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--save-as cannot be used with --plan")
	})

	t.Run("rejects non URL input", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("portals: []\n"), 0o600))

		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(saveAsFlagName, filepath.Join(dir, "remote.yaml")))

		_, _, err := sourcesForCommand(cmd, "", []string{configPath}, testDeclarativeConfig(), testDeclarativeLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--save-as requires exactly one URL source")
	})

	t.Run("rejects multiple sources", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("portals: []\n"))
			require.NoError(t, err)
		}))
		defer server.Close()

		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("portals: []\n"), 0o600))

		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(saveAsFlagName, filepath.Join(dir, "remote.yaml")))

		_, _, err := sourcesForCommand(
			cmd,
			"",
			[]string{server.URL, configPath},
			testDeclarativeConfig(),
			testDeclarativeLogger(),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--save-as requires exactly one URL source")
	})

	t.Run("rejects empty save path", func(t *testing.T) {
		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(saveAsFlagName, ""))

		_, _, err := sourcesForCommand(
			cmd,
			"",
			[]string{"https://example.com/config.yaml"},
			testDeclarativeConfig(),
			testDeclarativeLogger(),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--save-as cannot be empty")
	})
}

func TestResolveRemoteFileAuthPolicy(t *testing.T) {
	t.Run("defaults to auto", func(t *testing.T) {
		got, err := resolveRemoteFileAuthPolicy(newDeclarativeApplyCmd(), testDeclarativeConfig())
		require.NoError(t, err)
		assert.Equal(t, loader.URLFetchAuthAuto, got)
	})

	t.Run("uses config value", func(t *testing.T) {
		cfg := testDeclarativeConfig()
		cfg.Set(remoteFileAuthConfigPath, "none")

		got, err := resolveRemoteFileAuthPolicy(newDeclarativeApplyCmd(), cfg)
		require.NoError(t, err)
		assert.Equal(t, loader.URLFetchAuthNone, got)
	})

	t.Run("flag overrides config value", func(t *testing.T) {
		cfg := testDeclarativeConfig()
		cfg.Set(remoteFileAuthConfigPath, "none")
		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(remoteFileAuthFlagName, "auto"))

		got, err := resolveRemoteFileAuthPolicy(cmd, cfg)
		require.NoError(t, err)
		assert.Equal(t, loader.URLFetchAuthAuto, got)
	})

	t.Run("rejects invalid value", func(t *testing.T) {
		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(remoteFileAuthFlagName, "always"))

		_, err := resolveRemoteFileAuthPolicy(cmd, testDeclarativeConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--remote-file-auth must be one of")
	})
}

func TestRemoteFileFetchOptionsForSources(t *testing.T) {
	t.Run("enables token source for default Konnect cloud host", func(t *testing.T) {
		cfg := testDeclarativeConfig()
		cfg.Set(konnectcommon.PATConfigPath, "test-token")

		options, err := remoteFileFetchOptionsForSources(
			newDeclarativeApplyCmd(),
			cfg,
			testDeclarativeLogger(),
			[]loader.Source{{Path: "https://us.cloud.konghq.com/remote-file.yaml", Type: loader.SourceTypeURL}},
		)
		require.NoError(t, err)
		assert.Equal(t, loader.URLFetchAuthAuto, options.AuthPolicy)
		assert.True(t, options.AllowsAuthenticationForURL("https://us.cloud.konghq.com/remote-file.yaml"))
		assert.NotNil(t, options.TokenSource)
	})

	t.Run("does not enable token source for arbitrary host", func(t *testing.T) {
		cfg := testDeclarativeConfig()
		cfg.Set(konnectcommon.PATConfigPath, "test-token")

		options, err := remoteFileFetchOptionsForSources(
			newDeclarativeApplyCmd(),
			cfg,
			testDeclarativeLogger(),
			[]loader.Source{{Path: "https://example.com/config.yaml", Type: loader.SourceTypeURL}},
		)
		require.NoError(t, err)
		assert.False(t, options.AllowsAuthenticationForURL("https://example.com/config.yaml"))
		assert.Nil(t, options.TokenSource)
	})

	t.Run("honors remote-file-auth none", func(t *testing.T) {
		cfg := testDeclarativeConfig()
		cfg.Set(konnectcommon.PATConfigPath, "test-token")
		cmd := newDeclarativeApplyCmd()
		require.NoError(t, cmd.Flags().Set(remoteFileAuthFlagName, "none"))

		options, err := remoteFileFetchOptionsForSources(
			cmd,
			cfg,
			testDeclarativeLogger(),
			[]loader.Source{{Path: "https://us.cloud.konghq.com/remote-file.yaml", Type: loader.SourceTypeURL}},
		)
		require.NoError(t, err)
		assert.Equal(t, loader.URLFetchAuthNone, options.AuthPolicy)
		assert.False(t, options.AllowsAuthenticationForURL("https://us.cloud.konghq.com/remote-file.yaml"))
		assert.Nil(t, options.TokenSource)
	})

	t.Run("supports additional configured hosts", func(t *testing.T) {
		cfg := testDeclarativeConfig()
		cfg.Set(konnectcommon.PATConfigPath, "test-token")
		cfg.Set(remoteFileAuthHostsConfigPath, []string{"example.com"})

		options, err := remoteFileFetchOptionsForSources(
			newDeclarativeApplyCmd(),
			cfg,
			testDeclarativeLogger(),
			[]loader.Source{{Path: "https://example.com/config.yaml", Type: loader.SourceTypeURL}},
		)
		require.NoError(t, err)
		assert.True(t, options.AllowsAuthenticationForURL("https://example.com/config.yaml"))
		assert.NotNil(t, options.TokenSource)
	})
}

func TestDisplayTextDiff_UsesChangedFieldsForUpdateOutput(t *testing.T) {
	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:           "1:u:event_gateway_listener:listener-a",
				ResourceType: planner.ResourceTypeEventGatewayListener,
				ResourceRef:  "listener-a",
				ResourceID:   "listener-id",
				Action:       planner.ActionUpdate,
				Namespace:    "default",
				Fields: map[string]any{
					"name":        "listener-a",
					"description": "new description",
					"addresses":   []string{"0.0.0.0"},
				},
				ChangedFields: map[string]planner.FieldChange{
					"description": {
						Old: "old description",
						New: "new description",
					},
				},
			},
		},
		ExecutionOrder: []string{"1:u:event_gateway_listener:listener-a"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction: map[planner.ActionType]int{
				planner.ActionUpdate: 1,
			},
			ByResource: map[string]int{
				planner.ResourceTypeEventGatewayListener: 1,
			},
		},
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := displayTextDiff(cmd, plan, false)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, `description: "old description" → "new description"`)
	assert.NotContains(t, output, "addresses:")
	assert.NotContains(t, output, `name: "listener-a"`)
}

func TestDisplayTextDiff_RedactsSensitiveChangedFields(t *testing.T) {
	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:           "1:u:application_auth_strategy:portal-auth",
				ResourceType: "application_auth_strategy",
				ResourceRef:  "portal-auth",
				Action:       planner.ActionUpdate,
				Namespace:    "default",
				ChangedFields: map[string]planner.FieldChange{
					"oidc_client_secret": {
						Old: "old-secret-value",
						New: "new-secret-value",
					},
				},
			},
		},
		ExecutionOrder: []string{"1:u:application_auth_strategy:portal-auth"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction: map[planner.ActionType]int{
				planner.ActionUpdate: 1,
			},
			ByResource: map[string]int{
				"application_auth_strategy": 1,
			},
		},
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := displayTextDiff(cmd, plan, false)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "oidc_client_secret: [REDACTED] → [REDACTED]")
	assert.NotContains(t, output, "old-secret-value")
	assert.NotContains(t, output, "new-secret-value")
}

func TestDisplayTextDiff_RedactsSensitiveCreateFields(t *testing.T) {
	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:           "1:c:portal_custom_domain:my-domain",
				ResourceType: "portal_custom_domain",
				ResourceRef:  "my-domain",
				Action:       planner.ActionCreate,
				Namespace:    "default",
				Fields: map[string]any{
					"hostname": "portal.example.com",
					"ssl": map[string]any{
						"custom_private_key": "very-secret-private-key",
					},
				},
			},
		},
		ExecutionOrder: []string{"1:c:portal_custom_domain:my-domain"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction: map[planner.ActionType]int{
				planner.ActionCreate: 1,
			},
			ByResource: map[string]int{
				"portal_custom_domain": 1,
			},
		},
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := displayTextDiff(cmd, plan, false)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "custom_private_key: [REDACTED]")
	assert.NotContains(t, output, "very-secret-private-key")
}

func TestDisplayTextDiff_RedactsDeferredEnvValues(t *testing.T) {
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1:c:portal:env-portal",
		Action:       planner.ActionCreate,
		ResourceType: "portal",
		ResourceRef:  "env-portal",
		Fields: map[string]any{
			"name":        "env-portal",
			"description": "__ENV__:PORTAL_DESCRIPTION",
		},
		References: map[string]planner.ReferenceInfo{
			"default_application_auth_strategy_id": {
				Ref: "__ENV__:PORTAL_AUTH_STRATEGY",
			},
		},
	})
	plan.SetExecutionOrder([]string{"1:c:portal:env-portal"})

	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&out)

	err := displayTextDiff(command, plan, false)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "[redacted from !env]")
	assert.NotContains(t, out.String(), "__ENV__:PORTAL_DESCRIPTION")
	assert.NotContains(t, out.String(), "__ENV__:PORTAL_AUTH_STRATEGY")
}

func TestDisplayTextDiff_ShowsUnknownReferencesAsPending(t *testing.T) {
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1:c:portal:env-portal",
		Action:       planner.ActionCreate,
		ResourceType: "portal",
		ResourceRef:  "env-portal",
		Fields: map[string]any{
			"name": "env-portal",
		},
		References: map[string]planner.ReferenceInfo{
			"default_application_auth_strategy_id": {
				Ref: "basic-auth",
				ID:  resources.UnknownReferenceID,
			},
		},
	})
	plan.SetExecutionOrder([]string{"1:c:portal:env-portal"})

	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&out)

	err := displayTextDiff(command, plan, false)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "default_application_auth_strategy_id: basic-auth (to be resolved)")
	assert.NotContains(t, out.String(), "basic-auth → [unknown]")
}
