package state

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/util"
)

type IdentityGroupMembership struct {
	kkComps.KongGroupMembershipEntry
}

func (m IdentityGroupMembership) GetID() string { return m.ID }

func (m IdentityGroupMembership) GetName() string {
	return string(m.Member.Type) + "/" + m.Member.ID
}
func (m IdentityGroupMembership) GetLabels() map[string]string           { return nil }
func (m IdentityGroupMembership) GetNormalizedLabels() map[string]string { return nil }

func (c *Client) ListIdentityGroupMemberships(
	ctx context.Context,
	directoryID, groupID string,
) ([]IdentityGroupMembership, error) {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return nil, err
	}
	values, err := helpers.ListIdentityGroupMemberships(ctx, c.identityGroupsAPI, directoryID, groupID, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityGroupMembership, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityGroupMembership{value})
	}
	return result, nil
}

func (c *Client) GetIdentityGroupMembership(
	ctx context.Context,
	directoryID, groupID, id string,
) (*IdentityGroupMembership, error) {
	values, err := c.ListIdentityGroupMemberships(ctx, directoryID, groupID)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			group, groupErr := c.GetIdentityGroup(ctx, directoryID, groupID)
			if groupErr != nil {
				return nil, groupErr
			}
			if group == nil {
				return nil, nil
			}
		}
		return nil, err
	}
	for _, value := range values {
		if value.ID != id {
			continue
		}
		fresh, err := helpers.GetIdentityGroupMembership(ctx, c.identityGroupsAPI, directoryID, groupID, id, 100)
		if err != nil {
			if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
				return nil, nil
			}
			return nil, err
		}
		return &IdentityGroupMembership{*fresh}, nil
	}
	return nil, nil
}

func (c *Client) VerifyIdentityMembershipPrincipal(ctx context.Context, directoryID, id string) error {
	if !util.IsValidUUID(id) {
		return fmt.Errorf("membership principal ID must be a resolved UUID")
	}
	values, err := c.ListIdentityPrincipals(ctx, directoryID)
	if err != nil {
		return err
	}
	count := 0
	for _, value := range values {
		if value.ID == id {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("membership principal does not uniquely belong to the selected directory")
	}
	return nil
}

// Caller verifies fresh group ownership/protection. This method never creates
// a principal or replays an uncertain relationship creation.
func (c *Client) CreateIdentityGroupMembership(ctx context.Context, directoryID, groupID string,
	request kkComps.CreateGroupMembershipEntryBody,
) (*IdentityGroupMembership, error) {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return nil, err
	}
	if request.Member.Type != kkComps.KongGroupMembershipTypePrincipal {
		return nil, fmt.Errorf("nested group memberships are deferred (Q32)")
	}
	if err := c.VerifyIdentityMembershipPrincipal(ctx, directoryID, request.Member.ID); err != nil {
		return nil, err
	}
	all, err := c.ListIdentityGroupMemberships(ctx, directoryID, groupID)
	if err != nil {
		return nil, err
	}
	for _, entry := range all {
		if entry.Member.Type == request.Member.Type && entry.Member.ID == request.Member.ID {
			return nil, fmt.Errorf("group already has the requested member; regenerate the plan")
		}
	}
	response, err := c.identityGroupsAPI.CreateGroupMembershipEntry(
		httpclient.WithoutRetries(ctx),
		kkOps.CreateGroupMembershipEntryRequest{
			DirectoryID: directoryID, GroupID: groupID, CreateGroupMembershipEntryBody: request,
		},
		kkOps.WithRetries(retry.Config{Strategy: "none"}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create membership; inspect the collection before retrying: %w", err)
	}
	if response == nil || response.KongGroupMembershipEntry == nil ||
		!util.IsValidUUID(response.KongGroupMembershipEntry.ID) {
		return nil, fmt.Errorf("missing membership create response; inspect the collection before retrying")
	}
	created := response.KongGroupMembershipEntry
	if created.Member.Type != request.Member.Type || created.Member.ID != request.Member.ID {
		return nil, fmt.Errorf("created membership does not match the requested principal; inspect before retrying")
	}
	fresh, err := c.GetIdentityGroupMembership(ctx, directoryID, groupID, created.ID)
	if err != nil {
		return nil, err
	}
	if fresh == nil || fresh.Member.Type != request.Member.Type || fresh.Member.ID != request.Member.ID {
		return nil, fmt.Errorf("created membership was not retained under the selected group; inspect before retrying")
	}
	return fresh, nil
}

func (c *Client) DeleteIdentityGroupMembership(ctx context.Context, directoryID, groupID, id string) error {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return err
	}
	_, err := c.identityGroupsAPI.DeleteGroupMembershipEntry(ctx, kkOps.DeleteGroupMembershipEntryRequest{
		DirectoryID: directoryID, GroupID: groupID, GroupMembershipID: id,
	})
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete group membership: %w", err)
	}
	return nil
}

func (c *Client) VerifyIdentityGroupKeysEmpty(ctx context.Context, directoryID, groupID string) error {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return err
	}
	return helpers.VerifyIdentityGroupKeysEmpty(ctx, c.identityGroupsAPI, directoryID, groupID)
}
