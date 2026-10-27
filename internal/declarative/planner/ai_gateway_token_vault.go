package planner

import (
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
)

func addAIGatewayTokenVaultChanges(
	currentStores []kkComps.AIGatewayDatastoreRef,
	currentVault any,
	desired map[string]any,
	changed map[string]FieldChange,
	rs *resources.ResourceSet, gatewayRef string,
) error {
	current, err := resources.AIGatewayTokenVaultFields(currentStores, currentVault)
	if err != nil {
		return err
	}
	desiredCompare := normalizeAIGatewayJSONMap(desired)
	normalizeAIGateway23References(desiredCompare, rs, gatewayRef)
	for _, field := range []string{FieldDatastores, FieldTokenVault} {
		oldValue, newValue := current[field], desired[field]
		oldFields, newFields := comparableAIGatewayWriteOnlyPayloads(
			map[string]any{field: oldValue}, map[string]any{field: desiredCompare[field]},
			normalizeAIGatewayPayloadsForComparison, isAIGatewayUpstreamWriteOnlyField,
		)
		if !reflect.DeepEqual(oldFields, newFields) {
			changed[field] = FieldChange{Old: oldValue, New: newValue}
		}
	}
	return nil
}

func addAIGatewayDatastoreDependencies(plan *Plan, namespace, gatewayRef string) {
	var stores, consumers []int
	for i, change := range plan.Changes {
		if change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(change, gatewayRef) {
			continue
		}
		switch change.ResourceType {
		case ResourceTypeAIGatewayDatastore:
			stores = append(stores, i)
		case ResourceTypeAIGatewayProvider,
			ResourceTypeAIGatewayAuthStrategy,
			ResourceTypeAIGatewayMCPServer,
			ResourceTypeAIGatewayModel,
			ResourceTypeAIGatewayPolicy:
			consumers = append(consumers, i)
		}
	}
	for _, storeIndex := range stores {
		for _, consumerIndex := range consumers {
			store, consumer := &plan.Changes[storeIndex], &plan.Changes[consumerIndex]
			// Provision stores before consumers; detach consumers before deleting stores.
			if store.Action == ActionCreate && consumer.Action != ActionDelete {
				consumer.DependsOn = appendDependsOn(consumer.DependsOn, store.ID)
			} else if store.Action == ActionDelete {
				store.DependsOn = appendDependsOn(store.DependsOn, consumer.ID)
			}
		}
	}
}
