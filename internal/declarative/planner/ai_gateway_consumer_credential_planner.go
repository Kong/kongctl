package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) planAIGatewayConsumerCredentialChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayID string,
	consumerRef string,
	consumerName string,
	consumerID string,
	desired []resources.AIGatewayConsumerCredentialResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Consumer Credential changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("consumer_ref", consumerRef),
		slog.String("consumer_id", consumerID),
		slog.Int("desired_count", len(desired)),
	)

	currentCredentials, err := p.client.ListAIGatewayConsumerCredentials(ctx, gatewayID, consumerID)
	if err != nil {
		return fmt.Errorf(
			"failed to list AI Gateway Consumer Credentials for consumer %s: %w",
			consumerID,
			err,
		)
	}

	currentByID, currentByName := indexAIGatewayConsumerCredentials(currentCredentials)
	desiredKeys := make(map[string]bool)

	for _, desiredCredential := range desired {
		current, exists := matchCurrentAIGatewayConsumerCredential(desiredCredential, currentByID, currentByName)
		desiredKeys[desiredCredential.Name] = true
		if id := aiGatewayConsumerCredentialDesiredID(desiredCredential); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			p.planAIGatewayConsumerCredentialCreate(
				namespace,
				gatewayRef,
				gatewayID,
				consumerRef,
				consumerName,
				consumerID,
				desiredCredential,
				nil,
				plan,
			)
			continue
		}

		credentialID := resources.AIGatewayConsumerCredentialID(current.AIGatewayConsumerCredential)
		if credential := p.resources.GetAIGatewayConsumerCredentialByRef(desiredCredential.Ref); credential != nil {
			credential.SetKonnectID(credentialID)
		}

		needsReplace, changedFields, err := shouldReplaceAIGatewayConsumerCredential(current, desiredCredential)
		if err != nil {
			return err
		}
		if !needsReplace {
			continue
		}

		deleteChangeID := p.planAIGatewayConsumerCredentialDelete(
			namespace,
			gatewayRef,
			gatewayID,
			consumerRef,
			consumerID,
			credentialID,
			desiredCredential.Ref,
			resources.AIGatewayConsumerCredentialName(current.AIGatewayConsumerCredential),
			changedFields,
			plan,
		)
		p.planAIGatewayConsumerCredentialCreate(
			namespace,
			gatewayRef,
			gatewayID,
			consumerRef,
			consumerName,
			consumerID,
			desiredCredential,
			[]string{deleteChangeID},
			plan,
		)
	}

	if plan.Metadata.Mode == PlanModeSync && !p.isAIGatewayExternal(gatewayRef) {
		for _, current := range currentCredentials {
			credentialID := resources.AIGatewayConsumerCredentialID(current.AIGatewayConsumerCredential)
			credentialName := resources.AIGatewayConsumerCredentialName(current.AIGatewayConsumerCredential)
			if desiredKeys[credentialID] || desiredKeys[credentialName] {
				continue
			}
			resourceRef := credentialName
			if resourceRef == "" {
				resourceRef = credentialID
			}
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(
				ResourceTypeAIGatewayConsumerCredential,
				resourceRef,
				isProtected,
				ActionDelete,
			); err != nil {
				return err
			}
			p.planAIGatewayConsumerCredentialDelete(
				namespace,
				gatewayRef,
				gatewayID,
				consumerRef,
				consumerID,
				credentialID,
				resourceRef,
				credentialName,
				nil,
				plan,
			)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayConsumerCredentialCreatesForNewConsumer(
	namespace string,
	gatewayRef string,
	gatewayID string,
	consumerRef string,
	consumerName string,
	consumerCreateID string,
	credentials []resources.AIGatewayConsumerCredentialResource,
	plan *Plan,
) {
	if len(credentials) == 0 {
		return
	}
	var dependsOn []string
	if consumerCreateID != "" {
		dependsOn = []string{consumerCreateID}
	}
	for _, credential := range credentials {
		p.planAIGatewayConsumerCredentialCreate(
			namespace,
			gatewayRef,
			gatewayID,
			consumerRef,
			consumerName,
			"",
			credential,
			dependsOn,
			plan,
		)
	}
}

func (p *Planner) planAIGatewayConsumerCredentialCreate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	consumerRef string,
	consumerName string,
	consumerID string,
	credential resources.AIGatewayConsumerCredentialResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := credential.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(
			credential.GetRef(),
			fmt.Sprintf("failed to build AI Gateway Consumer Credential create payload: %s", err),
		)
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayConsumerCredential, credential.Ref),
		ResourceType: ResourceTypeAIGatewayConsumerCredential,
		ResourceRef:  credential.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
		Parent:       &ParentInfo{Ref: consumerRef, ID: consumerID},
		References: map[string]ReferenceInfo{
			FieldAIGatewayID: {
				Ref: gatewayRef,
				ID:  gatewayID,
				LookupFields: map[string]string{
					FieldName: gatewayRef,
				},
			},
			FieldAIGatewayConsumerID: {
				Ref: consumerRef,
				ID:  consumerID,
				LookupFields: map[string]string{
					FieldName: consumerName,
				},
			},
		},
	}

	plan.AddChange(change)
}

