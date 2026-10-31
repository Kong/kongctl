package resources

import (
	"fmt"
	"net/url"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/maturity"
)

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeIdentityAuthServer, identityAuthServerSlice, ensureIdentityAuthServerSlice,
		AutoExplain[IdentityAuthServerResource](
			WithExplainAliases("identity.auth_servers", "identity.auth-server", "auth-server", "auth-servers"),
			WithExplainRecommendedFields("ref", "name", "audience"),
			WithExplainFieldHint("audience", ExplainFieldHint{Literal: "https://api.example.com"}),
			WithExplainFieldHint("signing_algorithm", ExplainFieldHint{Default: "RS256", Literal: "RS256"}),
			WithExplainFieldHint("dcr.default_access_token_duration", ExplainFieldHint{Default: int64(300), Literal: "300"}),
		),
		ExternalResolutionRegistration{Selectors: []string{SchemaFieldName}},
		WithNamespace(220, func(a *IdentityAuthServerResource) NamespaceParticipant {
			return NamespaceParticipant{
				Ref: a.Ref, Label: string(ResourceTypeIdentityAuthServer),
				Meta: &a.Kongctl, SupportsProtected: true, External: a.IsExternal(),
			}
		}),
		WithMaturity(maturity.Metadata{
			Level:   maturity.LevelTechPreview,
			Message: "Requires the Kong Identity SDK preview and regional auth-server administration access.",
		}),
		WithRootSyncScope(),
		withCollectionValidation(240, authServerCollectionValidation),
	)
}

type IdentityAuthServerDCR struct {
	DefaultAccessTokenDuration *int64 `yaml:"default_access_token_duration,omitempty" json:"default_access_token_duration,omitempty"` //nolint:lll
}

type IdentityAuthServerResource struct {
	Scopes           []IdentityAuthServerScopeResource `yaml:"scopes,omitempty" json:"scopes,omitzero"`
	Claims           []IdentityAuthServerClaimResource `yaml:"claims,omitempty" json:"claims,omitzero"`
	BaseResource     `yaml:",inline" json:",inline"`
	Name             string                              `yaml:"name" json:"name"`
	Audience         string                              `yaml:"audience" json:"audience"`
	Description      *string                             `yaml:"description,omitempty" json:"description,omitempty"`
	SigningAlgorithm *kkComps.AuthServerSigningAlgorithm `yaml:"signing_algorithm,omitempty" json:"signing_algorithm,omitempty"` //nolint:lll
	TrustedOrigins   []string                            `yaml:"trusted_origins,omitempty" json:"trusted_origins,omitzero"`
	DCR              *IdentityAuthServerDCR              `yaml:"dcr,omitempty" json:"dcr,omitempty"`
	Labels           map[string]string                   `yaml:"labels,omitempty" json:"labels,omitzero"`
	External         *ExternalBlock                      `yaml:"_external,omitempty" json:"_external,omitempty"`
}

func identityAuthServerSlice(rs *ResourceSet) *[]IdentityAuthServerResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.AuthServers
}

func ensureIdentityAuthServerSlice(rs *ResourceSet) *[]IdentityAuthServerResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.AuthServers
}

func (a IdentityAuthServerResource) GetType() ResourceType              { return ResourceTypeIdentityAuthServer }
func (a IdentityAuthServerResource) GetMoniker() string                 { return a.Name }
func (a IdentityAuthServerResource) GetLabels() map[string]string       { return a.Labels }
func (a *IdentityAuthServerResource) SetLabels(value map[string]string) { a.Labels = value }
func (a *IdentityAuthServerResource) SetDefaults()                      {}
func (a IdentityAuthServerResource) GetDependencies() []ResourceRef     { return nil }
func (a IdentityAuthServerResource) Protected() bool                    { return a.Kongctl.IsProtected() }
func (a *IdentityAuthServerResource) GetExternalBlock() *ExternalBlock  { return a.External }
func (a *IdentityAuthServerResource) IsExternal() bool                  { return a.External != nil }
func (a IdentityAuthServerResource) GetKonnectMonikerFilter() string    { return a.Name }
func (a *IdentityAuthServerResource) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(a.Name, value, matchOptions{}, a.External)
	if ok {
		a.SetKonnectID(id)
	}
	return ok
}

func (a IdentityAuthServerResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return err
	}
	if a.IsExternal() {
		return a.External.Validate()
	}
	if a.Name == "" {
		return fmt.Errorf("auth-server name is required")
	}
	if a.Audience == "" {
		return fmt.Errorf("auth-server audience is required")
	}
	if a.SigningAlgorithm != nil && !a.SigningAlgorithm.IsExact() {
		return fmt.Errorf("unsupported auth-server signing_algorithm")
	}
	seen := make(map[string]bool)
	for _, origin := range a.TrustedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || !parsed.IsAbs() {
			return fmt.Errorf("trusted_origins entries must be absolute URIs")
		}
		if seen[origin] {
			return fmt.Errorf("trusted_origins entries must be unique")
		}
		seen[origin] = true
	}
	if a.DCR != nil && a.DCR.DefaultAccessTokenDuration != nil {
		value := *a.DCR.DefaultAccessTokenDuration
		if value < 60 || value > 2592000 {
			return fmt.Errorf("dcr.default_access_token_duration must be between 60 and 2592000 seconds")
		}
	}
	return nil
}

func authServerCollectionValidation(rs *ResourceSet) func(*IdentityAuthServerResource) error {
	seen := make(map[string]bool)
	return func(a *IdentityAuthServerResource) error {
		if err := validateCollectionResource(rs, a); err != nil {
			return err
		}
		if a.IsExternal() {
			return nil
		}
		if seen[a.Name] {
			return fmt.Errorf("duplicate auth-server name %q", a.Name)
		}
		seen[a.Name] = true
		return nil
	}
}
