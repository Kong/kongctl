package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestTrustedIDPDeletionOrdersOnlyProvenIndependentDetachments(t *testing.T) {
	const directory = "7a5e1b58-48a4-4586-b02d-0d91eccca152"
	const principal = "731559d3-39bd-48c7-b33c-eb192847eb20"
	const identity = "731559d3-39bd-48c7-b33c-eb192847eb21"
	const trust = "731559d3-39bd-48c7-b33c-eb192847eb22"
	const issuer = "https://issuer.example.test"
	for _, mode := range []string{
		"delete", "retarget", "principal-delete", "unchanged-issuer", "ownership-only",
		"wrong-directory", "unobservable", "second-identity", "new-association",
	} {
		t.Run(mode, func(t *testing.T) {
			var oidc kkComps.KongPrincipalIdentity
			require.NoError(
				t,
				json.Unmarshal(
					[]byte(
						`{"labels":{},"managed_by":{},"created_at":"2026-10-31T00:00:00Z",`+
							`"updated_at":"2026-10-31T00:00:00Z","id":"`+identity+`","type":"oidc",`+
							`"issuer":"`+issuer+`","claim":{"name":"sub","value":"staff"}}`,
					),
					&oidc,
				),
			)
			identities := &fixture.PrincipalIdentitiesAPI{Data: []kkComps.KongPrincipalIdentity{oidc}}
			trusts := &fixture.TrustedIDPsAPI{Associations: func(string, string) []helpers.IdentityPrincipal {
				if mode == "new-association" {
					return nil
				}
				return []helpers.IdentityPrincipal{{ID: principal, Active: new(false)}}
			}}
			client := state.NewClient(
				state.ClientConfig{IdentityTrustedIDPsAPI: trusts, IdentityPrincipalIdentitiesAPI: identities},
			)
			plan := &Plan{Changes: []PlannedChange{{
				ID: "trust-delete", ResourceType: ResourceTypeIdentityTrustedIDP,
				Action: ActionDelete, ResourceID: trust, Parent: &ParentInfo{ID: directory},
				Fields: map[string]any{FieldIssuerURL: issuer},
			}}}
			detach := PlannedChange{
				ID: "identity-detach", ResourceType: ResourceTypeIdentityPrincipalIdentity,
				ResourceID: identity, Action: ActionDelete, Parent: &ParentInfo{ID: principal},
				References: map[string]ReferenceInfo{FieldDirectoryID: {ID: directory}}, Namespace: "independent-owner",
			}
			switch mode {
			case "retarget":
				detach.Action = ActionUpdate
				detach.Fields = map[string]any{FieldType: "oidc", FieldIssuer: "https://other.example.test"}
			case "principal-delete":
				detach.ResourceType = ResourceTypeIdentityPrincipal
				detach.ResourceID = principal
				detach.Parent = &ParentInfo{ID: directory}
			case "unchanged-issuer":
				detach.Action = ActionUpdate
				detach.Fields = map[string]any{FieldType: "oidc", FieldIssuer: issuer}
			case "ownership-only":
				detach.Action = ActionUpdate
				detach.Fields = map[string]any{FieldManagedBy: map[string]string{"tool": "kongctl"}}
			case "wrong-directory":
				detach.References[FieldDirectoryID] = ReferenceInfo{ID: principal}
			case "unobservable":
				identities.Data = nil
			case "second-identity":
				var second kkComps.KongPrincipalIdentity
				require.NoError(
					t,
					json.Unmarshal(
						[]byte(
							`{"labels":{},"managed_by":{},"created_at":"2026-10-31T00:00:00Z",`+
								`"updated_at":"2026-10-31T00:00:00Z","id":"731559d3-39bd-48c7-b33c-eb192847eb23",`+
								`"type":"oidc","issuer":"`+issuer+`","claim":{"name":"email","value":"staff@example.test"}}`,
						),
						&second,
					),
				)
				identities.Data = append(identities.Data, second)
			case "new-association":
				detach.Action = ActionCreate
				detach.Fields = map[string]any{FieldType: "oidc", FieldIssuer: issuer}
			}
			plan.Changes = append(plan.Changes, detach)
			err := NewPlanner(client, slog.Default()).resolveIdentityTrustedIDPDeletes(t.Context(), plan)
			if mode == "delete" || mode == "retarget" || mode == "principal-delete" {
				require.NoError(t, err)
				require.Contains(t, plan.Changes[0].DependsOn, detach.ID)
			} else {
				require.Error(t, err)
			}
		})
	}
}
