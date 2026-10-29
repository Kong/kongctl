package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

func TestOrganizationTeamRoleAssignmentErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			transport := customPolicyHTTPClient(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, http.MethodPost, req.Method)
				require.Equal(t, "/v3/teams/team-id/assigned-roles", req.URL.Path)
				return &http.Response{
					StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(fmt.Sprintf(
						`{"status":%d,"title":%q,"detail":"The requested role was not found","instance":"trace-2640"}`,
						status, http.StatusText(status),
					))),
					Request: req,
				}, nil
			})
			sdk := kkSDK.New(kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(transport))
			client := state.NewClient(state.ClientConfig{
				OrganizationTeamRolesAPI: &helpers.OrganizationTeamRolesAPIImpl{SDK: sdk},
			})
			plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
			plan.AddChange(planner.PlannedChange{
				ID: "assign-role", ResourceType: planner.ResourceTypeOrganizationTeamRole,
				ResourceRef: "ai-viewers-role", Namespace: "test", Action: planner.ActionCreate,
				Parent: &planner.ParentInfo{ID: "team-id"},
				Fields: map[string]any{
					planner.FieldRoleName: "Admin", planner.FieldEntityID: "*",
					planner.FieldEntityTypeName: "AI Gateways", planner.FieldEntityRegion: "eu",
				},
			})
			plan.SetExecutionOrder([]string{"assign-role"})
			ctx := context.WithValue(t.Context(), log.LoggerKey, slog.Default())
			result := New(client, nil, false).Execute(ctx, plan)
			require.Equal(t, 1, calls)
			require.Equal(t, 1, result.FailureCount)
			require.Len(t, result.Errors, 1)
			failure := result.Errors[0]
			require.Equal(t, "ai-viewers-role", failure.ResourceName)
			require.Equal(t, "ai-viewers-role", failure.ResourceRef)
			for _, want := range []string{
				"ai-viewers-role", "Admin", "AI Gateways", "trace-2640",
				"The requested role was not found",
			} {
				require.Contains(t, failure.Error, want)
			}
			require.NotContains(t, failure.Error, "[unknown]")
			if status == http.StatusNotFound {
				require.Contains(t, failure.Error, "kongctl api get /v3/roles --base-url https://global.api.konghq.com")
			} else {
				require.NotContains(t, failure.Error, "kongctl api get")
			}
		})
	}
}

func TestExecutionResourceName(t *testing.T) {
	for _, tt := range []struct {
		name   string
		fields map[string]any
		want   string
	}{
		{"missing fields", nil, "manifest-ref"},
		{"role fields", map[string]any{planner.FieldRoleName: "Admin"}, "manifest-ref"},
		{"empty name", map[string]any{planner.FieldName: ""}, "manifest-ref"},
		{"unknown name", map[string]any{planner.FieldName: resources.UnknownReferenceID}, "manifest-ref"},
		{"deferred name", map[string]any{planner.FieldName: tags.BuildEnvPlaceholder("TEAM_NAME", "")}, "manifest-ref"},
		{"name", map[string]any{planner.FieldName: "team-name"}, "team-name"},
		{"display name", map[string]any{planner.FieldDisplayName: "Team Name"}, "Team Name"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, executionResourceName(planner.PlannedChange{
				ResourceRef: "manifest-ref", Fields: tt.fields,
			}))
		})
	}
}
