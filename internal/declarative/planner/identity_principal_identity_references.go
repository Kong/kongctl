package planner

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func (p *Planner) resolvePrincipalIdentityTargetDeletes(plan *Plan) error {
	for idx := range plan.Changes {
		target := &plan.Changes[idx]
		if target.Action != ActionDelete {
			continue
		}
		switch target.ResourceType {
		case ResourceTypeIdentityAuthServer, ResourceTypeIdentityAuthServerClient, ResourceTypeControlPlane,
			ResourceTypeAIGateway, ResourceTypeAIGatewayConsumer:
		default:
			continue
		}
		parentID := ""
		if target.Parent != nil {
			parentID = target.Parent.ID
		}
		uses := func(identity resources.IdentityPrincipalIdentityResource) bool {
			return identity.ReferencesTarget(resources.ResourceType(target.ResourceType), target.ResourceID, parentID)
		}
		if target.ResourceType == ResourceTypeIdentityAuthServerClient {
			plan.AddWarning(target.ID, "Deleting an OAuth client can remove its principal identity associations. "+
				"Associations outside the declared directory/principal collections are not inspected.")
		}
		for _, key := range slices.Sorted(maps.Keys(p.resourceCache.identityPrincipalIdentities)) {
			directoryID, principalID, _ := strings.Cut(key, "/")
			target.IdentityReferenceScopes = append(target.IdentityReferenceScopes,
				IdentityReferenceScope{DirectoryID: directoryID, PrincipalID: principalID})
			for _, observed := range p.resourceCache.identityPrincipalIdentities[key] {
				identity := resources.IdentityPrincipalIdentityResource{
					Type:         observed.Type,
					AuthServerID: observed.AuthServerID, ClientID: observed.ClientID,
					ControlPlaneID: observed.ControlPlaneID, ConsumerID: observed.ConsumerID,
				}
				if !uses(identity) {
					continue
				}
				detached := false
				for _, change := range plan.Changes {
					if change.ResourceType != ResourceTypeIdentityPrincipalIdentity ||
						change.ResourceID != observed.ID ||
						change.Parent == nil ||
						change.Parent.ID != principalID ||
						change.References[FieldDirectoryID].ID != directoryID {
						continue
					}
					switch change.Action {
					case ActionDelete:
						detached = true
					case ActionUpdate:
						data, err := json.Marshal(change.Fields)
						if err != nil {
							return err
						}
						var next resources.IdentityPrincipalIdentityResource
						if err := json.Unmarshal(data, &next); err != nil {
							return err
						}
						next.Ref = change.ResourceRef
						if err := next.Validate(); err != nil {
							return err
						}
						detached = !uses(next)
					case ActionCreate, ActionExternalTool:
						// These operations cannot remove an existing association.
					}
					if detached {
						target.DependsOn = append(target.DependsOn, change.ID)
						break
					}
				}
				if !detached {
					return fmt.Errorf(
						"cannot delete %s %q: still referenced by principal identity %s",
						target.ResourceType,
						target.ResourceRef,
						observed.ID,
					)
				}
			}
		}
		for _, change := range plan.Changes {
			if change.ResourceType != ResourceTypeIdentityPrincipalIdentity || change.Action == ActionDelete {
				continue
			}
			data, err := json.Marshal(change.Fields)
			if err != nil {
				return err
			}
			var next resources.IdentityPrincipalIdentityResource
			if err := json.Unmarshal(data, &next); err != nil {
				return err
			}
			if uses(next) {
				return fmt.Errorf(
					"cannot write identity %q while deleting its associated %s",
					change.ResourceRef,
					target.ResourceType,
				)
			}
		}
	}
	return nil
}
