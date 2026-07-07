package aigateway

import (
	"bytes"
	"context"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

type aiGatewayAPIStub struct {
	t        *testing.T
	gateways []kkComps.AIGateway
}

func (s *aiGatewayAPIStub) ListAiGateways(
	_ context.Context,
	_ *int64,
	_ *int64,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewaysResponse, error) {
	return &kkOps.ListAiGatewaysResponse{
		ListAIGatewaysResponse: &kkComps.ListAIGatewaysResponse{
			Data: s.gateways,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(s.gateways))},
			},
		},
	}, nil
}

func (s *aiGatewayAPIStub) CreateAiGateway(
	context.Context,
	kkComps.CreateAIGatewayRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayResponse, error) {
	s.t.Fatalf("unexpected CreateAiGateway call")
	return nil, nil
}

func (s *aiGatewayAPIStub) GetAiGateway(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayResponse, error) {
	s.t.Fatalf("unexpected GetAiGateway call")
	return nil, nil
}

func (s *aiGatewayAPIStub) UpdateAiGateway(
	context.Context,
	string,
	kkComps.UpdateAIGatewayRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayResponse, error) {
	s.t.Fatalf("unexpected UpdateAiGateway call")
	return nil, nil
}

func (s *aiGatewayAPIStub) DeleteAiGateway(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayResponse, error) {
	s.t.Fatalf("unexpected DeleteAiGateway call")
	return nil, nil
}

type aiGatewayTestConfig struct{}

func (aiGatewayTestConfig) GetString(string) string               { return "" }
func (aiGatewayTestConfig) GetBool(string) bool                   { return false }
func (aiGatewayTestConfig) GetInt(string) int                     { return 50 }
func (aiGatewayTestConfig) GetIntOrElse(_ string, orElse int) int { return orElse }
func (aiGatewayTestConfig) GetStringSlice(string) []string        { return nil }
func (aiGatewayTestConfig) SetString(string, string)              {}
func (aiGatewayTestConfig) Set(string, any)                       {}
func (aiGatewayTestConfig) Get(string) any                        { return nil }
func (aiGatewayTestConfig) InConfig(string) bool                  { return false }
func (aiGatewayTestConfig) BindFlag(string, *pflag.Flag) error    { return nil }
func (aiGatewayTestConfig) GetProfile() string                    { return "test" }
func (aiGatewayTestConfig) GetPath() string                       { return "" }

func TestAIGatewayHelpListsModelsOnce(t *testing.T) {
	t.Parallel()

	rootCmd, err := NewAIGatewayCmd(
		verbs.Get,
		func(verbs.VerbValue, *cobra.Command) {},
		func(*cobra.Command, []string) error { return nil },
	)
	require.NoError(t, err)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--help"})

	require.NoError(t, rootCmd.Execute())
	require.Equal(t, 1, strings.Count(out.String(), "\n  models "))
}

func TestAIGatewayChildHelpDescribesGatewayNameLookup(t *testing.T) {
	t.Parallel()

	rootCmd, err := NewAIGatewayCmd(
		verbs.Get,
		func(verbs.VerbValue, *cobra.Command) {},
		func(*cobra.Command, []string) error { return nil },
	)
	require.NoError(t, err)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"model-providers", "--help"})

	require.NoError(t, rootCmd.Execute())
	require.Contains(t, out.String(), "The name or display_name of the AI Gateway that owns the resource.")
}

func TestRunListByNameOrDisplayNameMatchesNameFirst(t *testing.T) {
	t.Parallel()

	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(context.Background())

	gateway, err := runListByNameOrDisplayName(
		"support-gateway",
		&aiGatewayAPIStub{
			t: t,
			gateways: []kkComps.AIGateway{
				{ID: "name-match-id", Name: "support-gateway", DisplayName: "Support Gateway"},
				{ID: "display-name-match-id", Name: "other-gateway", DisplayName: "support-gateway"},
			},
		},
		helper,
		aiGatewayTestConfig{},
	)

	require.NoError(t, err)
	require.NotNil(t, gateway)
	require.Equal(t, "name-match-id", gateway.ID)
}

func TestRunListByNameOrDisplayNameFallsBackToDisplayName(t *testing.T) {
	t.Parallel()

	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(context.Background())

	gateway, err := runListByNameOrDisplayName(
		"Support Gateway",
		&aiGatewayAPIStub{
			t: t,
			gateways: []kkComps.AIGateway{
				{ID: "display-name-match-id", Name: "support-gateway", DisplayName: "Support Gateway"},
			},
		},
		helper,
		aiGatewayTestConfig{},
	)

	require.NoError(t, err)
	require.NotNil(t, gateway)
	require.Equal(t, "display-name-match-id", gateway.ID)
}

func TestResolveAIGatewayIDByNameUsesSharedNameResolution(t *testing.T) {
	t.Parallel()

	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(context.Background())

	gatewayID, err := resolveAIGatewayIDByName(
		"support-gateway",
		&aiGatewayAPIStub{
			t: t,
			gateways: []kkComps.AIGateway{
				{ID: "gateway-id", Name: "support-gateway", DisplayName: "Support Gateway"},
			},
		},
		helper,
		aiGatewayTestConfig{},
	)

	require.NoError(t, err)
	require.Equal(t, "gateway-id", gatewayID)
}

func TestRunListByNameOrDisplayNameReportsBothFields(t *testing.T) {
	t.Parallel()

	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(context.Background())

	_, err := runListByNameOrDisplayName(
		"missing-gateway",
		&aiGatewayAPIStub{t: t},
		helper,
		aiGatewayTestConfig{},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), `AI Gateway with name or display_name "missing-gateway" not found`)
}

func TestRedactAIGatewayIdentityProviderSecrets(t *testing.T) {
	t.Parallel()

	redacted := redactAIGatewayIdentityProviderSecrets(map[string]any{
		"config": map[string]any{
			"client_secret": []any{"secret"},
			"nested": []any{
				map[string]any{"client_secret": "nested-secret"},
			},
		},
	})

	require.Equal(t, "[redacted]", redacted["config"].(map[string]any)["client_secret"])
	nested := redacted["config"].(map[string]any)["nested"].([]any)[0].(map[string]any)
	require.Equal(t, "[redacted]", nested["client_secret"])
}

var _ config.Hook = aiGatewayTestConfig{}
