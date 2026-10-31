package resources

import (
	"fmt"
	"net/url"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/kong/kongctl/internal/util"
)

type IdentityAuthServerClientResource struct {
	BaseResource            `yaml:",inline" json:",inline"`
	AuthServer              string                                     `yaml:"auth_server,omitempty" json:"auth_server,omitempty"`                               //nolint:lll
	Name                    string                                     `yaml:"name" json:"name"`                                                                 //nolint:lll
	GrantTypes              []kkComps.AuthServerGrantType              `yaml:"grant_types" json:"grant_types"`                                                   //nolint:lll
	ResponseTypes           []kkComps.ResponseType                     `yaml:"response_types" json:"response_types"`                                             //nolint:lll
	RedirectURIs            []string                                   `yaml:"redirect_uris,omitempty" json:"redirect_uris,omitzero"`                            //nolint:lll
	LoginURI                *string                                    `yaml:"login_uri,omitempty" json:"login_uri,omitempty"`                                   //nolint:lll
	AccessTokenDuration     *int64                                     `yaml:"access_token_duration,omitempty" json:"access_token_duration,omitempty"`           //nolint:lll
	IDTokenDuration         *int64                                     `yaml:"id_token_duration,omitempty" json:"id_token_duration,omitempty"`                   //nolint:lll
	AllowAllScopes          *bool                                      `yaml:"allow_all_scopes,omitempty" json:"allow_all_scopes,omitempty"`                     //nolint:lll
	AllowScopes             []string                                   `yaml:"allow_scopes,omitempty" json:"allow_scopes,omitzero"`                              //nolint:lll
	TokenEndpointAuthMethod *kkComps.AuthServerTokenEndpointAuthMethod `yaml:"token_endpoint_auth_method,omitempty" json:"token_endpoint_auth_method,omitempty"` //nolint:lll
	Labels                  map[string]string                          `yaml:"labels,omitempty" json:"labels,omitzero"`                                          //nolint:lll
}

func (c IdentityAuthServerClientResource) GetType() ResourceType {
	return ResourceTypeIdentityAuthServerClient
}
func (c IdentityAuthServerClientResource) GetMoniker() string                 { return c.Name }
func (c IdentityAuthServerClientResource) GetLabels() map[string]string       { return c.Labels }
func (c *IdentityAuthServerClientResource) SetLabels(value map[string]string) { c.Labels = value }
func (c *IdentityAuthServerClientResource) SetDefaults()                      {}
func (c IdentityAuthServerClientResource) Protected() bool                    { return c.Kongctl.IsProtected() }
func (c IdentityAuthServerClientResource) GetKonnectMonikerFilter() string    { return c.Name }
func (c *IdentityAuthServerClientResource) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(c.Name, value, matchOptions{}, nil)
	if ok {
		c.SetKonnectID(id)
	}
	return ok
}

func (c IdentityAuthServerClientResource) GetParentRef() *ResourceRef {
	if c.AuthServer == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityAuthServer, Ref: NormalizeResourceRef(c.AuthServer)}
}

func (c IdentityAuthServerClientResource) GetDependencies() []ResourceRef {
	if parent := c.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (c IdentityAuthServerClientResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"auth_server":  string(ResourceTypeIdentityAuthServer),
		"allow_scopes": string(ResourceTypeIdentityAuthServerScope),
	}
}

func (c IdentityAuthServerClientResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return err
	}
	if c.Name == "" {
		return fmt.Errorf("client name is required")
	}
	if c.GrantTypes == nil {
		return fmt.Errorf("client grant_types must be explicitly declared")
	}
	if c.ResponseTypes == nil {
		return fmt.Errorf("client response_types must be explicitly declared, including [] when empty")
	}
	seen := make(map[kkComps.AuthServerGrantType]bool)
	for _, value := range c.GrantTypes {
		if !value.IsExact() || seen[value] {
			return fmt.Errorf("client grant_types contains an unsupported or duplicate value")
		}
		seen[value] = true
	}
	for _, value := range c.ResponseTypes {
		if !value.IsExact() {
			return fmt.Errorf("client response_types contains an unsupported value")
		}
	}
	// Public SDK enum filtering omits this value, but canonical Sliver and the
	// deployed API support it. Wire tests already verify SDK string passthrough.
	if c.TokenEndpointAuthMethod != nil && !c.TokenEndpointAuthMethod.IsExact() &&
		*c.TokenEndpointAuthMethod != kkComps.AuthServerTokenEndpointAuthMethod("client_secret_basic") {
		return fmt.Errorf("unsupported client token_endpoint_auth_method")
	}
	for _, duration := range []*int64{c.AccessTokenDuration, c.IDTokenDuration} {
		if duration != nil && (*duration < 60 || *duration > 2592000) {
			return fmt.Errorf("client token durations must be between 60 and 2592000 seconds")
		}
	}
	for _, value := range c.RedirectURIs {
		if err := validateIdentityClientURI(value, false); err != nil {
			return err
		}
	}
	if c.LoginURI != nil && *c.LoginURI != "" {
		if err := validateIdentityClientURI(*c.LoginURI, true); err != nil {
			return err
		}
	}
	seenScopes := make(map[string]bool)
	for _, value := range c.AllowScopes {
		if seenScopes[value] {
			return fmt.Errorf("client allow_scopes contains duplicate entries")
		}
		seenScopes[value] = true
		if util.IsValidUUID(value) {
			continue
		}
		if _, selector, ok := tags.ParseRefPlaceholder(value); !ok || selector != "id" {
			return fmt.Errorf("client allow_scopes accepts scope UUIDs or !ref scope#id")
		}
	}
	return nil
}

