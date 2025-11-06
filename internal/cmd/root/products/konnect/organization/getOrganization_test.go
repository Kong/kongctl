package organization

import (
	"context"
	"testing"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrs "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationToDisplayRecord(t *testing.T) {
	now := time.Date(2024, time.January, 15, 10, 30, 45, 0, time.UTC)
	id := "12345678-1234-1234-1234-1234567890ab"
	ownerID := "abcdef12-3456-7890-abcd-ef1234567890"
	state := kkComps.MeOrganizationStateActive
	retention := int64(30)
	loginPath := "/login/acme"
	name := "Acme Org"

	tests := []struct {
		name     string
		input    *kkComps.MeOrganization
		expected textDisplayRecord
	}{
		{
			name:  "nil organization",
			input: nil,
			expected: textDisplayRecord{
				ID:                  "n/a",
				Name:                "n/a",
				State:               "n/a",
				OwnerID:             "n/a",
				LoginPath:           "n/a",
				RetentionPeriodDays: "n/a",
				LocalCreatedTime:    "n/a",
				LocalUpdatedTime:    "n/a",
			},
		},
		{
			name: "populated organization",
			input: &kkComps.MeOrganization{
				ID:                  kk.String(id),
				Name:                kk.String(name),
				State:               state.ToPointer(),
				OwnerID:             kk.String(ownerID),
				LoginPath:           &loginPath,
				RetentionPeriodDays: &retention,
				CreatedAt:           &now,
				UpdatedAt:           &now,
			},
			expected: textDisplayRecord{
				ID:                  "1234…",
				Name:                name,
				State:               string(state),
				OwnerID:             "abcd…",
				LoginPath:           loginPath,
				RetentionPeriodDays: "30",
				LocalCreatedTime:    now.In(time.Local).Format("2006-01-02 15:04:05"),
				LocalUpdatedTime:    now.In(time.Local).Format("2006-01-02 15:04:05"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, organizationToDisplayRecord(tt.input))
		})
	}
}

type stubMeAPI struct {
	getOrganizationsMe func(ctx context.Context, opts ...kkOps.Option) (*kkOps.GetOrganizationsMeResponse, error)
}

func (s *stubMeAPI) GetUsersMe(_ context.Context, _ ...kkOps.Option) (*kkOps.GetUsersMeResponse, error) {
	panic("GetUsersMe should not be called in this test")
}

func (s *stubMeAPI) GetOrganizationsMe(
	ctx context.Context,
	opts ...kkOps.Option,
) (*kkOps.GetOrganizationsMeResponse, error) {
	if s.getOrganizationsMe != nil {
		return s.getOrganizationsMe(ctx, opts...)
	}
	return nil, nil
}

func TestRunGetOrganization(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expected := &kkComps.MeOrganization{
			ID: kk.String("org-id"),
		}

		api := &stubMeAPI{
			getOrganizationsMe: func(
				_ context.Context,
				_ ...kkOps.Option,
			) (*kkOps.GetOrganizationsMeResponse, error) {
				return &kkOps.GetOrganizationsMeResponse{
					MeOrganization: expected,
				}, nil
			},
		}

		helper := cmd.NewMockHelper(t)
		helper.EXPECT().GetContext().Return(context.Background())

		org, err := runGetOrganization(api, helper)
		require.NoError(t, err)
		assert.Equal(t, expected, org)
	})

	t.Run("error", func(t *testing.T) {
		api := &stubMeAPI{
			getOrganizationsMe: func(
				_ context.Context,
				_ ...kkOps.Option,
			) (*kkOps.GetOrganizationsMeResponse, error) {
				return nil, kkErrs.NewSDKError("bad request", 400, "", nil)
			},
		}

		helper := cmd.NewMockHelper(t)
		helper.EXPECT().GetContext().Return(context.Background())
		helper.EXPECT().GetCmd().Return(&cobra.Command{Use: "organization"})

		org, err := runGetOrganization(api, helper)
		assert.Nil(t, org)
		var execErr *cmd.ExecutionError
		require.ErrorAs(t, err, &execErr)
	})
}
