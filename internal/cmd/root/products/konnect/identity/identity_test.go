package identity

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/spf13/cobra"
)

func TestNewIdentityCmdDirectoryAliases(t *testing.T) {
	cmd, err := NewIdentityCmd(verbs.Get, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	directoryCmd, _, err := cmd.Find([]string{"directories"})
	if err != nil {
		t.Fatalf("expected directory command lookup to succeed: %v", err)
	}
	if directoryCmd == nil {
		t.Fatal("expected directory command")
	}

	for _, alias := range []string{"directories", "dir", "dirs"} {
		if !slices.Contains(directoryCmd.Aliases, alias) {
			t.Fatalf("expected alias %q in %v", alias, directoryCmd.Aliases)
		}
	}
}

func TestNewIdentityCmdPrincipalChildCommands(t *testing.T) {
	cmd, err := NewIdentityCmd(verbs.Get, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	principalsCmd, _, err := cmd.Find([]string{"directory", "principals"})
	if err != nil {
		t.Fatalf("expected principals command lookup to succeed: %v", err)
	}
	if principalsCmd == nil {
		t.Fatal("expected principals command")
	}
	if !slices.Contains(principalsCmd.Aliases, "principal") {
		t.Fatalf("expected principal alias in %v", principalsCmd.Aliases)
	}

	identitiesCmd, _, err := cmd.Find([]string{"directory", "principals", "identities"})
	if err != nil {
		t.Fatalf("expected identities command lookup to succeed: %v", err)
	}
	if identitiesCmd == nil {
		t.Fatal("expected identities command")
	}
	if !slices.Contains(identitiesCmd.Aliases, "identity") {
		t.Fatalf("expected identity alias in %v", identitiesCmd.Aliases)
	}
}

func TestDirectoryDetailViewIncludesRealmConfig(t *testing.T) {
	allowAll := true
	ttl := int64(300)
	realmTTL := int64(10)
	directory := directoryResource{
		ID:                    "d67a4203-b1e8-4631-a626-5fe7c55efe88",
		Name:                  "workforce",
		Description:           "Workforce identities",
		AllowedControlPlanes:  []string{"cp-1", "cp-2"},
		AllowAllControlPlanes: &allowAll,
		TTLSecs:               &ttl,
		Labels:                map[string]string{"env": "test"},
		RealmConfig: &directoryRealmConfig{
			TTL:            &realmTTL,
			ConsumerGroups: []string{"employees"},
		},
	}

	detail := directoryDetailView(directory)
	for _, expected := range []string{
		"name: workforce",
		"allowed_control_planes: cp-1, cp-2",
		"allow_all_control_planes: true",
		"ttl_secs: 300",
		"labels: env=test",
		"realm_config.ttl: 10",
		"realm_config.consumer_groups: employees",
	} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("expected detail to contain %q, got:\n%s", expected, detail)
		}
	}
}

func TestNormalizePrincipalMetadata(t *testing.T) {
	displayName := "duplicate-display-name-test"
	provisionedBy := kkComps.ProvisionedByKonnectUser
	createdAt := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	principal := normalizePrincipal("dir-1", kkComps.KongPrincipal{
		ID:          "principal-1",
		DisplayName: &displayName,
		Description: "principal",
		Metadata: map[string]kkComps.KongIdentityMetadata{
			"active": kkComps.CreateKongIdentityMetadataBoolean(true),
			"count":  kkComps.CreateKongIdentityMetadataInteger(2),
			"group":  kkComps.CreateKongIdentityMetadataStr("engineering"),
			"groups": kkComps.CreateKongIdentityMetadataArrayOfStr([]string{"engineering", "platform"}),
			"ids":    kkComps.CreateKongIdentityMetadataArrayOfInteger([]int64{1, 2}),
		},
		Labels:        map[string]string{"env": "test"},
		ManagedBy:     map[string]string{"tool": "kongctl"},
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
		ProvisionedBy: &provisionedBy,
	})

	if principal.DirectoryID != "dir-1" {
		t.Fatalf("expected directory ID dir-1, got %q", principal.DirectoryID)
	}
	if principal.DisplayName == nil || *principal.DisplayName != displayName {
		t.Fatalf("expected display name %q, got %v", displayName, principal.DisplayName)
	}
	if principal.Metadata["active"] != true {
		t.Fatalf("expected boolean metadata, got %v", principal.Metadata["active"])
	}
	if principal.Metadata["count"] != int64(2) {
		t.Fatalf("expected integer metadata, got %v", principal.Metadata["count"])
	}
	if principal.Metadata["group"] != "engineering" {
		t.Fatalf("expected string metadata, got %v", principal.Metadata["group"])
	}
}

