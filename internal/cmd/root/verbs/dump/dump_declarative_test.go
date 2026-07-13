package dump

import (
	"context"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"sigs.k8s.io/yaml"
)

func requireKongctlMeta(
	t *testing.T,
	meta *declresources.KongctlMeta,
	wantNamespace string,
	wantProtected bool,
) {
	t.Helper()

	if meta == nil {
		t.Fatalf("expected kongctl metadata")
	}

	if wantNamespace == "" {
		if meta.Namespace != nil {
			t.Fatalf("expected namespace to be omitted, got %q", *meta.Namespace)
		}
	} else if meta.Namespace == nil || *meta.Namespace != wantNamespace {
		got := "<nil>"
		if meta.Namespace != nil {
			got = *meta.Namespace
		}
		t.Fatalf("expected namespace %q, got %q", wantNamespace, got)
	}

	if wantProtected {
		if meta.Protected == nil || !*meta.Protected {
			t.Fatalf("expected protected metadata to be true")
		}
	} else if meta.Protected != nil && *meta.Protected {
		t.Fatalf("expected protected metadata to be omitted or false")
	}
}

func TestMapPortalToDeclarativeResource(t *testing.T) {
	description := "Portal description"
	authID := "auth-strategy"

	portal := kkComps.ListPortalsResponsePortal{
		ID:                               "portal-id",
		Name:                             "portal-name",
		DisplayName:                      "Portal Display",
		DefaultAPIVisibility:             kkComps.ListPortalsResponseDefaultAPIVisibilityPrivate,
		DefaultPageVisibility:            kkComps.ListPortalsResponseDefaultPageVisibilityPublic,
		DefaultApplicationAuthStrategyID: &authID,
		Labels: map[string]string{
			decllabels.NamespaceKey: "team-alpha",
			decllabels.ProtectedKey: decllabels.TrueValue,
			"custom":                "value",
		},
	}
	portal.Description = &description
	portal.AuthenticationEnabled = true
	portal.RbacEnabled = true
	portal.AutoApproveDevelopers = true
	portal.AutoApproveApplications = false

	resource := mapPortalToDeclarativeResource(portal)

	if resource.Ref != portal.ID {
		t.Fatalf("expected ref %q, got %q", portal.ID, resource.Ref)
	}

	if resource.Name != portal.Name {
		t.Fatalf("expected name %q, got %q", portal.Name, resource.Name)
	}

	if resource.DisplayName == nil || *resource.DisplayName != portal.DisplayName {
		t.Fatalf("expected display name pointer with %q", portal.DisplayName)
	}

	if resource.Description == nil || *resource.Description != description {
		t.Fatalf("expected description pointer with %q", description)
	}

	requireKongctlMeta(t, resource.Kongctl, "team-alpha", true)

	if resource.Labels == nil {
		t.Fatalf("expected user labels to be preserved")
	}

	if len(resource.Labels) != 1 {
		t.Fatalf("expected only user labels to remain, got %v", resource.Labels)
	}

	if _, exists := resource.Labels[decllabels.NamespaceKey]; exists {
		t.Fatalf("expected namespace label to be stripped from user labels")
	}

	if _, exists := resource.Labels[decllabels.ProtectedKey]; exists {
		t.Fatalf("expected protected label to be stripped from user labels")
	}

	if val, exists := resource.Labels["custom"]; !exists || val == nil || *val != "value" {
		t.Fatalf("expected custom label to be preserved, got %v", resource.Labels)
	}

	if resource.DefaultAPIVisibility == nil || *resource.DefaultAPIVisibility != kkComps.DefaultAPIVisibilityPrivate {
		t.Fatalf("expected default API visibility private, got %+v", resource.DefaultAPIVisibility)
	}

	if resource.DefaultPageVisibility == nil || *resource.DefaultPageVisibility != kkComps.DefaultPageVisibilityPublic {
		t.Fatalf("expected default page visibility public, got %+v", resource.DefaultPageVisibility)
	}

	if resource.AutoApproveDevelopers == nil || !*resource.AutoApproveDevelopers {
		t.Fatalf("expected auto approve developers to be true")
	}

	if resource.AutoApproveApplications == nil {
		t.Fatalf("expected auto approve applications pointer to be set")
	}

	if *resource.AutoApproveApplications != portal.AutoApproveApplications {
		t.Fatalf("expected auto approve applications to match input")
	}
}

