package executor

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

func executorOwnedModel(t *testing.T, variant string) kkComps.AIGatewayModel {
	t.Helper()
	capability := "generate"
	if variant == "api" {
		capability = "files"
	}
	payload := map[string]any{
		"id": "model-id", "name": "owned", "display_name": "Owned", "type": variant,
		"config": map[string]any{"route": map[string]any{}}, "formats": []any{map[string]any{"type": "openai"}},
		"targets": []any{
			map[string]any{"name": "gpt-4o", "provider": "openai", "config": map[string]any{"type": "openai"}},
		},
		"capabilities": []string{
			capability,
		}, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
		"managed_by": map[string]string{
			resources.ModelNamespaceKey: "team-a",
			resources.ModelProtectedKey: "false",
			"other-tool":                "retained",
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var model kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal(data, &model))
	return model
}

func TestSavedModelPlansRevalidateOwnershipBeforeMutation(t *testing.T) {
	for _, variant := range []string{"model", "api"} {
		for _, action := range []planner.ActionType{planner.ActionUpdate, planner.ActionDelete} {
			for _, owner := range []string{"", "team-b"} {
				t.Run(variant+"/"+string(action)+"/"+owner, func(t *testing.T) {
					model := executorOwnedModel(t, variant)
					fields, err := resources.AIGatewayModelMutablePayloadMap(model)
					require.NoError(t, err)
					change := planner.PlannedChange{
						ResourceType: planner.ResourceTypeAIGatewayModel,
						ResourceID:   "model-id", Namespace: "team-a", Action: action, Fields: fields,
						Parent: &planner.ParentInfo{Ref: "shared", ID: "gateway-id"},
					}
					encoded, err := json.Marshal(change)
					require.NoError(t, err)
					var saved planner.PlannedChange
					require.NoError(t, json.Unmarshal(encoded, &saved))
					resources.AIGatewayModelManagedBy(model)[resources.ModelNamespaceKey] = owner
					api := &modelOwnershipExecutionAPI{model: model}
					client := state.NewClient(state.ClientConfig{AIGatewayModelAPI: api})
					adapter := NewAIGatewayModelAdapter(client)
					base := NewBaseExecutor(adapter, client, false)
					ctx := context.WithValue(t.Context(), log.LoggerKey, slog.Default())
					if action == planner.ActionUpdate {
						_, err = base.Update(ctx, saved)
					} else {
						err = base.Delete(ctx, saved)
					}
					require.Error(t, err)
					require.Zero(t, api.updates)
					require.Zero(t, api.deletes)
				})
			}
		}
	}
}

func TestModelExecutionPreservesFreshMetadataAndProtection(t *testing.T) {
	for _, variant := range []string{"model", "api"} {
		t.Run(variant, func(t *testing.T) {
			model := executorOwnedModel(t, variant)
			delete(resources.AIGatewayModelManagedBy(model), "other-tool")
			payload, err := resources.AIGatewayModelMutablePayloadMap(model)
			require.NoError(t, err)
			change := planner.PlannedChange{
				ResourceType: planner.ResourceTypeAIGatewayModel, Namespace: "team-a",
				Action: planner.ActionUpdate, ResourceID: "model-id", Fields: payload,
				Parent: &planner.ParentInfo{ID: "gateway-id"},
			}
			api := &modelOwnershipExecutionAPI{model: model}
			adapter := NewAIGatewayModelAdapter(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}))
			execCtx := NewExecutionContext(&change)
			var req kkComps.UpdateAIGatewayModelRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, payload, &req, nil))
			resources.AIGatewayModelManagedBy(api.model)["new-tool"] = "added-after-planning"
			_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
			require.NoError(t, err)
			require.Equal(t, "added-after-planning", modelUpdateMetadata(api.lastUpdate)["new-tool"])
			resources.AIGatewayModelManagedBy(api.model)[resources.ModelProtectedKey] = "true"
			_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
			require.ErrorContains(t, err, "protected")
			err = adapter.Delete(t.Context(), "model-id", execCtx)
			require.ErrorContains(t, err, "protected")
			require.Zero(t, api.deletes)
		})
	}
}

func TestModelCreateRejectsConcurrentNameCollision(t *testing.T) {
	model := executorOwnedModel(t, "model")
	payload, err := resources.AIGatewayModelMutablePayloadMap(model)
	require.NoError(t, err)
	change := planner.PlannedChange{Namespace: "team-a", Fields: payload, Parent: &planner.ParentInfo{ID: "gateway-id"}}
	api := &modelOwnershipExecutionAPI{model: model}
	adapter := NewAIGatewayModelAdapter(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}))
	var req kkComps.CreateAIGatewayModelRequest
	execCtx := NewExecutionContext(&change)
	require.NoError(t, adapter.MapCreateFields(t.Context(), execCtx, payload, &req))
	_, err = adapter.Create(t.Context(), req, "team-a", execCtx)
	require.ErrorContains(t, err, "already exists")
}

