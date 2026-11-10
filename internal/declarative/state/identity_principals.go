package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

type IdentityPrincipal struct {
	helpers.IdentityPrincipal
	NormalizedLabels map[string]string
}

func (p IdentityPrincipal) GetID() string                          { return p.ID }
func (p IdentityPrincipal) GetName() string                        { return p.ManagedBy[resources.ManagedByRefKey] }
func (p IdentityPrincipal) GetLabels() map[string]string           { return p.Labels }
func (p IdentityPrincipal) GetNormalizedLabels() map[string]string { return p.NormalizedLabels }
func normalizeIdentityPrincipal(value helpers.IdentityPrincipal) IdentityPrincipal {
	var ownership map[string]string
	if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
		ownership = resources.NormalizedChildOwnership(value.ManagedBy)
	}
	return IdentityPrincipal{IdentityPrincipal: value, NormalizedLabels: ownership}
}

func (c *Client) ListIdentityPrincipals(ctx context.Context, directoryID string) ([]IdentityPrincipal, error) {
	if err := ValidateAPIClient(c.identityPrincipalsAPI, "Kong Identity principals API"); err != nil {
		return nil, err
	}
	values, err := helpers.ListIdentityPrincipals(ctx, c.identityPrincipalsAPI, directoryID, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityPrincipal, 0, len(values))
	for _, value := range values {
		result = append(result, normalizeIdentityPrincipal(value))
	}
	return result, nil
}

func (c *Client) GetIdentityPrincipal(ctx context.Context, directoryID, id string) (*IdentityPrincipal, error) {
	if err := ValidateAPIClient(c.identityPrincipalsAPI, "Kong Identity principals API"); err != nil {
		return nil, err
	}
	value, err := helpers.GetIdentityPrincipal(ctx, c.identityPrincipalsAPI, directoryID, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	result := normalizeIdentityPrincipal(*value)
	return &result, nil
}

func (c *Client) CreateIdentityPrincipal(ctx context.Context, directoryID string,
	request kkComps.CreatePrincipalBody,
) (*IdentityPrincipal, error) {
	if err := ValidateAPIClient(c.identityPrincipalsAPI, "Kong Identity principals API"); err != nil {
		return nil, err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityPrincipal, &request); err != nil {
		return nil, err
	}
	// managed_by is not a server-enforced unique key. Observe an uncertain
	// outcome before a new create instead of replaying it inside either client.
	response, err := c.identityPrincipalsAPI.CreatePrincipal(httpclient.WithoutRetries(ctx), directoryID, request,
		kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("failed to create principal: %w", err)
	}
	if response == nil || response.KongPrincipal == nil || response.KongPrincipal.ID == "" {
		return nil, fmt.Errorf("missing created principal response")
	}
	// Read back through the presence-preserving model and check persisted identity.
	created, err := c.GetIdentityPrincipal(ctx, directoryID, response.KongPrincipal.ID)
	if err != nil {
		return nil, err
	}
	if created == nil || !principalMetadataRetained(created.ManagedBy, request.ManagedBy) {
		return nil, fmt.Errorf(
			"principal %s was created but the API did not retain its ownership identity",
			response.KongPrincipal.ID,
		)
	}
	if request.Metadata != nil && !identityMetadataRetained(created.Metadata, request.Metadata) {
		return nil, fmt.Errorf("principal %s was created but the API did not retain requested metadata", created.ID)
	}
	if request.Active != nil && (created.Active == nil || *created.Active != *request.Active) {
		return nil, fmt.Errorf(
			"principal %s was created but the API did not retain observable requested active state (Q10)", created.ID,
		)
	}
	return created, nil
}

func (c *Client) ReplaceIdentityPrincipal(ctx context.Context, directoryID, id string,
	request kkComps.ReplacePrincipalBody,
) error {
	if err := ValidateAPIClient(c.identityPrincipalsAPI, "Kong Identity principals API"); err != nil {
		return err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityPrincipal, &request); err != nil {
		return err
	}
	response, err := c.identityPrincipalsAPI.ReplacePrincipal(ctx, kkOps.ReplacePrincipalRequest{
		DirectoryID: directoryID, PrincipalID: id, ReplacePrincipalBody: request,
	})
	if err != nil {
		return fmt.Errorf("failed to replace principal: %w", err)
	}
	if response == nil || response.KongPrincipal == nil ||
		!principalMetadataRetained(response.KongPrincipal.ManagedBy, request.ManagedBy) {
		return fmt.Errorf("principal API did not retain ownership metadata")
	}
	if request.Metadata != nil && !identityMetadataRetained(response.KongPrincipal.Metadata, request.Metadata) {
		return fmt.Errorf("principal API did not retain requested metadata")
	}
	if request.Active != nil {
		current, err := c.GetIdentityPrincipal(ctx, directoryID, id)
		if err != nil {
			return fmt.Errorf("principal %s was updated but active state could not be verified: %w", id, err)
		}
		if current == nil || current.Active == nil || *current.Active != *request.Active {
			return fmt.Errorf("principal %s was updated but the API did not retain observable requested active state (Q10)", id)
		}
	}
	return nil
}

func identityMetadataRetained(observed, requested map[string]kkComps.KongIdentityMetadata) bool {
	if len(observed) == 0 && len(requested) == 0 {
		return true
	}
	actual, err := json.Marshal(observed)
	if err != nil {
		return false
	}
	expected, err := json.Marshal(requested)
	return err == nil && bytes.Equal(actual, expected)
}

func principalMetadataRetained(observed, requested map[string]string) bool {
	if len(observed) != len(requested) {
		return false
	}
	for key, value := range requested {
		if observed[key] != value {
			return false
		}
	}
	return true
}

func (c *Client) DeleteIdentityPrincipal(ctx context.Context, directoryID, id string) error {
	if err := ValidateAPIClient(c.identityPrincipalsAPI, "Kong Identity principals API"); err != nil {
		return err
	}
	_, err := c.identityPrincipalsAPI.DeletePrincipal(ctx, directoryID, id)
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete principal: %w", err)
	}
	return nil
}

func (p IdentityPrincipal) GetManagedBy() map[string]string { return p.ManagedBy }