func TestNormalizePrincipalIdentityUnionTypes(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	workspaceID := "workspace-1"

	tests := []struct {
		name     string
		identity kkComps.KongPrincipalIdentity
		assert   func(t *testing.T, got principalIdentityResource)
	}{
		{
			name: "oidc",
			identity: kkComps.CreateKongPrincipalIdentityOidc(kkComps.KongPrincipalIdentityOIDCResponse{
				ID:        "identity-1",
				Type:      kkComps.KongPrincipalIdentityOIDCResponseTypeOidc,
				Issuer:    "https://issuer.example",
				Claim:     kkComps.Claim{Name: "sub", Value: "user-1"},
				Labels:    map[string]string{"type": "oidc"},
				CreatedAt: now,
				UpdatedAt: now,
			}),
			assert: func(t *testing.T, got principalIdentityResource) {
				if got.Type != "oidc" || got.Issuer != "https://issuer.example" ||
					got.ClaimName != "sub" || got.ClaimValue != "user-1" {
					t.Fatalf("unexpected oidc identity: %+v", got)
				}
			},
		},
		{
			name: "auth_server_client",
			identity: kkComps.CreateKongPrincipalIdentityAuthServerClient(
				kkComps.KongPrincipalIdentityAuthServerClientResponse{
					ID:           "identity-2",
					Type:         kkComps.KongPrincipalIdentityAuthServerClientResponseTypeAuthServerClient,
					AuthServerID: "as-1",
					ClientID:     "client-1",
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			),
			assert: func(t *testing.T, got principalIdentityResource) {
				if got.Type != "auth_server_client" || got.AuthServerID != "as-1" || got.ClientID != "client-1" {
					t.Fatalf("unexpected auth server client identity: %+v", got)
				}
			},
		},
		{
			name: "control_plane_consumer",
			identity: kkComps.CreateKongPrincipalIdentityControlPlaneConsumer(
				kkComps.KongPrincipalIdentityCPConsumerResponse{
					ID:             "identity-3",
					Type:           kkComps.KongPrincipalIdentityCPConsumerResponseTypeControlPlaneConsumer,
					ControlPlaneID: "cp-1",
					ConsumerID:     "consumer-1",
					WorkspaceID:    &workspaceID,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
			),
			assert: func(t *testing.T, got principalIdentityResource) {
				if got.Type != "control_plane_consumer" || got.ControlPlaneID != "cp-1" ||
					got.ConsumerID != "consumer-1" || got.WorkspaceID == nil || *got.WorkspaceID != workspaceID {
					t.Fatalf("unexpected control plane consumer identity: %+v", got)
				}
			},
		},
		{
			name: "custom",
			identity: kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
				ID:        "identity-4",
				Type:      kkComps.KongPrincipalIdentityCustomResponseTypeCustom,
				Key:       "external_id",
				Value:     "user-1",
				CreatedAt: now,
				UpdatedAt: now,
			}),
			assert: func(t *testing.T, got principalIdentityResource) {
				if got.Type != "custom" || got.Key != "external_id" || got.Value != "user-1" {
					t.Fatalf("unexpected custom identity: %+v", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePrincipalIdentity("dir-1", "principal-1", tt.identity)
			if got.DirectoryID != "dir-1" || got.PrincipalID != "principal-1" {
				t.Fatalf("unexpected parent identifiers: %+v", got)
			}
			if got.ID == "" {
				t.Fatalf("expected identity ID, got %+v", got)
			}
			if got.CreatedAt == nil || got.UpdatedAt == nil {
				t.Fatalf("expected timestamps, got %+v", got)
			}
			tt.assert(t, got)
		})
	}
}

func TestResolveIdentityDirectoryIDDefaultsToOnlyDirectory(t *testing.T) {
	helper := newIdentityDirectoryResolutionHelper(t)
	api := &stubIdentityDirectoryAPI{
		directories: []kkComps.KongDirectory{{
			ID:   "dir-1",
			Name: "workforce",
		}},
	}

	got, err := resolveIdentityDirectoryID(helper, api, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "dir-1" {
		t.Fatalf("expected dir-1, got %q", got)
	}
	if api.listPageSize == nil || *api.listPageSize != 2 {
		t.Fatalf("expected directory lookup page size 2, got %v", api.listPageSize)
	}
}

func TestResolveIdentityDirectoryIDRequiresExplicitDirectoryWhenNoneExist(t *testing.T) {
	helper := newIdentityDirectoryResolutionHelper(t)
	api := &stubIdentityDirectoryAPI{}

	_, err := resolveIdentityDirectoryID(helper, api, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, expected := range []string{
		"a directory identifier is required because no identity directories were found",
		"--directory-id",
		"--directory-name",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %q, got %q", expected, err.Error())
		}
	}
	if api.listPageSize == nil || *api.listPageSize != 2 {
		t.Fatalf("expected directory lookup page size 2, got %v", api.listPageSize)
	}
}

func TestResolveIdentityDirectoryIDRequiresExplicitDirectoryWhenMultipleExist(t *testing.T) {
	helper := newIdentityDirectoryResolutionHelper(t)
	api := &stubIdentityDirectoryAPI{
		directories: []kkComps.KongDirectory{
			{ID: "dir-1", Name: "workforce"},
			{ID: "dir-2", Name: "contractors"},
		},
	}

	_, err := resolveIdentityDirectoryID(helper, api, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, expected := range []string{
		"a directory identifier is required because multiple identity directories exist",
		"--directory-id",
		"--directory-name",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %q, got %q", expected, err.Error())
		}
	}
	if api.listPageSize == nil || *api.listPageSize != 2 {
		t.Fatalf("expected directory lookup page size 2, got %v", api.listPageSize)
	}
}

func TestDirectoryAdoptLabelsPreserveExistingLabels(t *testing.T) {
	result := directoryAdoptLabels(map[string]string{
		"team":              "platform",
		labels.ProtectedKey: labels.TrueValue,
	}, "identity")

	if result["team"] != "platform" {
		t.Fatalf("expected user label to be preserved, got %v", result)
	}
	if result[labels.ProtectedKey] != labels.TrueValue {
		t.Fatalf("expected protected label to be preserved, got %v", result)
	}
	if result[labels.NamespaceKey] != "identity" {
		t.Fatalf("expected namespace label to be set, got %v", result)
	}
}

func newIdentityDirectoryResolutionHelper(t *testing.T) cmdpkg.Helper {
	t.Helper()

	c := &cobra.Command{Use: "principals"}
	c.SetContext(context.Background())
	addIdentityDirectoryScopeFlags(c)

	return cmdpkg.BuildHelper(c, nil)
}

type stubIdentityDirectoryAPI struct {
	directories  []kkComps.KongDirectory
	listPageSize *int64
}

func (s *stubIdentityDirectoryAPI) ListKongDirectories(
	_ context.Context,
	page *kkComps.CursorPageParameters,
	_ *string,
	_ ...kkOps.Option,
) (*kkOps.ListKongDirectoriesResponse, error) {
	if page != nil && page.Size != nil {
		size := *page.Size
		s.listPageSize = &size
	}

	return &kkOps.ListKongDirectoriesResponse{
		ListKongDirectories: &kkComps.ListKongDirectories{Data: s.directories},
	}, nil
}

func (s *stubIdentityDirectoryAPI) CreateDirectory(
	_ context.Context,
	_ kkComps.CreateDirectoryBody,
	_ ...kkOps.Option,
) (*kkOps.CreateDirectoryResponse, error) {
	return nil, nil
}

func (s *stubIdentityDirectoryAPI) ReplaceDirectory(
	_ context.Context,
	_ string,
	_ kkComps.ReplaceDirectoryBody,
	_ ...kkOps.Option,
) (*kkOps.ReplaceDirectoryResponse, error) {
	return nil, nil
}

func (s *stubIdentityDirectoryAPI) GetDirectory(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetDirectoryResponse, error) {
	return nil, nil
}

func (s *stubIdentityDirectoryAPI) DeleteDirectory(
	_ context.Context,
	_ string,
	_ *kkOps.DeleteDirectoryQueryParamForce,
	_ ...kkOps.Option,
) (*kkOps.DeleteDirectoryResponse, error) {
	return nil, nil
}

func (s *stubIdentityDirectoryAPI) GetRealmConfig(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetRealmConfigResponse, error) {
	return nil, nil
}
