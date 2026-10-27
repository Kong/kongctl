package planner

// Deleting a schema-validation policy cascades to its children. Order explicit
// child deletes first so concurrent execution cannot race that cascade.
func adjustEventGatewayPolicyDeleteDependencies(
	changes []PlannedChange,
	resourceType string,
	parents map[string]string,
) {
	deletes := make(map[string]int)
	for i, change := range changes {
		if change.ResourceType == resourceType && change.Action == ActionDelete {
			deletes[change.ResourceID] = i
		}
	}
	for _, child := range changes {
		if child.ResourceType != resourceType || child.Action != ActionDelete {
			continue
		}
		if parent, deleted := deletes[parents[child.ResourceID]]; deleted {
			changes[parent].DependsOn = appendDependsOn(changes[parent].DependsOn, child.ID)
		}
	}
}

// Backend deletion cascades to its virtual clusters. Delete explicitly pruned
// virtual clusters first, using their observed destinations rather than desired
// resources (which no longer contain the deleted children).
func adjustEventGatewayBackendClusterDeleteDependencies(changes []PlannedChange) {
	type backendKey struct {
		namespace, gatewayID, backend string
	}
	backends := make(map[backendKey]int)
	backendsByName := make(map[backendKey]int)
	for i := range changes {
		change := &changes[i]
		if change.Action == ActionDelete && change.ResourceType == ResourceTypeEventGatewayBackendCluster &&
			change.Parent != nil && change.Parent.ID != "" && change.ResourceID != "" {
			backends[backendKey{change.Namespace, change.Parent.ID, change.ResourceID}] = i
			if change.ResourceRef != "" {
				backendsByName[backendKey{change.Namespace, change.Parent.ID, change.ResourceRef}] = i
			}
		}
	}
	for _, change := range changes {
		if change.Action != ActionDelete || change.ResourceType != ResourceTypeEventGatewayVirtualCluster ||
			change.Parent == nil {
			continue
		}
		reference := change.References[FieldEventGatewayBackendClusterID]
		i, ok := backends[backendKey{change.Namespace, change.Parent.ID, reference.ID}]
		// An observed ID is authoritative; only name-only destinations use the fallback.
		if reference.ID == "" {
			i, ok = backendsByName[backendKey{change.Namespace, change.Parent.ID, reference.LookupFields[FieldName]}]
		}
		if ok {
			changes[i].DependsOn = appendDependsOn(changes[i].DependsOn, change.ID)
		}
	}
}
