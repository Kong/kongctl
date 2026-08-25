package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayConfigStoreSecretChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayID string,
	storeRef string,
	storeID string,
	storeChangeID string,
	desired []resources.AIGatewayConfigStoreSecretResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Config Store secret changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("config_store_ref", storeRef),
		slog.String("config_store_id", storeID),
		slog.Int("desired_count", len(desired)),
	)
	if storeID == "" {
		for _, secret := range desired {
			if !p.aiGatewayConfigStoreSecretHasValueSource(secret.Ref) {
				return missingAIGatewayConfigStoreSecretValueError(secret.Ref)
			}
			p.planAIGatewayConfigStoreSecretCreate(
				namespace,
				gatewayRef,
				gatewayID,
				storeRef,
				"",
				secret,
				[]string{storeChangeID},
				plan,
			)
		}
		return nil
	}

	currentSecrets, err := p.client.ListAIGatewayConfigStoreSecrets(ctx, gatewayID, storeID)
	if err != nil {
		return fmt.Errorf("failed to list secrets for AI Gateway Config Store %s: %w", storeRef, err)
	}
	currentByKey := make(map[string]state.AIGatewayConfigStoreSecret, len(currentSecrets))
	for _, secret := range currentSecrets {
		currentByKey[secret.Key] = secret
	}
	desiredKeys := make(map[string]bool, len(desired))
	for _, desiredSecret := range desired {
		desiredKeys[desiredSecret.Key] = true
		if _, exists := currentByKey[desiredSecret.Key]; !exists {
			if !p.aiGatewayConfigStoreSecretHasValueSource(desiredSecret.Ref) {
				return missingAIGatewayConfigStoreSecretValueError(desiredSecret.Ref)
			}
			p.planAIGatewayConfigStoreSecretCreate(
				namespace,
				gatewayRef,
				gatewayID,
				storeRef,
				storeID,
				desiredSecret,
				nil,
				plan,
			)
			continue
		}
		if resource := p.resources.GetAIGatewayConfigStoreSecretByRef(desiredSecret.Ref); resource != nil {
			resource.SetKonnectID(desiredSecret.Key)
		}
	}

	if plan.Metadata.Mode == PlanModeSync && p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGatewayConfigStore,
		storeRef,
		resources.ResourceTypeAIGatewayConfigStoreSecret,
	) {
		for _, current := range currentSecrets {
			if desiredKeys[current.Key] {
				continue
			}
			plan.AddChange(PlannedChange{
				ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayConfigStoreSecret, current.Key),
				ResourceType: ResourceTypeAIGatewayConfigStoreSecret,
				ResourceRef:  current.Key,
				ResourceID:   current.Key,
				Action:       ActionDelete,
				Namespace:    namespace,
				Fields:       map[string]any{FieldKey: current.Key},
				Parent:       &ParentInfo{Ref: storeRef, ID: storeID},
				References: map[string]ReferenceInfo{
					FieldAIGatewayID: {Ref: gatewayRef, ID: gatewayID},
				},
			})
		}
	}
	return nil
}

func (p *Planner) planAIGatewayConfigStoreSecretCreate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	storeRef string,
	storeID string,
	secret resources.AIGatewayConfigStoreSecretResource,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayConfigStoreSecret, secret.Ref),
		ResourceType: ResourceTypeAIGatewayConfigStoreSecret,
		ResourceRef:  secret.Ref,
		Action:       ActionCreate,
		Fields:       map[string]any{FieldKey: secret.Key},
		Namespace:    namespace,
		DependsOn:    dependsOn,
		References: map[string]ReferenceInfo{
			FieldAIGatewayID: {Ref: gatewayRef, ID: gatewayID},
		},
	}
	if storeID != "" {
		change.Parent = &ParentInfo{Ref: storeRef, ID: storeID}
	} else {
		change.References[FieldConfigStoreID] = ReferenceInfo{
			Ref: storeRef,
			LookupFields: map[string]string{
				FieldName: storeRef,
			},
		}
	}
	plan.AddChange(change)
}

func (p *Planner) aiGatewayConfigStoreSecretHasValueSource(ref string) bool {
	declarations := p.resources.SecretSources[ref]
	_, ok := declarations["/value"]
	return ok
}

func missingAIGatewayConfigStoreSecretValueError(ref string) error {
	return fmt.Errorf(
		"AI Gateway Config Store secret %q does not exist and requires value: !secret with a deferred source",
		ref,
	)
}
