package planner

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type aiGateway23UsageClient func(*http.Request) (*http.Response, error)

func (f aiGateway23UsageClient) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestAIGatewayDatastoreDeletionChecksUsageAndDetachment(t *testing.T) {
	for _, detach := range []bool{false, true} {
		t.Run(
			map[bool]string{false: "surviving user", true: "detached user"}[detach],
			func(t *testing.T) {
				calls := 0
				httpClient := aiGateway23UsageClient(
					func(req *http.Request) (*http.Response, error) {
						require.Equal(
							t,
							"/v1/ai-gateways/gateway-id/datastores/store-id/usage",
							req.URL.Path,
						)
						calls++
						body := `{"data":[{"type":"model_provider","id":"provider-id",
 "name":"provider","display_name":"Provider"}],"meta":{"page":{"size":100}}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": {"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					},
				)
				sdk := &helpers.KonnectSDK{
					SDK: kkSDK.New(
						kkSDK.WithServerURL("https://konnect.test"),
						kkSDK.WithClient(httpClient),
					),
				}
				p := NewPlanner(
					state.NewClient(
						state.ClientConfig{AIGatewayDatastoresAPI: sdk.GetAIGatewayDatastoresAPI()},
					),
					slog.Default(),
				)
				p.resources = &resources.ResourceSet{}
				plan := &Plan{Changes: []PlannedChange{{
					ID: "delete-store", ResourceType: ResourceTypeAIGatewayDatastore,
					Action: ActionDelete, ResourceID: "store-id", Namespace: "platform",
					Parent: &ParentInfo{Ref: "gateway", ID: "gateway-id"}, Fields: map[string]any{FieldName: "store"},
				}}}
				if detach {
					plan.Changes = append(plan.Changes, PlannedChange{
						ID: "detach-provider", ResourceType: ResourceTypeAIGatewayProvider,
						Action: ActionUpdate, ResourceID: "provider-id", Namespace: "consumer-team",
						Parent: &ParentInfo{Ref: "consumer-gateway-alias", ID: "gateway-id"},
						Fields: map[string]any{},
						ChangedFields: map[string]FieldChange{FieldDatastores: {
							Old: []any{map[string]any{FieldName: "store"}}, New: nil,
						}},
					})
				}
				err := p.resolveFinalAIGatewayDependencies(t.Context(), plan)
				if detach {
					require.NoError(t, err)
					require.Contains(t, plan.Changes[0].DependsOn, "detach-provider")
				} else {
					require.ErrorContains(t, err, "still references it")
				}
				require.Equal(t, 1, calls)
			},
		)
	}
}

func TestAIGateway23DependenciesAcrossNamespacesAndGatewayAliases(t *testing.T) {
	p := NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default())
	p.resources = &resources.ResourceSet{}
	plan := &Plan{Changes: []PlannedChange{
		{
			ID: "store", ResourceType: ResourceTypeAIGatewayDatastore, Action: ActionCreate, Namespace: "platform",
			Parent: &ParentInfo{Ref: "platform-alias", ID: "shared-id"},
		},
		{
			ID: "ca", ResourceType: ResourceTypeAIGatewayCACertificate, Action: ActionCreate, Namespace: "platform",
			Parent: &ParentInfo{Ref: "platform-alias", ID: "shared-id"},
		},
		{
			ID: "server", ResourceType: ResourceTypeAIGatewayMCPServer, Action: ActionCreate, Namespace: "consumer-team",
			Parent: &ParentInfo{Ref: "consumer-alias", ID: "shared-id"},
		},
		{
			ID: "other-server", ResourceType: ResourceTypeAIGatewayMCPServer, Action: ActionCreate, Namespace: "consumer-team",
			Parent: &ParentInfo{Ref: "consumer-alias", ID: "different-id"},
		},
	}}
	require.NoError(t, p.resolveFinalAIGatewayDependencies(t.Context(), plan))
	require.Contains(t, plan.Changes[2].DependsOn, "store")
	require.Contains(t, plan.Changes[2].DependsOn, "ca")
	require.Empty(t, plan.Changes[3].DependsOn)
}

func TestAIGatewayTokenVaultReferenceDoesNotCauseDrift(t *testing.T) {
	store := resources.AIGatewayDatastoreResource{
		BaseResource: resources.BaseResource{Ref: "local-store"}, AIGateway: "gateway",
		CreateAIGatewayDatastoreRequest: kkComps.CreateCreateAIGatewayDatastoreRequestTokenVault(
			kkComps.TokenVaultDatastore{Name: "api-store"},
		),
	}
	rs := &resources.ResourceSet{AIGatewayDatastores: []resources.AIGatewayDatastoreResource{store}}
	desired := map[string]any{
		FieldDatastores: []any{
			map[string]any{FieldName: tags.RefPlaceholderPrefix + "local-store#name"},
		},
	}
	changed := map[string]FieldChange{}
	require.NoError(
		t,
		addAIGatewayTokenVaultChanges(
			[]kkComps.AIGatewayDatastoreRef{{Name: "api-store"}},
			nil,
			desired,
			changed,
			rs,
			"gateway",
		),
	)
	require.Empty(t, changed)
	require.Equal(
		t,
		tags.RefPlaceholderPrefix+"local-store#name",
		desired[FieldDatastores].([]any)[0].(map[string]any)[FieldName],
	)
}

func TestAIGateway23ReferenceNormalizationAcrossAliases(t *testing.T) {
	for _, consumerID := range []string{"shared-id", "different-id", ""} {
		t.Run("consumer-id="+consumerID, func(t *testing.T) {
			platform := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: "platform"}}
			platform.SetKonnectID("shared-id")
			consumer := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: "consumer"}}
			consumer.SetKonnectID(consumerID)
			store := resources.AIGatewayDatastoreResource{
				BaseResource: resources.BaseResource{Ref: "store"}, AIGateway: "platform",
				CreateAIGatewayDatastoreRequest: kkComps.CreateCreateAIGatewayDatastoreRequestTokenVault(
					kkComps.TokenVaultDatastore{Name: "api-store"},
				),
			}
			rs := &resources.ResourceSet{
				AIGateways:          []resources.AIGatewayResource{platform, consumer},
				AIGatewayDatastores: []resources.AIGatewayDatastoreResource{store},
			}
			placeholder := tags.RefPlaceholderPrefix + "store#name"
			payload := map[string]any{FieldDatastores: []any{map[string]any{FieldName: placeholder}}}
			normalizeAIGateway23References(payload, rs, "consumer")
			want := placeholder
			if consumerID == "shared-id" {
				want = "api-store"
			}
			require.Equal(t, want, payload[FieldDatastores].([]any)[0].(map[string]any)[FieldName])
		})
	}
}

func TestAIGatewayDatastoreDeleteRejectsPlannedAliasedReference(t *testing.T) {
	platform := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: "platform"}}
	platform.SetKonnectID("shared-id")
	consumer := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: "consumer"}}
	consumer.SetKonnectID("shared-id")
	store := resources.AIGatewayDatastoreResource{
		BaseResource: resources.BaseResource{Ref: "store"}, AIGateway: "platform",
		CreateAIGatewayDatastoreRequest: kkComps.CreateCreateAIGatewayDatastoreRequestTokenVault(
			kkComps.TokenVaultDatastore{Name: "api-store"},
		),
	}
	calls := 0
	httpClient := aiGateway23UsageClient(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "/v1/ai-gateways/shared-id/datastores/store-id/usage", req.URL.Path)
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}}, Request: req,
			Body: io.NopCloser(strings.NewReader(`{"data":[],"meta":{"page":{"size":100}}}`)),
		}, nil
	})
	sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
		kkSDK.WithServerURL("https://konnect.test"), kkSDK.WithClient(httpClient),
	)}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayDatastoresAPI: sdk.GetAIGatewayDatastoresAPI()}),
		slog.Default())
	p.resources = &resources.ResourceSet{
		AIGateways:          []resources.AIGatewayResource{platform, consumer},
		AIGatewayDatastores: []resources.AIGatewayDatastoreResource{store},
	}
	plan := &Plan{Changes: []PlannedChange{
		{
			ID: "delete-store", ResourceType: ResourceTypeAIGatewayDatastore, Action: ActionDelete,
			ResourceID: "store-id", Namespace: "platform", Parent: &ParentInfo{Ref: "platform", ID: "shared-id"},
			Fields: map[string]any{FieldName: "api-store"},
		},
		{
			ID: "create-server", ResourceType: ResourceTypeAIGatewayMCPServer, Action: ActionCreate,
			Namespace: "consumer-team", Parent: &ParentInfo{Ref: "consumer", ID: "shared-id"},
			Fields: map[string]any{FieldDatastores: []any{map[string]any{
				FieldName: tags.RefPlaceholderPrefix + "store#name",
			}}},
		},
	}}
	err := p.resolveFinalAIGatewayDependencies(t.Context(), plan)
	require.ErrorContains(t, err, "still references it")
	require.Equal(t, 1, calls)
}