func (p *Planner) planAIGatewayConsumerCredentialDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	consumerRef string,
	consumerID string,
	credentialID string,
	resourceRef string,
	credentialName string,
	changedFields map[string]FieldChange,
	plan *Plan,
) string {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionDelete, ResourceTypeAIGatewayConsumerCredential, resourceRef),
		ResourceType:  ResourceTypeAIGatewayConsumerCredential,
		ResourceRef:   resourceRef,
		ResourceID:    credentialID,
		Action:        ActionDelete,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Fields: map[string]any{
			FieldName: credentialName,
		},
		Parent: &ParentInfo{Ref: consumerRef, ID: consumerID},
		References: map[string]ReferenceInfo{
			FieldAIGatewayID: {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			FieldAIGatewayConsumerID: {
				Ref: consumerRef,
				ID:  consumerID,
			},
		},
	}
	plan.AddChange(change)
	return change.ID
}

func shouldReplaceAIGatewayConsumerCredential(
	current state.AIGatewayConsumerCredential,
	desired resources.AIGatewayConsumerCredentialResource,
) (bool, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayConsumerCredentialMutablePayloadMap(
		current.AIGatewayConsumerCredential,
	)
	if err != nil {
		return false, nil, fmt.Errorf("failed to normalize current AI Gateway Consumer Credential: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Consumer Credential %q: %w",
			desired.Ref,
			err,
		)
	}

	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil
	}
	return true, changedFields, nil
}

func indexAIGatewayConsumerCredentials(
	credentials []state.AIGatewayConsumerCredential,
) (map[string]state.AIGatewayConsumerCredential, map[string]state.AIGatewayConsumerCredential) {
	byID := make(map[string]state.AIGatewayConsumerCredential)
	byName := make(map[string]state.AIGatewayConsumerCredential)
	for _, credential := range credentials {
		if id := resources.AIGatewayConsumerCredentialID(credential.AIGatewayConsumerCredential); id != "" {
			byID[id] = credential
		}
		if name := resources.AIGatewayConsumerCredentialName(credential.AIGatewayConsumerCredential); name != "" {
			byName[name] = credential
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayConsumerCredential(
	desired resources.AIGatewayConsumerCredentialResource,
	currentByID map[string]state.AIGatewayConsumerCredential,
	currentByName map[string]state.AIGatewayConsumerCredential,
) (state.AIGatewayConsumerCredential, bool) {
	if id := aiGatewayConsumerCredentialDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name]
	return current, exists
}

func aiGatewayConsumerCredentialDesiredID(desired resources.AIGatewayConsumerCredentialResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}
