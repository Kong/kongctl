package auditlogs

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/stretchr/testify/require"
)

func TestExtractDestinationRecords(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"data": []any{
			map[string]any{
				"id":                    "dest-2",
				"name":                  "second",
				"endpoint":              "https://second.example/audit-logs",
				"log_format":            "cef",
				"skip_ssl_verification": true,
			},
			map[string]any{
				"id":                    "dest-1",
				"name":                  "first",
				"endpoint":              "https://first.example/audit-logs",
				"log_format":            "json",
				"skip_ssl_verification": false,
			},
		},
	}

	records := extractDestinationRecords(payload)
	require.Len(t, records, 2)
	require.Equal(t, "dest-1", records[0].ID)
	require.Equal(t, "first", records[0].Name)
	require.Equal(t, "https://first.example/audit-logs", records[0].Endpoint)
	require.NotNil(t, records[0].SkipSSLVerification)
	require.False(t, *records[0].SkipSSLVerification)

	require.Equal(t, "dest-2", records[1].ID)
	require.Equal(t, "second", records[1].Name)
	require.NotNil(t, records[1].SkipSSLVerification)
	require.True(t, *records[1].SkipSSLVerification)
}

func TestFindDestinationRecord(t *testing.T) {
	t.Parallel()

	records := []auditLogDestinationRecord{
		{ID: "dest-1", Name: "alpha", Endpoint: "https://alpha.example"},
		{ID: "dest-2", Name: "beta", Endpoint: "https://beta.example"},
		{ID: "dest-3", Name: "beta", Endpoint: "https://beta-2.example"},
	}

	t.Run("find by id", func(t *testing.T) {
		t.Parallel()

		record, err := findDestinationRecord(records, "dest-1")
		require.NoError(t, err)
		require.Equal(t, "alpha", record.Name)
	})

	t.Run("find by unique name", func(t *testing.T) {
		t.Parallel()

		record, err := findDestinationRecord(records, "alpha")
		require.NoError(t, err)
		require.Equal(t, "dest-1", record.ID)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		_, err := findDestinationRecord(records, "missing")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("ambiguous name", func(t *testing.T) {
		t.Parallel()

		_, err := findDestinationRecord(records, "beta")
		require.Error(t, err)
		require.Contains(t, err.Error(), "multiple audit-log destinations matched")
	})
}

func TestExtractWebhookConfig(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"enabled":                  true,
		"endpoint":                 "https://example.test/audit-logs",
		"log_format":               "json",
		"skip_ssl_verification":    false,
		"audit_log_destination_id": "dest-123",
		"updated_at":               "2026-02-20T00:00:00Z",
	}

	webhookCfg := extractWebhookConfig(payload)
	require.NotNil(t, webhookCfg.Enabled)
	require.True(t, *webhookCfg.Enabled)
	require.Equal(t, "https://example.test/audit-logs", webhookCfg.Endpoint)
	require.Equal(t, "json", webhookCfg.LogFormat)
	require.NotNil(t, webhookCfg.SkipSSLVerification)
	require.False(t, *webhookCfg.SkipSSLVerification)
	require.Equal(t, "dest-123", webhookCfg.DestinationID)
	require.Equal(t, "2026-02-20T00:00:00Z", webhookCfg.UpdatedAt)
}

func TestResolveOutputPayloadAppliesJQFilter(t *testing.T) {
	t.Parallel()

	helper, streams := newJQTestHelper(t, ".name", false, true)
	raw := auditLogDestinationRecord{
		ID:   "dest-1",
		Name: "alpha",
	}

	filtered, handled, err := resolveOutputPayload(helper, cmdcommon.JSON, raw)
	require.NoError(t, err)
	require.False(t, handled)
	require.Equal(t, "alpha", filtered)

	require.Empty(t, streams.Out.(*strings.Builder).String())
	helper.AssertExpectations(t)
}

func TestResolveOutputPayloadHandlesJQRawOutput(t *testing.T) {
	t.Parallel()

	helper, streams := newJQTestHelper(t, ".name", true, true)
	raw := auditLogDestinationRecord{
		ID:   "dest-1",
		Name: "alpha",
	}

	filtered, handled, err := resolveOutputPayload(helper, cmdcommon.JSON, raw)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, filtered)
	require.Equal(t, "alpha\n", streams.Out.(*strings.Builder).String())
	helper.AssertExpectations(t)
}

func TestResolveOutputPayloadRejectsJQForTextOutput(t *testing.T) {
	t.Parallel()

	helper, _ := newJQTestHelper(t, ".name", false, false)
	raw := auditLogDestinationRecord{
		ID:   "dest-1",
		Name: "alpha",
	}

	_, handled, err := resolveOutputPayload(helper, cmdcommon.TEXT, raw)
	require.Error(t, err)
	require.False(t, handled)
	require.Contains(t, err.Error(), "--jq is only supported with --output json or --output yaml")
	helper.AssertExpectations(t)
}

func newJQTestHelper(
	t *testing.T,
	jqExpression string,
	rawOutput bool,
	expectStreams bool,
) (*cmd.MockHelper, *iostreams.IOStreams) {
	t.Helper()

	cmdObj := &cobra.Command{Use: "audit-logs-test"}
	jqoutput.AddFlags(cmdObj.Flags())

	require.NoError(t, cmdObj.Flags().Set(jqoutput.FlagName, jqExpression))
	if rawOutput {
		require.NoError(t, cmdObj.Flags().Set(jqoutput.RawOutputFlagName, "true"))
	}

	cfg := newTestConfigHook(t)
	require.NoError(t, jqoutput.BindFlags(cfg, cmdObj.Flags()))

	streams := &iostreams.IOStreams{
		Out:    &strings.Builder{},
		ErrOut: &strings.Builder{},
	}

	helper := &cmd.MockHelper{}
	helper.EXPECT().GetConfig().Return(cfg, nil)
	helper.EXPECT().GetCmd().Return(cmdObj)
	if expectStreams {
		helper.EXPECT().GetStreams().Return(streams)
	}

	return helper, streams
}

func newTestConfigHook(t *testing.T) config.Hook {
	t.Helper()

	main := viper.New()
	main.Set("default", map[string]any{
		"output": "json",
		"jq": map[string]any{
			"color": map[string]any{
				"enabled": "never",
				"theme":   "friendly",
			},
		},
	})

	return config.BuildProfiledConfig("default", "/tmp/kongctl-test-config.yaml", main)
}