func TestProtectedModelsOnlyAllowPayloadPreservingUnprotection(t *testing.T) {
	for _, variant := range []string{"model", "api"} {
		t.Run(variant, func(t *testing.T) {
			model := executorOwnedModel(t, variant)
			metadata := resources.AIGatewayModelManagedBy(model)
			metadata[resources.ModelProtectedKey] = "true"
			fields, err := resources.AIGatewayModelMutablePayloadMap(model)
			require.NoError(t, err)
			fields[planner.FieldManagedBy] = resources.BuildModelManagedBy(metadata, nil, "team-a", false, "dev")
			change := planner.PlannedChange{
				Namespace: "team-a", Fields: fields,
				Parent:     &planner.ParentInfo{ID: "gateway-id"},
				Protection: planner.ProtectionChange{Old: true, New: false},
				ChangedFields: map[string]planner.FieldChange{
					planner.FieldManagedBy: {Old: metadata, New: fields[planner.FieldManagedBy]},
				},
			}
			api := &modelOwnershipExecutionAPI{model: model}
			adapter := NewAIGatewayModelAdapter(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}))
			execCtx := NewExecutionContext(&change)
			var req kkComps.UpdateAIGatewayModelRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, fields, &req, nil))
			_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
			require.NoError(t, err)
			require.Equal(t, 1, api.updates)
			change.SecretWrites = []planner.SecretWriteIntent{{Field: "/config/secret"}}
			_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
			require.ErrorContains(t, err, "protected")
			require.Equal(t, 1, api.updates)
			change.SecretWrites = nil
			if req.AIGatewayModelAPI != nil {
				req.AIGatewayModelAPI.DisplayName = "tampered"
			} else {
				req.AIGatewayModelModel.DisplayName = "tampered"
			}
			_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
			require.ErrorContains(t, err, "protected")
			require.Equal(t, 1, api.updates)
		})
	}
}

type modelOwnershipExecutionAPI struct {
	helpers.AIGatewayModelAPI
	model            kkComps.AIGatewayModel
	updates, deletes int
	lastUpdate       kkComps.UpdateAIGatewayModelRequest
}

func (a *modelOwnershipExecutionAPI) GetAiGatewayModel(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayModelResponse, error) {
	return &kkOps.GetAiGatewayModelResponse{AIGatewayModel: &a.model}, nil
}

func (a *modelOwnershipExecutionAPI) ListAiGatewayModels(
	context.Context,
	kkOps.ListAiGatewayModelsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	return &kkOps.ListAiGatewayModelsResponse{
		ListAIGatewayModelsResponse: &kkComps.ListAIGatewayModelsResponse{Data: []kkComps.AIGatewayModel{a.model}},
	}, nil
}

func (a *modelOwnershipExecutionAPI) UpdateAiGatewayModel(
	_ context.Context,
	req kkOps.UpdateAiGatewayModelRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayModelResponse, error) {
	a.updates++
	a.lastUpdate = req.UpdateAIGatewayModelRequest
	return &kkOps.UpdateAiGatewayModelResponse{AIGatewayModel: &a.model}, nil
}

func (a *modelOwnershipExecutionAPI) DeleteAiGatewayModel(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayModelResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayModelResponse{}, nil
}

func TestSavedModelUnprotectionRejectsConcurrentProtectionChange(t *testing.T) {
	model := executorOwnedModel(t, "model")
	metadata := resources.AIGatewayModelManagedBy(model)
	metadata[resources.ModelProtectedKey] = "true"
	fields, err := resources.AIGatewayModelMutablePayloadMap(model)
	require.NoError(t, err)
	fields[planner.FieldManagedBy] = resources.BuildModelManagedBy(metadata, nil, "team-a", false, "dev")
	change := planner.PlannedChange{
		Namespace: "team-a",
		Fields:    fields,
		Parent:    &planner.ParentInfo{ID: "gateway-id"},
		Protection: planner.ProtectionChange{
			Old: true,
			New: false,
		},
		ChangedFields: map[string]planner.FieldChange{planner.FieldManagedBy: {
			Old: metadata,
			New: fields[planner.FieldManagedBy],
		}},
	}
	data, err := json.Marshal(change)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(data, &saved))
	api := &modelOwnershipExecutionAPI{model: model}
	adapter := NewAIGatewayModelAdapter(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}))
	execCtx := NewExecutionContext(&saved)
	var req kkComps.UpdateAIGatewayModelRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, saved.Fields, &req, nil))
	metadata[resources.ModelProtectedKey] = "false"
	_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
	require.ErrorContains(t, err, "protection changed after planning")
	require.Zero(t, api.updates)
}

func TestModelExecutionRejectsFreshMetadataCapacityOverflow(t *testing.T) {
	model := executorOwnedModel(t, "model")
	fields, err := resources.AIGatewayModelMutablePayloadMap(model)
	require.NoError(t, err)
	change := planner.PlannedChange{Namespace: "team-a", Fields: fields, Parent: &planner.ParentInfo{ID: "gateway-id"}}
	api := &modelOwnershipExecutionAPI{model: model}
	adapter := NewAIGatewayModelAdapter(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}))
	execCtx := NewExecutionContext(&change)
	var req kkComps.UpdateAIGatewayModelRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, fields, &req, nil))
	metadata := resources.AIGatewayModelManagedBy(api.model)
	for _, key := range []string{"new-one", "new-two", "new-three", "new-four"} {
		metadata[key] = "other-tool"
	}
	_, err = adapter.Update(t.Context(), "model-id", req, "team-a", execCtx)
	require.ErrorContains(t, err, "5-entry")
	require.Zero(t, api.updates)
}
