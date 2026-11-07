package state

import (
	"context"
	"fmt"
	"maps"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

type IdentityGroup struct {
	helpers.IdentityGroup
	NormalizedLabels map[string]string
}

func (g IdentityGroup) GetID() string                          { return g.ID }
func (g IdentityGroup) GetName() string                        { return g.Name }
func (g IdentityGroup) GetLabels() map[string]string           { return g.Labels }
func (g IdentityGroup) GetManagedBy() map[string]string        { return g.ManagedBy }
func (g IdentityGroup) GetNormalizedLabels() map[string]string { return g.NormalizedLabels }

func normalizeIdentityGroup(value helpers.IdentityGroup) IdentityGroup {
	var ownership map[string]string
	if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
		ownership = resources.NormalizedChildOwnership(value.ManagedBy)
	}
	return IdentityGroup{IdentityGroup: value, NormalizedLabels: ownership}
}

func (c *Client) ListIdentityGroups(ctx context.Context, directoryID string) ([]IdentityGroup, error) {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return nil, err
	}
	values, err := helpers.ListIdentityGroups(ctx, c.identityGroupsAPI, directoryID, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityGroup, 0, len(values))
	for _, value := range values {
		result = append(result, normalizeIdentityGroup(value))
	}
	return result, nil
}

func (c *Client) GetIdentityGroup(ctx context.Context, directoryID, id string) (*IdentityGroup, error) {
	values, err := c.ListIdentityGroups(ctx, directoryID)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			directory, directoryErr := c.GetIdentityDirectoryByID(ctx, directoryID)
			if directoryErr != nil {
				return nil, directoryErr
			}
			if directory == nil {
				return nil, nil
			}
		}
		return nil, err
	}
	for _, value := range values {
		if value.ID != id {
			continue
		}
		count := 0
		for _, other := range values {
			if other.Name == value.Name {
				count++
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("multiple groups match name %q in the selected directory", value.Name)
		}
		fresh, err := helpers.ResolveIdentityGroup(ctx, c.identityGroupsAPI, directoryID, id, 100)
		if err != nil {
			if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
				return nil, nil
			}
			return nil, err
		}
		if fresh.Name != value.Name {
			return nil, fmt.Errorf("group name changed during lookup")
		}
		result := normalizeIdentityGroup(*fresh)
		return &result, nil
	}
	return nil, nil
}

func (c *Client) CreateIdentityGroup(ctx context.Context, directoryID string,
	request kkComps.CreateGroupBody,
) (*IdentityGroup, error) {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return nil, err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityGroup, &request); err != nil {
		return nil, err
	}
	response, err := c.identityGroupsAPI.CreateGroup(httpclient.WithoutRetries(ctx), directoryID, request,
		kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("failed to create identity group: %w", err)
	}
	if response == nil || response.KongGroup == nil || response.KongGroup.ID == "" {
		return nil, fmt.Errorf("missing created identity group response")
	}
	created, err := c.GetIdentityGroup(ctx, directoryID, response.KongGroup.ID)
	if err != nil {
		return nil, err
	}
	if created == nil || created.Name != request.Name || !maps.Equal(created.ManagedBy, request.ManagedBy) {
		return nil, fmt.Errorf(
			"group %s was created but ownership identity could not be verified",
			response.KongGroup.ID,
		)
	}
	if err := helpers.ValidateIdentityGroupManagement(created.IdentityGroup); err != nil {
		return nil, err
	}
	if request.Metadata != nil && !identityMetadataRetained(created.Metadata, request.Metadata) {
		return nil, fmt.Errorf("group %s was created but the API did not retain requested metadata", created.ID)
	}
	if request.Description != nil && created.Description != *request.Description ||
		request.Labels != nil && !maps.Equal(created.Labels, request.Labels) {
		return nil, fmt.Errorf("group %s was created but the API did not retain requested public settings", created.ID)
	}
	return created, nil
}

func (c *Client) ReplaceIdentityGroup(ctx context.Context, directoryID, id string,
	request kkComps.ReplaceGroupBody,
) error {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityGroup, &request); err != nil {
		return err
	}
	response, err := c.identityGroupsAPI.ReplaceGroup(ctx, kkOps.ReplaceGroupRequest{
		DirectoryID: directoryID, GroupID: id, ReplaceGroupBody: request,
	})
	if err != nil {
		return fmt.Errorf("failed to replace identity group: %w", err)
	}
	if response == nil || response.KongGroup == nil || response.KongGroup.ID != id ||
		response.KongGroup.Name != request.Name ||
		!maps.Equal(response.KongGroup.ManagedBy, request.ManagedBy) {
		return fmt.Errorf("group API did not retain requested ownership identity")
	}
	if request.Metadata != nil && !identityMetadataRetained(response.KongGroup.Metadata, request.Metadata) {
		return fmt.Errorf("group API did not retain requested metadata")
	}
	if request.Description != nil && response.KongGroup.Description != *request.Description ||
		request.Labels != nil && !maps.Equal(response.KongGroup.Labels, request.Labels) ||
		request.Type != nil && response.KongGroup.Type != nil &&
			string(*response.KongGroup.Type) != string(*request.Type) {
		return fmt.Errorf("group API did not retain requested public settings")
	}
	return nil
}

// VerifyPrincipalGroupMembershipsEmpty observes direct relationships rather than
// interpreting the principal's computed groups as independently writable links.
func (c *Client) VerifyPrincipalGroupMembershipsEmpty(ctx context.Context, directoryID, principalID string) error {
	groups, err := c.ListIdentityGroups(ctx, directoryID)
	if err != nil {
		return fmt.Errorf("cannot verify principal group memberships: %w", err)
	}
	for _, group := range groups {
		entries, err := helpers.ListIdentityGroupMemberships(ctx, c.identityGroupsAPI, directoryID, group.ID, 100)
		if err != nil {
			if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
				// An independently deleted group cannot retain this direct link.
				current, lookupErr := c.GetIdentityGroup(ctx, directoryID, group.ID)
				if lookupErr != nil {
					return fmt.Errorf("cannot verify principal group memberships: %w", lookupErr)
				}
				if current == nil {
					continue
				}
			}
			return fmt.Errorf("cannot verify principal group memberships: %w", err)
		}
		for _, entry := range entries {
			if entry.Member.Type == kkComps.KongGroupMembershipTypePrincipal && entry.Member.ID == principalID {
				return fmt.Errorf(
					"principal still has a direct membership in group %q; remove it explicitly before deleting the principal",
					group.Name,
				)
			}
		}
	}
	return nil
}

func (c *Client) VerifyIdentityGroupEmpty(ctx context.Context, directoryID, id string) error {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return err
	}
	return helpers.VerifyIdentityGroupEmpty(ctx, c.identityGroupsAPI, directoryID, id)
}

func (c *Client) DeleteIdentityGroup(ctx context.Context, directoryID, id string) error {
	if err := ValidateAPIClient(c.identityGroupsAPI, "Kong Identity groups API"); err != nil {
		return err
	}
	_, err := c.identityGroupsAPI.DeleteGroup(ctx, directoryID, id)
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete identity group: %w", err)
	}
	return nil
}
