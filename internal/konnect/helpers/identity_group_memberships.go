package helpers

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityGroupMembershipsReader interface {
	ListGroupMembership(context.Context, kkOps.ListGroupMembershipRequest, ...kkOps.Option) (
		*kkOps.ListGroupMembershipResponse, error,
	)
	GetGroupMembershipEntry(context.Context, kkOps.GetGroupMembershipEntryRequest, ...kkOps.Option) (
		*kkOps.GetGroupMembershipEntryResponse, error,
	)
}

func validIdentityGroupMembership(value kkComps.KongGroupMembershipEntry) bool {
	return util.IsValidUUID(value.ID) && util.IsValidUUID(value.Member.ID) && value.Member.Type.IsExact()
}

func ListIdentityGroupMemberships(ctx context.Context, api IdentityGroupMembershipsReader,
	directoryID, groupID string, pageSize int64,
) ([]kkComps.KongGroupMembershipEntry, error) {
	if api == nil {
		return nil, fmt.Errorf("identity group membership reader is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(groupID) {
		return nil, fmt.Errorf("directory and group IDs must be UUIDs")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListGroupMembershipRequest{
		DirectoryID: directoryID, GroupID: groupID, Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]kkComps.KongGroupMembershipEntry, 0)
	seen, members := make(map[string]bool), make(map[kkComps.KongGroupMembershipType]map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListGroupMembership(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list identity group memberships: %w", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing identity group membership collection response")
		}
		var page identityGroupPage[kkComps.KongGroupMembershipEntry]
		if err := decodeIdentityGroupResponse(response.RawResponse, &page); err != nil {
			return nil, err
		}
		next, err := page.next()
		if err != nil {
			return nil, err
		}
		for _, value := range page.Data {
			if !validIdentityGroupMembership(value) || seen[value.ID] {
				return nil, fmt.Errorf("identity group membership collection contains an invalid or repeated entry")
			}
			if members[value.Member.Type] == nil {
				members[value.Member.Type] = make(map[string]bool)
			}
			if members[value.Member.Type][value.Member.ID] {
				return nil, fmt.Errorf("identity group membership collection repeats a member")
			}
			seen[value.ID], members[value.Member.Type][value.Member.ID] = true, true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate identity group memberships: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

func GetIdentityGroupMembership(ctx context.Context, api IdentityGroupMembershipsReader,
	directoryID, groupID, entryID string, pageSize int64,
) (*kkComps.KongGroupMembershipEntry, error) {
	if !util.IsValidUUID(entryID) {
		return nil, fmt.Errorf("group membership entry ID must be a UUID")
	}
	values, err := ListIdentityGroupMemberships(ctx, api, directoryID, groupID, pageSize)
	if err != nil {
		return nil, err
	}
	var match *kkComps.KongGroupMembershipEntry
	for _, value := range values {
		if value.ID == entryID {
			match = &value
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("membership entry %s not found in the selected group", entryID)
	}
	response, err := api.GetGroupMembershipEntry(ctx, kkOps.GetGroupMembershipEntryRequest{
		DirectoryID: directoryID, GroupID: groupID, GroupMembershipID: entryID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get identity group membership: %w", err)
	}
	if response == nil || response.KongGroupMembershipEntry == nil {
		return nil, fmt.Errorf("missing identity group membership response")
	}
	value := response.KongGroupMembershipEntry
	if !validIdentityGroupMembership(*value) || value.ID != entryID ||
		value.Member.ID != match.Member.ID || value.Member.Type != match.Member.Type {
		return nil, fmt.Errorf("identity group membership changed during lookup")
	}
	return value, nil
}
