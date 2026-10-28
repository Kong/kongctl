package executor

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayVaultAdapter implements ResourceOperations for AI Gateway Vaults.
type AIGatewayVaultAdapter struct {
	client *state.Client
}

// NewAIGatewayVaultAdapter creates a new AI Gateway Vault adapter.
func NewAIGatewayVaultAdapter(client *state.Client) *AIGatewayVaultAdapter {
	return &AIGatewayVaultAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayVaultRequest.
func (a *AIGatewayVaultAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayVaultRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Vault create", fields, create); err != nil {
		return err
	}
	if create.KonnectConfigStoreVault == nil &&
		create.EnvironmentVariableVault == nil &&
		create.AwsSecretsManagerVault == nil &&
		create.GoogleSecretManagerVault == nil &&
		create.AzureKeyVault == nil &&
		create.ConjurVault == nil &&
		create.HashiCorpVault == nil {
		return fmt.Errorf("type must be a supported AI Gateway Vault type")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayVaultRequest.
func (a *AIGatewayVaultAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayVaultRequest,
	_ map[string]string,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Vault update", fields, update); err != nil {
		return err
	}
	if update.KonnectConfigStoreVault == nil &&
		update.EnvironmentVariableVault == nil &&
		update.AwsSecretsManagerVault == nil &&
		update.GoogleSecretManagerVault == nil &&
		update.AzureKeyVault == nil &&
		update.ConjurVault == nil &&
		update.HashiCorpVault == nil {
		return fmt.Errorf("type must be a supported AI Gateway Vault type")
	}
	return nil
}

// Create creates an AI Gateway Vault.
func (a *AIGatewayVaultAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayVaultRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	payload, err := ownedChildPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateChildWriteContext("AI Gateway vault", execCtx, payload, namespace); err != nil {
		return "", err
	}
	vaults, err := a.client.ListAIGatewayVaults(ctx, gatewayID)
	if err != nil {
		return "", err
	}
	for _, vault := range vaults {
		if resources.AIGatewayVaultName(vault.AIGatewayVault) == payload[planner.FieldName] {
			return "", childCreationCollision(
				"AI Gateway vault",
				resources.AIGatewayVaultName(vault.AIGatewayVault),
			)
		}
	}
	return a.client.CreateAIGatewayVault(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Vault.
func (a *AIGatewayVaultAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayVaultRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	payload, err := ownedChildPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateChildWriteContext("AI Gateway vault", execCtx, payload, namespace); err != nil {
		return "", err
	}
	current, err := a.client.GetAIGatewayVault(ctx, gatewayID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("AI Gateway vault no longer exists")
	}
	currentPayload, err := resources.AIGatewayVaultMutablePayloadMap(current.AIGatewayVault)
	if err != nil {
		return "", err
	}
	payload, err = preserveOwnedMappedChildMetadataUpdate[kkComps.UpdateAIGatewayVaultRequest](
		"AI Gateway vault",
		currentPayload,
		payload,
		execCtx,
	)
	if err != nil {
		return "", err
	}
	payload, err = prepareOwnedMappedChildUpdate(
		"AI Gateway vault",
		currentPayload,
		payload,
		execCtx,
	)
	if err != nil {
		return "", err
	}
	// AppRole PUT resets an omitted secret_id instead of preserving it. GET
	// redacts that value, so a metadata-only write cannot safely reconstruct it.
	config, _ := payload[planner.FieldConfig].(map[string]any)
	if payload[planner.FieldType] == "hcv" && config["auth_method"] == "approle" {
		secretID, _ := config["secret_id"].(string)
		secretFile, _ := config["secret_id_file"].(string)
		if strings.TrimSpace(secretID) == "" && strings.TrimSpace(secretFile) == "" {
			return "", fmt.Errorf("AI Gateway AppRole vault PUT cannot preserve redacted secret_id; " +
				"metadata-only updates require backend PATCH support; " +
				"an unprotected credential update must explicitly select /config/secret_id")
		}
	}
	if err := mapAIGatewaySDKRequest("AI Gateway vault update", payload, &req); err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayVault(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Vault.
func (a *AIGatewayVaultAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetAIGatewayVault(ctx, gatewayID, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if err := resources.ValidateChildOwnership(
		"AI Gateway vault", resources.AIGatewayVaultManagedBy(current.AIGatewayVault), execCtx.Namespace,
	); err != nil {
		return err
	}
	if resources.IsChildProtected(resources.AIGatewayVaultManagedBy(current.AIGatewayVault)) {
		return fmt.Errorf("AI Gateway vault is protected and cannot be deleted")
	}
	if err := validateCurrentAIGatewayChildDeletion(
		ctx, a.client, execCtx, gatewayID, id, resources.AIGatewayVaultName(current.AIGatewayVault),
	); err != nil {
		return err
	}
	return a.client.DeleteAIGatewayVault(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Vault by ID.
func (a *AIGatewayVaultAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	vault, err := a.client.GetAIGatewayVault(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if vault == nil {
		return nil, nil
	}
	return &aiGatewayVaultResourceInfo{vault: vault}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayVaultAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Vaults")
}

// ResourceType returns the resource type.
func (a *AIGatewayVaultAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayVault
}

// RequiredFields returns required fields for create.
func (a *AIGatewayVaultAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldName, planner.FieldConfig}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayVaultAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayVaultAdapter) getAIGatewayIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange
	if gatewayRef, ok := change.References[planner.FieldAIGatewayID]; ok && !unresolvedReferenceID(gatewayRef.ID) {
		return gatewayRef.ID, nil
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("AI Gateway ID required for Vault operations")
}

type aiGatewayVaultResourceInfo struct {
	vault *state.AIGatewayVault
}

func (a *aiGatewayVaultResourceInfo) GetID() string {
	return resources.AIGatewayVaultID(a.vault.AIGatewayVault)
}

func (a *aiGatewayVaultResourceInfo) GetName() string {
	return resources.AIGatewayVaultName(a.vault.AIGatewayVault)
}

func (a *aiGatewayVaultResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayVaultLabels(a.vault.AIGatewayVault)
}

func (a *aiGatewayVaultResourceInfo) GetNormalizedLabels() map[string]string {
	return a.vault.NormalizedLabels
}
