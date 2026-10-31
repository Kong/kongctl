package executor

import (
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestSavedTargetDeleteRechecksLateIdentityReferences(t *testing.T) {
	for _, mode := range []string{
		"empty", "late-unowned", "failed-observation", "parent-removed", "different-server", "dry-run",
	} {
		t.Run(mode, func(t *testing.T) {
			const directory = "731559d3-39bd-48c7-b33c-eb192847eb20"
			const principal = "731559d3-39bd-48c7-b33c-eb192847eb21"
			api := &fixture.PrincipalIdentitiesAPI{}
			if mode == "failed-observation" {
				api.Error = fmt.Errorf("incomplete observation")
			}
			if mode == "parent-removed" {
				api.Error = &sdkerrors.NotFoundError{Status: 404}
			}
			if mode == "late-unowned" || mode == "different-server" || mode == "dry-run" {
				server := directory
				if mode == "different-server" {
					server = principal
				}
				api.Data = []kkComps.KongPrincipalIdentity{
					kkComps.CreateKongPrincipalIdentityAuthServerClient(
						kkComps.KongPrincipalIdentityAuthServerClientResponse{
							ID: "731559d3-39bd-48c7-b33c-eb192847eb22", AuthServerID: server, ClientID: "opaque-client",
						},
					),
				}
			}
			change := planner.PlannedChange{
				ResourceType: planner.ResourceTypeIdentityAuthServerClient,
				ResourceID:   "opaque-client", Action: planner.ActionDelete, Parent: &planner.ParentInfo{ID: directory},
				IdentityReferenceScopes: []planner.IdentityReferenceScope{
					{DirectoryID: directory, PrincipalID: principal},
				},
			}
			data, err := json.Marshal(change)
			require.NoError(t, err)
			var saved planner.PlannedChange
			require.NoError(t, json.Unmarshal(data, &saved))
			e := &Executor{
				client: state.NewClient(state.ClientConfig{IdentityPrincipalIdentitiesAPI: api}),
				dryRun: mode == "dry-run",
			}
			err = e.validateIdentityReferenceScopes(t.Context(), &saved)
			if mode == "late-unowned" || mode == "failed-observation" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if mode == "dry-run" {
				require.Empty(t, api.Requests)
			}
		})
	}
}