func TestMapEventGatewayToDeclarativeResourcePreservesMinRuntimeVersion(t *testing.T) {
	resource := mapEventGatewayToDeclarativeResource(kkComps.EventGatewayInfo{
		ID:                "event-gateway-id",
		Name:              "event-gateway",
		MinRuntimeVersion: "1.2",
	})

	if resource.MinRuntimeVersion == nil || *resource.MinRuntimeVersion != "1.2" {
		t.Fatalf("expected min_runtime_version to be preserved, got %#v", resource.MinRuntimeVersion)
	}
}

func TestCollectDeclarativePortalsUsesPortalDetails(t *testing.T) {
	listAuthEnabled := false
	detailAuthEnabled := true
	var getPortalIDs []string

	api := &portalPaginationStub{
		t: t,
		listPortalsFunc: func(
			_ context.Context,
			req kkOps.ListPortalsRequest,
		) (*kkOps.ListPortalsResponse, error) {
			pageNumber := int64(1)
			if req.PageNumber != nil {
				pageNumber = *req.PageNumber
			}

			switch pageNumber {
			case 1:
				return &kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.ListPortalsResponsePortal{
							{
								ID:                    "portal-id",
								Name:                  "portal-name",
								AuthenticationEnabled: listAuthEnabled,
							},
						},
					},
				}, nil
			case 2:
				return &kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{},
				}, nil
			default:
				t.Fatalf("unexpected page request: %d", pageNumber)
				return nil, nil
			}
		},
		getPortalFunc: func(_ context.Context, id string) (*kkOps.GetPortalResponse, error) {
			getPortalIDs = append(getPortalIDs, id)
			return &kkOps.GetPortalResponse{
				PortalResponse: &kkComps.PortalResponse{
					ID:                    id,
					Name:                  "portal-name",
					AuthenticationEnabled: detailAuthEnabled,
				},
			}, nil
		},
	}

	resources, err := collectDeclarativePortals(t.Context(), api, 100, filterOptions{})
	if err != nil {
		t.Fatalf("unexpected error collecting portals: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected one portal resource, got %d", len(resources))
	}

	if resources[0].AuthenticationEnabled == nil || !*resources[0].AuthenticationEnabled {
		t.Fatalf("expected authentication_enabled to come from portal detail, got %+v",
			resources[0].AuthenticationEnabled)
	}

	if len(getPortalIDs) != 1 || getPortalIDs[0] != "portal-id" {
		t.Fatalf("expected GetPortal to be called once for portal-id, got %v", getPortalIDs)
	}
}

func TestMapAPIToDeclarativeResource(t *testing.T) {
	description := "API description"
	version := "v1"
	slug := "api-slug"

	api := kkComps.APIResponseSchema{
		ID:          "api-id",
		Name:        "api-name",
		Description: &description,
		Version:     &version,
		Slug:        &slug,
		Labels: map[string]string{
			decllabels.NamespaceKey: "team-beta",
			decllabels.ProtectedKey: decllabels.TrueValue,
			"feature":               "payments",
		},
	}

	resource := mapAPIToDeclarativeResource(api)

	if resource.Ref != api.ID {
		t.Fatalf("expected ref %q, got %q", api.ID, resource.Ref)
	}

	if resource.Name != api.Name {
		t.Fatalf("expected name %q, got %q", api.Name, resource.Name)
	}

	if resource.Description == nil || *resource.Description != description {
		t.Fatalf("expected description pointer with %q", description)
	}

	if resource.Version == nil || *resource.Version != version {
		t.Fatalf("expected version pointer with %q", version)
	}

	if resource.Slug == nil || *resource.Slug != slug {
		t.Fatalf("expected slug pointer with %q", slug)
	}

	requireKongctlMeta(t, resource.Kongctl, "team-beta", true)

	if len(resource.Labels) != 1 || resource.Labels["feature"] != "payments" {
		t.Fatalf("expected only user labels to remain, got %v", resource.Labels)
	}
}

