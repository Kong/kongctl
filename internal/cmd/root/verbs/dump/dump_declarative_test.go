package dump

import (
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	"sigs.k8s.io/yaml"
)

func TestMapPortalToDeclarativeResource(t *testing.T) {
	description := "Portal description"
	authID := "auth-strategy"

	portal := kkComps.Portal{
		ID:                               "portal-id",
		Name:                             "portal-name",
		DisplayName:                      "Portal Display",
		Description:                      &description,
		AuthenticationEnabled:            true,
		RbacEnabled:                      true,
		DefaultAPIVisibility:             kkComps.ListPortalsResponseDefaultAPIVisibilityPrivate,
		DefaultPageVisibility:            kkComps.ListPortalsResponseDefaultPageVisibilityPublic,
		DefaultApplicationAuthStrategyID: &authID,
		AutoApproveDevelopers:            true,
		AutoApproveApplications:          false,
		Labels: map[string]string{
			decllabels.NamespaceKey: "team-alpha",
			decllabels.ProtectedKey: decllabels.TrueValue,
			"custom":                "value",
		},
	}

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

	if resource.Kongctl != nil {
		t.Fatalf("expected kongctl metadata to be omitted when namespace flag not provided")
	}

	if resource.Labels == nil {
		t.Fatalf("expected user labels to be preserved")
	}

	if _, exists := resource.Labels[decllabels.NamespaceKey]; exists {
		t.Fatalf("expected namespace label to be stripped from user labels")
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

	if resource.Kongctl != nil {
		t.Fatalf("expected kongctl metadata to be omitted when namespace flag not provided")
	}

	if len(resource.Labels) != 1 || resource.Labels["feature"] != "payments" {
		t.Fatalf("expected only user labels to remain, got %v", resource.Labels)
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

	if resource.Kongctl != nil {
		t.Fatalf("expected kongctl metadata to be omitted for key auth strategy")
	}

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

	if resource.Kongctl != nil {
		t.Fatalf("expected kongctl metadata to be omitted for oidc strategy")
	}

	yamlBytes, err := yaml.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal oidc strategy to yaml: %v", err)
	}

	if !strings.Contains(string(yamlBytes), "ref: "+strategyID) {
		t.Fatalf("expected yaml to include ref %q, got:\n%s", strategyID, string(yamlBytes))
	}
}
