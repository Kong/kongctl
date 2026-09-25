package planner

// Backend deletion cascades to its virtual clusters. Delete explicitly pruned
// virtual clusters first, using their observed destinations rather than desired
// resources (which no longer contain the deleted children).
func adjustEventGatewayBackendClusterDeleteDependencies(changes []PlannedChange) {
	type backendKey struct {
		namespace, gatewayID, backendID string
	}
	backends := make(map[backendKey]int)
	for i := range changes {
		change := &changes[i]
		if change.Action == ActionDelete && change.ResourceType == ResourceTypeEventGatewayBackendCluster &&
			change.Parent != nil && change.Parent.ID != "" && change.ResourceID != "" {
			backends[backendKey{change.Namespace, change.Parent.ID, change.ResourceID}] = i
		}
	}
	for _, change := range changes {
		if change.Action != ActionDelete || change.ResourceType != ResourceTypeEventGatewayVirtualCluster ||
			change.Parent == nil {
			continue
		}
		backendID := change.References[FieldEventGatewayBackendClusterID].ID
		if i, ok := backends[backendKey{change.Namespace, change.Parent.ID, backendID}]; ok {
			changes[i].DependsOn = appendDependsOn(changes[i].DependsOn, change.ID)
		}
	}
}