func TestMapDashboardToDeclarativeResource(t *testing.T) {
	id := "dashboard-id"
	dashboard := kkComps.DashboardResponse{
		ID:   &id,
		Name: "API Summary",
		Definition: kkComps.Dashboard{
			Tiles: []kkComps.Tile{},
		},
		Labels: map[string]string{
			decllabels.NamespaceKey: "team-alpha",
			decllabels.ProtectedKey: decllabels.TrueValue,
			"team":                  "platform",
		},
	}

	resource := mapDashboardToDeclarativeResource(dashboard)

	if resource.Ref != id {
		t.Fatalf("expected ref %q, got %q", id, resource.Ref)
	}

	if resource.Name != dashboard.Name {
		t.Fatalf("expected name %q, got %q", dashboard.Name, resource.Name)
	}

	if resource.Definition.Tiles == nil || len(resource.Definition.Tiles) != 0 {
		t.Fatalf("expected definition tiles to be preserved, got %v", resource.Definition.Tiles)
	}

	requireKongctlMeta(t, resource.Kongctl, "team-alpha", true)

	if len(resource.Labels) != 1 || resource.Labels["team"] != "platform" {
		t.Fatalf("expected only user labels to remain, got %v", resource.Labels)
	}
}

func TestMapAIGatewayToDeclarativeResource(t *testing.T) {
	description := "AI Gateway description"
	gateway := kkComps.AIGateway{
		ID:          "ai-gateway-id",
		DisplayName: "AI Gateway",
		Name:        "sdk-name",
		Description: &description,
		ProxyUrls: []kkComps.AIGatewayProxyURL{
			{Host: "proxy.example.com", Port: 443, Protocol: "https"},
		},
		Labels: map[string]string{
			decllabels.NamespaceKey: "team-ai",
			decllabels.ProtectedKey: decllabels.TrueValue,
			"owner":                 "platform",
		},
	}

	resource := mapAIGatewayToDeclarativeResource(gateway)

	if resource.Ref != gateway.ID {
		t.Fatalf("expected ref %q, got %q", gateway.ID, resource.Ref)
	}
	if resource.DisplayName != gateway.DisplayName {
		t.Fatalf("expected display name %q, got %q", gateway.DisplayName, resource.DisplayName)
	}
	if resource.Name != gateway.Name {
		t.Fatalf("expected name %q, got %q", gateway.Name, resource.Name)
	}
	if resource.Description == nil || *resource.Description != description {
		t.Fatalf("expected description pointer with %q", description)
	}
	if len(resource.ProxyUrls) != 1 || resource.ProxyUrls[0].Host != "proxy.example.com" {
		t.Fatalf("expected proxy URL to be preserved, got %v", resource.ProxyUrls)
	}
	if len(resource.Labels) != 1 || resource.Labels["owner"] != "platform" {
		t.Fatalf("expected only user labels to remain, got %v", resource.Labels)
	}
	if resource.Kongctl == nil || resource.Kongctl.Namespace == nil || *resource.Kongctl.Namespace != "team-ai" {
		t.Fatalf("expected namespace metadata to be preserved, got %#v", resource.Kongctl)
	}
	if resource.Kongctl.Protected == nil || !*resource.Kongctl.Protected {
		t.Fatalf("expected protected metadata to be preserved, got %#v", resource.Kongctl)
	}
}

func TestBuildDeclarativeDefaults(t *testing.T) {
	if buildDeclarativeDefaults("") != nil {
		t.Fatalf("expected nil defaults when namespace is empty")
	}

	defaults := buildDeclarativeDefaults("shared")
	if defaults == nil || defaults.Kongctl == nil || defaults.Kongctl.Namespace == nil {
		t.Fatalf("expected defaults to include kongctl namespace")
	}

	if *defaults.Kongctl.Namespace != "shared" {
		t.Fatalf("expected namespace 'shared', got %q", *defaults.Kongctl.Namespace)
	}
}

