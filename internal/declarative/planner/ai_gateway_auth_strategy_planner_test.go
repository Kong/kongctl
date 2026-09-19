package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateAIGatewayAuthStrategyRejectsTypeChange(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, _, err := shouldUpdateAIGatewayAuthStrategy(
		state.AIGatewayAuthStrategy{
			Type:        "key-auth",
			DisplayName: "Support Key Auth",
			Config:      map[string]any{"key_names": []any{"apikey"}},
		},
		resources.AIGatewayAuthStrategyResource{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config:      map[string]any{"issuer": "https://issuer.example.com"},
		},
	)

	require.Error(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Contains(t, err.Error(), "changing AI Gateway Auth Strategy type")
}

func TestAIGatewayAuthStrategyConfigChangedIgnoresClientSecret(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth_methods": []any{"bearer"},
		"client_id":    []any{"support-client"},
		"issuer":       "https://issuer.example.com",
	}
	desired := map[string]any{
		"auth_methods":  []any{"bearer"},
		"client_id":     []any{"support-client"},
		"client_secret": []any{"super-secret"},
		"issuer":        "https://issuer.example.com",
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedComparesPublicVaultReferences(t *testing.T) {
	t.Parallel()

	config := func(values ...any) map[string]any {
		return map[string]any{
			"auth_methods":  []any{"bearer"},
			"client_id":     []any{"primary-client", "fallback-client"},
			"client_secret": values,
			"issuer":        "https://issuer.example.com",
		}
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(
		config("{vault://support-secrets/primary}", "hidden-current"),
		config("{vault://support-secrets/primary}", "hidden-desired"),
	))
	require.True(t, aiGatewayAuthStrategyConfigChanged(
		config("{vault://support-secrets/old-primary}", "hidden-current"),
		config("{vault://support-secrets/new-primary}", "hidden-desired"),
	))
}

func TestAIGatewayAuthStrategyConfigChangedIgnoresVaultReferenceMissingFromResponse(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth_methods": []any{"bearer"},
		"client_id":    []any{"primary-client"},
		"issuer":       "https://issuer.example.com",
	}
	desired := map[string]any{
		"auth_methods":  []any{"bearer"},
		"client_id":     []any{"primary-client"},
		"client_secret": []any{"{vault://support-secrets/primary}"},
		"issuer":        "https://issuer.example.com",
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedDetectsObservableChanges(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth_methods": []any{"bearer"},
		"client_id":    []any{"support-client"},
		"issuer":       "https://issuer.example.com",
	}
	desired := map[string]any{
		"auth_methods":  []any{"bearer"},
		"client_id":     []any{"support-client"},
		"client_secret": []any{"super-secret"},
		"issuer":        "https://issuer-updated.example.com",
	}

	require.True(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedIgnoresUndeclaredDefaults(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"hide_credentials": true,
		"key_in_body":      false,
		"key_in_header":    true,
		"key_in_query":     true,
		"key_names":        []any{"x-support-api-key"},
	}
	desired := map[string]any{
		"hide_credentials": true,
		"key_names":        []any{"x-support-api-key"},
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedComparesDeclaredDefaults(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"hide_credentials": true,
		"key_in_body":      false,
		"key_in_header":    true,
		"key_in_query":     true,
		"key_names":        []any{"x-support-api-key"},
	}
	desired := map[string]any{
		"hide_credentials": true,
		"key_in_body":      true,
		"key_names":        []any{"x-support-api-key"},
	}

	require.True(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyChangedFieldsScrubClientSecret(t *testing.T) {
	t.Parallel()

	needsUpdate, _, changedFields, err := shouldUpdateAIGatewayAuthStrategy(
		state.AIGatewayAuthStrategy{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"auth_methods": []any{"bearer"},
				"client_id":    []any{"support-client"},
				"issuer":       "https://issuer.example.com",
			},
		},
		resources.AIGatewayAuthStrategyResource{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"auth_methods":  []any{"bearer"},
				"client_id":     []any{"support-client"},
				"client_secret": []any{"super-secret"},
				"issuer":        "https://issuer-updated.example.com",
			},
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.NotContains(t, changedFields[FieldConfig].New, "client_secret")
}

func TestAIGatewayAuthStrategyChangedFieldsPruneUnpairedVaultReference(t *testing.T) {
	t.Parallel()

	needsUpdate, _, changedFields, err := shouldUpdateAIGatewayAuthStrategy(
		state.AIGatewayAuthStrategy{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"issuer": "https://issuer.example.com",
			},
		},
		resources.AIGatewayAuthStrategyResource{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"issuer":          "https://issuer-updated.example.com",
				FieldClientSecret: []any{"{vault://support-secrets/primary}"},
			},
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.NotContains(t, changedFields[FieldConfig].New, FieldClientSecret)
}

func TestAIGatewayAuthStrategyUpdatePreservesUndeclaredSecurityConfig(t *testing.T) {
	t.Parallel()

	current := state.AIGatewayAuthStrategy{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Support OIDC",
		Labels:      map[string]string{"owner": "security"},
		ManagedBy:   map[string]string{"terraform": "legacy"},
		Config: map[string]any{
			"auth_methods":             []any{"bearer"},
			"cache_tokens_salt":        "support-cache-salt",
			"consumer_groups_claim":    []any{"groups"},
			"consumer_groups_optional": false,
			"upstream_headers_claims":  []any{"sub"},
			"upstream_headers_names":   []any{"x-consumer-subject"},
		},
	}
	desired := resources.AIGatewayAuthStrategyResource{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Updated Support OIDC",
		Config: map[string]any{
			"auth_methods":      []any{"bearer"},
			"cache_tokens_salt": "support-cache-salt",
		},
	}

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayAuthStrategy(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Contains(t, changedFields, FieldDisplayName)
	require.NotContains(t, changedFields, FieldConfig)
	require.Equal(t, current.Config, fields[FieldConfig])
	require.Equal(t, current.Labels, fields[FieldLabels])
	require.Equal(t, current.ManagedBy, fields[FieldManagedBy])
}

func TestAIGatewayAuthStrategyUpdateOverlaysDeclaredSecurityConfig(t *testing.T) {
	t.Parallel()

	current := state.AIGatewayAuthStrategy{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Support OIDC",
		Config: map[string]any{
			"cache_tokens_salt":       "support-cache-salt",
			"consumer_groups_claim":   []any{"groups"},
			"upstream_headers_claims": []any{"sub"},
			"nested_extension": map[string]any{
				"preserved": true,
				"managed":   "old",
			},
		},
	}
	desired := resources.AIGatewayAuthStrategyResource{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Support OIDC",
		Config: map[string]any{
			"cache_tokens_salt":     "support-cache-salt",
			"consumer_groups_claim": []any{"roles"},
			"nested_extension": map[string]any{
				"managed": "new",
			},
		},
	}

	needsUpdate, fields, _, err := shouldUpdateAIGatewayAuthStrategy(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	config := fields[FieldConfig].(map[string]any)
	require.Equal(t, []any{"roles"}, config["consumer_groups_claim"])
	require.Equal(t, []any{"sub"}, config["upstream_headers_claims"])
	require.Equal(t, map[string]any{"preserved": true, "managed": "new"}, config["nested_extension"])
}

func TestAIGatewayAuthStrategyPlannerNameIdentity(t *testing.T) {
	t.Parallel()
	const (
		cachedID  = "11111111-1111-4111-8111-111111111111"
		refID     = "22222222-2222-4222-8222-222222222222"
		nameID    = "33333333-3333-4333-8333-333333333333"
		missingID = "44444444-4444-4444-8444-444444444444"
	)
	for _, tt := range []struct {
		name, ref, cachedID, desiredName                      string
		action                                                ActionType
		apply, missingDetail, readError, protected, unchanged bool
	}{
		{name: "name wins over UUID ref", ref: refID, desiredName: "desired-name", action: ActionUpdate},
		{
			name: "name wins over cached ID", ref: "local-ref", cachedID: cachedID,
			desiredName: "desired-name", action: ActionUpdate,
		},
		{
			name: "missing IDs do not hide name", ref: missingID, cachedID: missingID,
			desiredName: "desired-name", action: ActionUpdate,
		},
		{name: "UUID ref does not rename or retain", ref: refID, desiredName: "new-name", action: ActionCreate},
		{
			name: "cached ID does not rename or retain", ref: "local-ref", cachedID: cachedID,
			desiredName: "new-name", action: ActionCreate,
		},
		{
			name: "absent detail recreates and retains declared name", ref: refID,
			desiredName: "desired-name", action: ActionCreate, missingDetail: true,
		},
		{name: "detail error stops before sync", ref: refID, desiredName: "desired-name", readError: true},
		{
			name: "protection stops sync at first deletion", ref: refID,
			desiredName: "desired-name", action: ActionUpdate, protected: true,
		},
		{name: "apply does not prune", ref: refID, desiredName: "new-name", action: ActionCreate, apply: true},
		{name: "unchanged detail retains name", ref: refID, desiredName: "desired-name", unchanged: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := &nameIdentityAuthStrategyAPI{missingDetail: tt.missingDetail, readError: tt.readError}
			for _, identity := range []struct{ id, name string }{
				{cachedID, "cached-name"}, {refID, "ref-name"}, {nameID, "desired-name"},
			} {
				raw := map[string]any{
					"id": identity.id, "name": identity.name, "type": "key-auth",
					"display_name": "Original child", "config": map[string]any{},
					"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
				}
				if tt.protected && identity.id == cachedID {
					raw["labels"] = map[string]string{labels.ProtectedKey: labels.TrueValue}
				}
				data, err := json.Marshal(raw)
				require.NoError(t, err)
				var child kkComps.AIGatewayAuthStrategy
				require.NoError(t, json.Unmarshal(data, &child))
				api.children = append(api.children, child)
			}
			desired := resources.AIGatewayAuthStrategyResource{
				BaseResource: resources.BaseResource{Ref: tt.ref},
				Name:         tt.desiredName, Type: "key-auth", DisplayName: "Updated child",
			}
			if tt.unchanged {
				desired.DisplayName = "Original child"
			}
			desired.SetKonnectID(tt.cachedID)
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayAuthStrategiesAPI: api}), slog.Default())
			mode := PlanModeSync
			if tt.apply {
				mode = PlanModeApply
			}
			plan := NewPlan(CurrentPlanVersion, "test", mode)
			err := p.planAIGatewayAuthStrategyChanges(t.Context(), nil,
				"test-namespace", "Gateway", "gateway-id", "gateway-ref", "",
				[]resources.AIGatewayAuthStrategyResource{desired}, plan)
			require.Equal(t, []string{"gateway-id"}, api.lists)
			if tt.desiredName == "desired-name" {
				require.Equal(t, [][2]string{{"gateway-id", nameID}}, api.reads)
			} else {
				require.Empty(t, api.reads)
			}
			if tt.readError {
				require.ErrorContains(t, err, "failed to get AI Gateway Auth Strategy "+nameID)
				require.ErrorContains(t, err, "detail unavailable")
				require.Empty(t, plan.Changes)
				return
			}
			if tt.protected {
				require.ErrorContains(t, err, "protected")
			} else {
				require.NoError(t, err)
			}
			wantDeleteIDs := []string{cachedID, refID}
			if tt.desiredName == "new-name" {
				wantDeleteIDs = append(wantDeleteIDs, nameID)
			}
			if tt.apply || tt.protected {
				wantDeleteIDs = nil
			}
			offset := 0
			if !tt.unchanged {
				offset = 1
			}
			require.Len(t, plan.Changes, offset+len(wantDeleteIDs))
			if !tt.unchanged {
				change := plan.Changes[0]
				require.Equal(t, tt.action, change.Action)
				wantID := ""
				if tt.action == ActionUpdate {
					wantID = nameID
					require.Equal(t, FieldChange{Old: "Original child", New: "Updated child"},
						change.ChangedFields[FieldDisplayName])
				}
				require.Equal(t, wantID, change.ResourceID)
				require.Equal(t, tt.ref, change.ResourceRef)
				require.Equal(t, tt.desiredName, change.Fields[FieldName])
				require.Equal(t, "Updated child", change.Fields[FieldDisplayName])
			}
			for i, id := range wantDeleteIDs {
				require.Equal(t, ActionDelete, plan.Changes[offset+i].Action)
				require.Equal(t, id, plan.Changes[offset+i].ResourceID)
			}
			for _, change := range plan.Changes {
				require.Equal(t, ResourceTypeAIGatewayAuthStrategy, change.ResourceType)
				require.Equal(t, "test-namespace", change.Namespace)
				require.Equal(t, &ParentInfo{Ref: "gateway-ref", ID: "gateway-id"}, change.Parent)
			}
		})
	}
}

type nameIdentityAuthStrategyAPI struct {
	helpers.AIGatewayAuthStrategiesAPI
	children                 []kkComps.AIGatewayAuthStrategy
	lists                    []string
	reads                    [][2]string
	missingDetail, readError bool
}

func (a *nameIdentityAuthStrategyAPI) ListAiGatewayAuthStrategies(
	_ context.Context, req kkOps.ListAiGatewayAuthStrategiesRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayAuthStrategiesResponse, error) {
	a.lists = append(a.lists, req.GatewayID)
	return &kkOps.ListAiGatewayAuthStrategiesResponse{
		ListAIGatewayAuthStrategiesResponse: &kkComps.ListAIGatewayAuthStrategiesResponse{Data: a.children},
	}, nil
}

func (a *nameIdentityAuthStrategyAPI) GetAiGatewayAuthStrategy(
	_ context.Context, gatewayID, childID string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayAuthStrategyResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, childID})
	if a.readError {
		return nil, errors.New("detail unavailable")
	}
	if a.missingDetail {
		return &kkOps.GetAiGatewayAuthStrategyResponse{}, nil
	}
	for _, child := range a.children {
		current, err := state.NormalizeAIGatewayAuthStrategy(child)
		if err != nil {
			return nil, err
		}
		if current.ID == childID {
			return &kkOps.GetAiGatewayAuthStrategyResponse{AIGatewayAuthStrategy: &child}, nil
		}
	}
	return &kkOps.GetAiGatewayAuthStrategyResponse{}, nil
}
