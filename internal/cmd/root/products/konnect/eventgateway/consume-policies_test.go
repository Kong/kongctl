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

// ptr is a helper to create string pointers in tests.
func ptr(s string) *string { return &s }

// boolPtr is a helper to create bool pointers in tests.
func boolPtr(b bool) *bool { return &b }

func TestConsumePolicyFlagValidation(t *testing.T) {
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
			name: "both virtual cluster flags mutually exclusive",
			args: []string{
				"--gateway-id", "gw-1",
				"--virtual-cluster-id", "vc-1",
				"--virtual-cluster-name", "vc",
			},
			wantErr: "if any flags in the group [virtual-cluster-id virtual-cluster-name] are set none of the others can be",
		},
		{
			name: "both consume policy flags mutually exclusive",
			args: []string{
				"--gateway-id", "gw-1",
				"--virtual-cluster-id", "vc-1",
				"--consume-policy-id", "policy-1",
				"--consume-policy-name", "my-policy",
			},
			wantErr: "if any flags in the group [consume-policy-id consume-policy-name] are set none of the others can be",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(common.OutputConfigPath, "text")

			ctx := context.Background()
			ctx = context.WithValue(ctx, config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.Default())
			ctx = context.WithValue(ctx, iostreams.StreamsKey, iostreams.NewTestIOStreamsOnly())

			cmd := newGetEventGatewayConsumePoliciesCmd(verbs.Get, nil, nil)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestFindConsumePolicyByName(t *testing.T) {
	name1 := "Decrypt-Policy"
	policies := []kkComps.EventGatewayPolicy{
		{ID: "policy-1", Name: &name1, Type: "decrypt"},
		{ID: "policy-2", Name: nil, Type: "schema_validation"},
	}

	// Case-insensitive match
	result := findConsumePolicyByName(policies, "decrypt-policy")
	assert.NotNil(t, result)
	assert.Equal(t, "policy-1", result.ID)

	// Not found
	assert.Nil(t, findConsumePolicyByName(policies, "missing"))

	// No name set
	assert.Nil(t, findConsumePolicyByName(policies, ""))
}

func TestConsumePolicyToRecord(t *testing.T) {
	now := time.Now()
	name := "my-consume-policy"
	desc := "test description"
	enabled := true

	policy := kkComps.EventGatewayPolicy{
		ID:          "00000000-0000-0000-0000-000000000001",
		Name:        &name,
		Type:        "decrypt",
		Description: &desc,
		Enabled:     &enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	record := consumePolicyToRecord(policy)

	// ID is abbreviated
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, name, record.Name)
	assert.Equal(t, "decrypt", record.Type)
	assert.Equal(t, desc, record.Description)
	assert.Equal(t, "true", record.Enabled)
	assert.NotEmpty(t, record.LocalCreatedTime)
	assert.NotEmpty(t, record.LocalUpdatedTime)
}

func TestConsumePolicyToRecordNilFields(t *testing.T) {
	policy := kkComps.EventGatewayPolicy{
		ID:        "",
		Name:      nil,
		Type:      "",
		Enabled:   nil,
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}

	record := consumePolicyToRecord(policy)

	assert.Equal(t, valueNA, record.ID)
	assert.Equal(t, valueNA, record.Name)
	assert.Equal(t, valueNA, record.Type)
	assert.Equal(t, valueNA, record.Description)
	assert.Equal(t, valueNA, record.Enabled)
}