func TestMapAuthStrategyToDeclarativeResource_KeyAuth(t *testing.T) {
	strategyID := "key-auth-id"
	strategy := kkComps.CreateAppAuthStrategyKeyAuth(
		kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse{
			ID:          strategyID,
			Name:        "key-auth",
			DisplayName: "Key Auth",
			Configs: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyConfigs{
				KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{KeyNames: []string{"apikey"}},
			},
			Labels: map[string]string{
				decllabels.NamespaceKey: "default",
				"tier":                  "gold",
			},
		},
	)

	resource, err := mapAuthStrategyToDeclarativeResource(strategy)
	if err != nil {
		t.Fatalf("unexpected error mapping key auth strategy: %v", err)
	}

	if resource.Ref != strategyID {
		t.Fatalf("expected ref %q, got %q", strategyID, resource.Ref)
	}

	if resource.Type != kkComps.CreateAppAuthStrategyRequestTypeKeyAuth {
		t.Fatalf("expected type key_auth, got %s", resource.Type)
	}

	if resource.AppAuthStrategyKeyAuthRequest == nil {
		t.Fatalf("expected key auth request to be populated")
	}

	if resource.AppAuthStrategyKeyAuthRequest.Name != "key-auth" {
		t.Fatalf("expected name to be preserved")
	}

	configs := resource.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames
	if len(configs) != 1 || configs[0] != "apikey" {
		t.Fatalf("expected key names to be preserved, got %v", configs)
	}

	requireKongctlMeta(t, resource.Kongctl, "default", false)

	labels := resource.AppAuthStrategyKeyAuthRequest.Labels
	if len(labels) != 1 || labels["tier"] != "gold" {
		t.Fatalf("expected only user labels to remain, got %v", labels)
	}

	if _, exists := resource.AppAuthStrategyKeyAuthRequest.Labels[decllabels.NamespaceKey]; exists {
		t.Fatalf("expected namespace label to be stripped")
	}

	yamlBytes, err := yaml.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal auth strategy to yaml: %v", err)
	}

	if !strings.Contains(string(yamlBytes), "ref: "+strategyID) {
		t.Fatalf("expected yaml to include ref %q, got:\n%s", strategyID, string(yamlBytes))
	}
}

func TestMapAuthStrategyToDeclarativeResource_OIDC(t *testing.T) {
	strategyID := "oidc-id"
	strategy := kkComps.CreateAppAuthStrategyOpenidConnect(
		kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse{
			ID:          strategyID,
			Name:        "oidc",
			DisplayName: "OIDC",
			Configs: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyConfigs{
				OpenidConnect: kkComps.AppAuthStrategyConfigOpenIDConnect{
					Issuer:          "https://issuer.example.com",
					CredentialClaim: []string{"sub"},
				},
			},
			DcrProvider: &kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyDcrProvider{ID: "provider-123"},
			Labels: map[string]string{
				decllabels.NamespaceKey: "default",
				"team":                  "identity",
			},
		},
	)

	resource, err := mapAuthStrategyToDeclarativeResource(strategy)
	if err != nil {
		t.Fatalf("unexpected error mapping oidc strategy: %v", err)
	}

	if resource.Ref != strategyID {
		t.Fatalf("expected ref %q, got %q", strategyID, resource.Ref)
	}

	if resource.Type != kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect {
		t.Fatalf("expected type openid_connect, got %s", resource.Type)
	}

	if resource.AppAuthStrategyOpenIDConnectRequest == nil {
		t.Fatalf("expected oidc request to be populated")
	}

	if resource.AppAuthStrategyOpenIDConnectRequest.Name != "oidc" {
		t.Fatalf("expected name to be preserved")
	}

	providerID := resource.AppAuthStrategyOpenIDConnectRequest.DcrProviderID
	if providerID == nil || *providerID != "provider-123" {
		t.Fatalf("expected DCR provider ID to be preserved")
	}

	labels := resource.AppAuthStrategyOpenIDConnectRequest.Labels
	if len(labels) != 1 || labels["team"] != "identity" {
		t.Fatalf("expected only user labels to remain, got %v", labels)
	}

	requireKongctlMeta(t, resource.Kongctl, "default", false)

	yamlBytes, err := yaml.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal oidc strategy to yaml: %v", err)
	}

	if !strings.Contains(string(yamlBytes), "ref: "+strategyID) {
		t.Fatalf("expected yaml to include ref %q, got:\n%s", strategyID, string(yamlBytes))
	}
}