func validateIdentityClientURI(value string, login bool) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || (!login && parsed.Fragment != "") {
		return fmt.Errorf(
			"client redirect/login URIs must be absolute HTTPS URLs; redirect URIs cannot contain fragments",
		)
	}
	return nil
}

func clientNamespaceParticipant(c *IdentityAuthServerClientResource) NamespaceParticipant {
	return NamespaceParticipant{
		Ref: c.Ref, Label: string(ResourceTypeIdentityAuthServerClient),
		Meta: &c.Kongctl, SupportsProtected: true,
	}
}

func identityClientSlice(rs *ResourceSet) *[]IdentityAuthServerClientResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Clients
}

func ensureIdentityClientSlice(rs *ResourceSet) *[]IdentityAuthServerClientResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Clients
}

func validateIdentityClients(rs *ResourceSet, values []IdentityAuthServerClientResource) error {
	seen := make(map[string]bool)
	for i := range values {
		c := &values[i]
		if err := validateCollectionResource(rs, c); err != nil {
			return err
		}
		if parent := identityAuthServerByRef(rs, c.AuthServer); parent == nil {
			return fmt.Errorf("client %q requires a declared managed or external auth server", c.Ref)
		}
		for _, value := range c.AllowScopes {
			ref, _, placeholder := tags.ParseRefPlaceholder(value)
			if !placeholder {
				continue
			}
			target, ok := rs.GetResourceByRef(ref)
			if !ok || target.GetType() != ResourceTypeIdentityAuthServerScope {
				return fmt.Errorf("client %q allow_scopes must reference an auth-server scope", c.Ref)
			}
			scope := target.(*IdentityAuthServerScopeResource)
			if NormalizeResourceRef(scope.AuthServer) != NormalizeResourceRef(c.AuthServer) {
				return fmt.Errorf("client %q scope reference must use the same auth server", c.Ref)
			}
		}
		key := NormalizeResourceRef(c.AuthServer) + "\x00" + c.Name
		if seen[key] {
			return fmt.Errorf("duplicate client name %q under auth server %q", c.Name, c.AuthServer)
		}
		seen[key] = true
	}
	return nil
}

func captureIdentityClientScopes(rs *ResourceSet, defaultNamespace string) {
	if rs.Identity == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	if scope.RootInScope(ResourceTypeIdentityAuthServer) {
		scope.IdentityAuthServerNamespaces = make(map[string]struct{})
		if len(rs.Identity.AuthServers) == 0 {
			scope.IdentityAuthServerNamespaces[defaultNamespace] = struct{}{}
		}
		for _, server := range rs.Identity.AuthServers {
			if !server.IsExternal() {
				scope.IdentityAuthServerNamespaces[GetNamespace(server.Kongctl)] = struct{}{}
			}
		}
	}
	for _, server := range rs.Identity.AuthServers {
		var namespaces []string
		for _, client := range server.Clients {
			namespaces = append(namespaces, GetNamespace(client.Kongctl))
		}
		CaptureOwnedChildCollection(
			scope,
			ResourceTypeIdentityAuthServerClient,
			server.Ref,
			defaultNamespace,
			scope.ChildInScope(
				ResourceTypeIdentityAuthServer,
				server.Ref,
				ResourceTypeIdentityAuthServerClient,
			),
			namespaces,
		)
	}
	for _, client := range rs.Identity.Clients {
		scope.AddChildNamespace(ResourceTypeIdentityAuthServerClient, client.AuthServer, GetNamespace(client.Kongctl))
	}
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityAuthServerClient,
		identityClientSlice,
		ensureIdentityClientSlice,
		AutoExplain[IdentityAuthServerClientResource](
			WithExplainAliases("identity.clients", "identity.auth_servers.clients", "auth-server-clients"),
			WithExplainRecommendedFields("ref", "auth_server", "name", "grant_types", "response_types"),
			WithExplainFieldHint("grant_types", ExplainFieldHint{Literal: "[client_credentials]"}),
			WithExplainFieldHint("response_types", ExplainFieldHint{Literal: "[]"}),
			WithExplainFieldHint("token_endpoint_auth_method", ExplainFieldHint{
				Default: "client_secret_post",
				Enum:    []any{"private_key_jwt", "client_secret_basic", "client_secret_post", "none"},
			}),
		),
		WithNamespace(230, clientNamespaceParticipant),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]IdentityAuthServerClientResource {
			var groups [][]IdentityAuthServerClientResource
			if rs.Identity != nil {
				for i := range rs.Identity.AuthServers {
					groups = append(groups, rs.Identity.AuthServers[i].Clients)
				}
			}
			return groups
		}, clientNamespaceParticipant),
		WithIndependentChildLabelOwnership(captureIdentityClientScopes,
			func(c *IdentityAuthServerClientResource) *KongctlMeta { return c.Kongctl }),
		WithChildSyncScope(ResourceTypeIdentityAuthServer),
		withReferenceValidation(130),
		WithExternalUnsupportedReason(
			"use an external auth-server parent and explicitly adopt the client before managing it",
		),
		WithMaturity(maturity.Metadata{Level: maturity.LevelTechPreview}),
		withChildLoad(ResourceTypeIdentityAuthServerClient, ensureIdentityClientSlice,
			childLoad[IdentityAuthServerClientResource, IdentityAuthServerResource]{
				family: ResourceTypeIdentityAuthServer, extractOrder: 30, validateOrder: 30, validatePhase: 270,
				nested:    func(s *IdentityAuthServerResource) *[]IdentityAuthServerClientResource { return &s.Clients },
				setParent: func(c *IdentityAuthServerClientResource, ref string) { c.AuthServer = ref },
				validate:  validateIdentityClients,
			}),
	)
}
