package helpers

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type IdentityGroupsAPI interface {
	IdentityGroupsReader
	IdentityGroupMembershipsReader
	CreateGroup(context.Context, string, kkComps.CreateGroupBody, ...kkOps.Option) (*kkOps.CreateGroupResponse, error)
	ReplaceGroup(context.Context, kkOps.ReplaceGroupRequest, ...kkOps.Option) (*kkOps.ReplaceGroupResponse, error)
	DeleteGroup(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteGroupResponse, error)
	ListGroupKeys(context.Context, kkOps.ListGroupKeysRequest, ...kkOps.Option) (*kkOps.ListGroupKeysResponse, error)
}

func ValidateIdentityGroupManagement(value IdentityGroup) error {
	// KongGroupType explicitly defines omitted/null as the ordinary group type.
	if value.Type != nil && *value.Type != kkComps.KongGroupTypeGroup || value.ProvisionedBy == nil ||
		*value.ProvisionedBy != kkComps.ProvisionedByKonnectUser || value.TeamManager != nil {
		return fmt.Errorf(
			"management requires an ordinary group provisioned by konnect_user; team/importer writes are deferred (Q32)",
		)
	}
	return nil
}

// IdentityGroupReplacement preserves omitted public settings on full PUT.
func IdentityGroupReplacement(value IdentityGroup) (kkComps.ReplaceGroupBody, error) {
	if err := ValidateIdentityGroupManagement(value); err != nil {
		return kkComps.ReplaceGroupBody{}, err
	}
	return kkComps.ReplaceGroupBody{
		Name: value.Name, Description: &value.Description, Type: new(kkComps.ReplaceGroupBodyTypeGroup),
		Metadata: value.Metadata, Labels: value.Labels, ManagedBy: value.ManagedBy,
	}, nil
}

// VerifyIdentityGroupEmpty does not grant deletion authority over descendants.
func VerifyIdentityGroupEmpty(ctx context.Context, api IdentityGroupsAPI, directoryID, groupID string) error {
	entries, err := ListIdentityGroupMemberships(ctx, api, directoryID, groupID, 100)
	if err != nil {
		return fmt.Errorf("cannot verify group memberships: %w", err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("group still has memberships; remove them explicitly before deleting it")
	}
	// No paging/filter parameters: prove an empty, terminal first page. Any key
	// blocks deletion, so there is no need to traverse or expose credential data.
	response, err := api.ListGroupKeys(ctx, kkOps.ListGroupKeysRequest{DirectoryID: directoryID, GroupID: groupID})
	if err != nil {
		return identitySecretAPIError("verify group API keys before deletion", err)
	}
	if response == nil {
		return fmt.Errorf("missing group API-key collection response")
	}
	var page identityGroupPage[json.RawMessage]
	if err := decodeIdentityGroupResponse(response.RawResponse, &page); err != nil {
		return fmt.Errorf("cannot verify group API-key collection")
	}
	next, err := page.next()
	if err != nil {
		return fmt.Errorf("cannot verify group API-key pagination")
	}
	if len(page.Data) != 0 || next != nil && *next != "" {
		return fmt.Errorf(
			"group has API keys or an incomplete credential observation; remove keys explicitly before deleting it",
		)
	}
	return nil
}