func TestValidateFilterOptions(t *testing.T) {
	tests := []struct {
		name    string
		filter  filterOptions
		wantErr bool
	}{
		{name: "no filter", filter: filterOptions{}, wantErr: false},
		{name: "name only", filter: filterOptions{name: "my-portal"}, wantErr: false},
		{name: "id only", filter: filterOptions{id: "abc-123"}, wantErr: false},
		{name: "both set", filter: filterOptions{name: "x", id: "y"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilterOptions(tt.filter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateFilterOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseFilterName(t *testing.T) {
	tests := []struct {
		input  string
		wantOp string
		wantV  string
	}{
		{input: "my-portal", wantOp: "eq", wantV: "my-portal"},
		{input: "*portal*", wantOp: "contains", wantV: "portal"},
		{input: "*portal", wantOp: "contains", wantV: "portal"},
		{input: "portal*", wantOp: "contains", wantV: "portal"},
		{input: "***portal***", wantOp: "contains", wantV: "portal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			op, val := parseFilterName(tt.input)
			if op != tt.wantOp || val != tt.wantV {
				t.Fatalf("parseFilterName(%q) = (%q, %q), want (%q, %q)",
					tt.input, op, val, tt.wantOp, tt.wantV)
			}
		})
	}
}

func TestFilterOptionsHasFilter(t *testing.T) {
	if (filterOptions{}).hasFilter() {
		t.Fatal("empty filter should return false")
	}
	if !(filterOptions{name: "x"}).hasFilter() {
		t.Fatal("filter with name should return true")
	}
	if !(filterOptions{id: "x"}).hasFilter() {
		t.Fatal("filter with id should return true")
	}
}

func TestNormalizeResourceListMapsDashboardAliasesToAnalyticsDashboards(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "dashboard", want: []string{"analytics.dashboards"}},
		{input: "dashboards", want: []string{"analytics.dashboards"}},
		{input: "analytics.dashboard", want: []string{"analytics.dashboards"}},
		{input: "analytics.dashboards", want: []string{"analytics.dashboards"}},
		{input: "organization.teams,dashboards", want: []string{"organization.teams", "analytics.dashboards"}},
		{input: "ai-gateway", want: []string{"ai_gateways"}},
		{input: "ai-gateways", want: []string{"ai_gateways"}},
		{input: "aigw", want: []string{"ai_gateways"}},
		{input: "ai-gateway-model-provider", want: []string{"ai_gateway_model_providers"}},
		{input: "ai_gateway_model_provider", want: []string{"ai_gateway_model_providers"}},
		{input: "ai_gateway_model_providers", want: []string{"ai_gateway_model_providers"}},
		{input: "ai_gateways,analytics.dashboards", want: []string{"ai_gateways", "analytics.dashboards"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeResourceList(tt.input, declarativeAllowedResources)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestFilterByNameOrID(t *testing.T) {
	portals := []declresources.PortalResource{
		{BaseResource: declresources.BaseResource{Ref: "id-1"}, CreatePortal: kkComps.CreatePortal{Name: "alpha"}},
		{BaseResource: declresources.BaseResource{Ref: "id-2"}, CreatePortal: kkComps.CreatePortal{Name: "beta"}},
		{BaseResource: declresources.BaseResource{Ref: "id-3"}, CreatePortal: kkComps.CreatePortal{Name: "alpha-dev"}},
	}
	accessor := func(p declresources.PortalResource) (string, string) { return p.Name, p.Ref }

	t.Run("no filter", func(t *testing.T) {
		result := filterByNameOrID(portals, filterOptions{}, accessor)
		if len(result) != 3 {
			t.Fatalf("expected 3 results, got %d", len(result))
		}
	})

	t.Run("exact name match", func(t *testing.T) {
		result := filterByNameOrID(portals, filterOptions{name: "alpha"}, accessor)
		if len(result) != 1 || result[0].Name != "alpha" {
			t.Fatalf("expected [alpha], got %v", result)
		}
	})

	t.Run("contains name match", func(t *testing.T) {
		result := filterByNameOrID(portals, filterOptions{name: "*alpha*"}, accessor)
		if len(result) != 2 {
			t.Fatalf("expected 2 results for *alpha*, got %d", len(result))
		}
	})

	t.Run("id match", func(t *testing.T) {
		result := filterByNameOrID(portals, filterOptions{id: "id-2"}, accessor)
		if len(result) != 1 || result[0].Name != "beta" {
			t.Fatalf("expected [beta], got %v", result)
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := filterByNameOrID(portals, filterOptions{name: "gamma"}, accessor)
		if len(result) != 0 {
			t.Fatalf("expected 0 results, got %d", len(result))
		}
	})
}
