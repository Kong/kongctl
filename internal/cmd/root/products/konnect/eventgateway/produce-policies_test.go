package eventgateway

import (
	"context"
	"log/slog"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestProducePolicyFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "both gateway flags mutually exclusive",
			args:    []string{"--gateway-id", "gw-1", "--gateway-name", "gw"},
			wantErr: "if any flags in the group [gateway-id gateway-name] are set none of the others can be",
		},
		{
			name: "both produce policy flags mutually exclusive",
			args: []string{
				"--gateway-id", "gw-1",
				"--virtual-cluster-id", "vc-1",
				"--policy-id", "policy-1",
				"--policy-name", "policy",
			},
			wantErr: "if any flags in the group [policy-id policy-name] are set none of the others can be",
		},
		{
			name: "both virtual cluster flags mutually exclusive",
			args: []string{
				"--gateway-id", "gw-1",
				"--virtual-cluster-id", "vc-1",
				"--virtual-cluster-name", "vc",
			},
			wantErr: "if any flags in the group [virtual-cluster-id virtual-cluster-name] are set none of the others can be", //nolint:lll
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Minimal context - Cobra validates before SDK is needed
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(common.OutputConfigPath, "text")

			ctx := context.Background()
			ctx = context.WithValue(ctx, config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.Default())
			ctx = context.WithValue(ctx, iostreams.StreamsKey, iostreams.NewTestIOStreamsOnly())

			cmd := newGetEventGatewayProducePoliciesCmd(verbs.Get, nil, nil)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestFindProducePolicyByName(t *testing.T) {
	policies := []kkComps.EventGatewayPolicy{
		{ID: "policy-1", Name: ptr("Alpha-Policy")},
		{ID: "policy-2", Name: nil},
		{ID: "policy-3", Name: ptr("Beta-Policy")},
	}

	// Case-insensitive match
	assert.Equal(t, "policy-1", findProducePolicyByName(policies, "alpha-policy").ID)
	assert.Equal(t, "policy-3", findProducePolicyByName(policies, "BETA-POLICY").ID)
	// Not found
	assert.Nil(t, findProducePolicyByName(policies, "missing"))
	// Empty name
	assert.Nil(t, findProducePolicyByName(policies, ""))
}

func TestProducePolicyToRecord(t *testing.T) {
	now := time.Now()
	policy := kkComps.EventGatewayPolicy{
		ID:          "test-id-1234-5678-9abc-def012345678",
		Type:        "modify_headers",
		Name:        ptr("Test Policy"),
		Description: ptr("Test Description"),
		Enabled:     ptrBool(true),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	record := producePolicyToRecord(policy)

	assert.Contains(t, record.ID, "test-id") // May be abbreviated
	assert.Equal(t, "Test Policy", record.Name)
	assert.Equal(t, "modify_headers", record.Type)
	assert.Equal(t, "Test Description", record.Description)
	assert.Equal(t, "true", record.Enabled)
	assert.NotEmpty(t, record.LocalCreatedTime)
	assert.NotEmpty(t, record.LocalUpdatedTime)
}

func TestProducePolicyToRecord_NilFields(t *testing.T) {
	policy := kkComps.EventGatewayPolicy{
		ID:        "test-id",
		Type:      "encrypt",
		Name:      nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	record := producePolicyToRecord(policy)

	assert.Equal(t, valueNA, record.Name)
	assert.Equal(t, valueNA, record.Description)
	assert.Equal(t, valueNA, record.Enabled)
}

func TestProducePolicyDetailView(t *testing.T) {
	// Nil returns empty
	assert.Empty(t, producePolicyWithConfigDetailView(nil))

	// Populated fields appear
	policy := &producePolicyWithConfig{
		ID:          "test-id",
		Type:        "schema_validation",
		Name:        ptr("Test Policy"),
		Description: ptr("Test Description"),
		Enabled:     ptrBool(true),
		Condition:   ptr("true"),
		Config:      map[string]any{"key": "value"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	detail := producePolicyWithConfigDetailView(policy)

	assert.Contains(t, detail, "id: test-id")
	assert.Contains(t, detail, "type: schema_validation")
	assert.Contains(t, detail, "name: Test Policy")
	assert.Contains(t, detail, "description: Test Description")
	assert.Contains(t, detail, "enabled: true")
	assert.Contains(t, detail, "condition: true")
}

// Helper to create pointer to string
func ptr(s string) *string {
	return &s
}

// Helper to create pointer to bool
func ptrBool(b bool) *bool {
	return &b
}
