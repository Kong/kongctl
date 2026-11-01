package helpers

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityPrincipalGroupsReader interface {
	ListPrincipalGroups(context.Context, kkOps.ListPrincipalGroupsRequest, ...kkOps.Option) (
		*kkOps.ListPrincipalGroupsResponse, error)
}

// IdentityPrincipalGroup is the principal-scoped projection, not the group
// resource or an authorization to manage its direct or inherited membership.
type IdentityPrincipalGroup struct {
	ID            string                   `json:"id"                       yaml:"id"`
	Name          string                   `json:"name"                     yaml:"name"`
	Type          *kkComps.KongGroupType   `json:"type,omitempty"           yaml:"type,omitempty"`
	Direct        *bool                    `json:"direct"                   yaml:"direct"`
	ProvisionedBy *kkComps.ProvisionedBy   `json:"provisioned_by,omitempty" yaml:"provisioned_by,omitempty"`
	TeamManager   *kkComps.KongTeamManager `json:"team_manager,omitempty"   yaml:"team_manager,omitempty"`
	MembershipID  *string                  `json:"membership_id"            yaml:"membership_id"`
}

func ListIdentityPrincipalGroups(ctx context.Context, api IdentityPrincipalGroupsReader,
	directoryID, principalID string, pageSize int64,
) ([]IdentityPrincipalGroup, error) {
	if api == nil {
		return nil, fmt.Errorf("principal group reader is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return nil, fmt.Errorf("directory and principal IDs must be UUIDs")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListPrincipalGroupsRequest{
		DirectoryID: directoryID, PrincipalID: principalID, Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]IdentityPrincipalGroup, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListPrincipalGroups(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list principal groups: %w", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing principal group collection response")
		}
		var page identityGroupPage[IdentityPrincipalGroup]
		if err := decodeIdentityGroupResponse(response.RawResponse, &page); err != nil {
			return nil, err
		}
		next, err := page.next()
		if err != nil {
			return nil, err
		}
		for _, value := range page.Data {
			if !util.IsValidUUID(value.ID) || value.Name == "" || value.Direct == nil || seen[value.ID] {
				return nil, fmt.Errorf("principal group collection contains an invalid or repeated resource")
			}
			if *value.Direct != (value.MembershipID != nil) ||
				value.MembershipID != nil && !util.IsValidUUID(*value.MembershipID) {
				return nil, fmt.Errorf("principal group membership ID contradicts its direct/inherited status")
			}
			seen[value.ID] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate principal groups: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

func ResolveIdentityPrincipalGroup(ctx context.Context, api IdentityPrincipalGroupsReader,
	directoryID, principalID, identifier string, pageSize int64,
) (*IdentityPrincipalGroup, error) {
	values, err := ListIdentityPrincipalGroups(ctx, api, directoryID, principalID, pageSize)
	if err != nil {
		return nil, err
	}
	var match *IdentityPrincipalGroup
	byID := util.IsValidUUID(identifier)
	for _, value := range values {
		if byID && value.ID == identifier || !byID && value.Name == identifier {
			if match != nil {
				return nil, fmt.Errorf("multiple groups match the selector under the selected principal")
			}
			match = &value
		}
	}
	if match == nil {
		return nil, fmt.Errorf("group not found under the selected principal")
	}
	return match, nil
}
