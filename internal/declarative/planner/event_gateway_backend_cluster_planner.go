package planner

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayBackendClusterChanges plans changes for Event Gateway Backend Clusters for a specific gateway
func (p *Planner) planEventGatewayBackendClusterChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	desired []resources.EventGatewayBackendClusterResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Backend Cluster changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		// Gateway exists: full diff
		return p.planBackendClusterChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist: plan creates only (dependencies will be set up)
	p.planBackendClusterCreatesForNewGateway(namespace, gatewayRef, gatewayName, desired, plan)
	return nil
}

// planBackendClusterChangesForExistingGateway handles full diff for clusters of an existing gateway
func (p *Planner) planBackendClusterChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewayBackendClusterResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway backend clusters",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	// 1. List current backend clusters for this gateway
	currentClusters, err := p.client.ListEventGatewayBackendClusters(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list backend clusters for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current backend clusters",
		"gateway_id", gatewayID,
		"current_count", len(currentClusters),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayBackendCluster)
	for _, cluster := range currentClusters {
		currentByName[cluster.Name] = cluster
	}

	desiredNames := make(map[string]bool)

	// 3. Compare desired vs current
	for _, desiredCluster := range desired {
		desiredNames[desiredCluster.Name] = true

		current, exists := currentByName[desiredCluster.Name]

		if !exists {
			// CREATE
			p.logger.Debug("Planning backend cluster CREATE",
				"cluster_name", desiredCluster.Name,
				"gateway_ref", gatewayRef,
			)
			p.planBackendClusterCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredCluster, []string{}, plan)
		} else {
			// CHECK UPDATE
			p.logger.Debug("Checking if backend cluster needs update",
				"cluster_name", desiredCluster.Name,
				"cluster_id", current.ID,
			)

			// Fetch full details if needed
			fullCluster, err := p.client.GetEventGatewayBackendCluster(ctx, gatewayID, current.ID)
			if err != nil {
				return fmt.Errorf("failed to get backend cluster %s: %w", current.ID, err)
			}

			needsUpdate, updateFields := p.shouldUpdateBackendCluster(*fullCluster, desiredCluster)
			if needsUpdate {
				p.logger.Debug("Planning backend cluster UPDATE",
					"cluster_name", desiredCluster.Name,
					"cluster_id", current.ID,
					"update_fields", updateFields,
				)
				p.planBackendClusterUpdate(
					namespace, gatewayRef, gatewayName, gatewayID,
					current.ID, desiredCluster, updateFields, plan)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged clusters
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning backend cluster DELETE (sync mode)",
					"cluster_name", name,
					"cluster_id", current.ID,
				)
				p.planBackendClusterDelete(gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planBackendClusterCreatesForNewGateway plans creates for clusters when the gateway doesn't exist yet
func (p *Planner) planBackendClusterCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	clusters []resources.EventGatewayBackendClusterResource,
	plan *Plan,
) {
	p.logger.Debug("Planning backend cluster creates for new gateway",
		"gateway_ref", gatewayRef,
		"cluster_count", len(clusters),
	)

	for _, cluster := range clusters {
		p.planBackendClusterCreate(namespace, gatewayRef, gatewayName, "", cluster, []string{}, plan)
	}
}

// planBackendClusterCreate plans a CREATE change for a backend cluster
func (p *Planner) planBackendClusterCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	cluster resources.EventGatewayBackendClusterResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := make(map[string]any)
	fields["name"] = cluster.Name

	if cluster.Description != nil {
		fields["description"] = *cluster.Description
	}

	fields["authentication"] = cluster.Authentication
	fields["bootstrap_servers"] = cluster.BootstrapServers
	fields["tls"] = cluster.TLS

	if cluster.InsecureAllowAnonymousVirtualClusterAuth != nil {
		fields["insecure_allow_anonymous_virtual_cluster_auth"] = *cluster.InsecureAllowAnonymousVirtualClusterAuth
	}

	if cluster.MetadataUpdateIntervalSeconds != nil {
		fields["metadata_update_interval_seconds"] = *cluster.MetadataUpdateIntervalSeconds
	}

	if len(cluster.Labels) > 0 {
		fields["labels"] = cluster.Labels
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayBackendCluster, cluster.Ref),
		ResourceType: ResourceTypeEventGatewayBackendCluster,
		ResourceRef:  cluster.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	// Set parent reference
	if gatewayID != "" {
		change.Parent = &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		}
	} else {
		// Gateway doesn't exist yet, add reference for runtime resolution
		change.References = map[string]ReferenceInfo{
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  "",
				LookupFields: map[string]string{
					"name": gatewayName,
				},
			},
		}
	}

	p.logger.Debug("Enqueuing backend cluster CREATE",
		slog.String("cluster_ref", cluster.Ref),
		slog.String("cluster_name", cluster.Name),
		slog.String("gateway_ref", gatewayRef),
	)

	plan.AddChange(change)
}

// planBackendClusterUpdate plans an UPDATE change for a backend cluster
func (p *Planner) planBackendClusterUpdate(
	namespace string,
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	clusterID string,
	cluster resources.EventGatewayBackendClusterResource,
	updateFields map[string]any,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayBackendCluster, cluster.Ref),
		ResourceType: ResourceTypeEventGatewayBackendCluster,
		ResourceRef:  cluster.Ref,
		ResourceID:   clusterID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
	}

	change.Parent = &ParentInfo{
		Ref: gatewayRef,
		ID:  gatewayID,
	}

	p.logger.Debug("Enqueuing backend cluster UPDATE",
		slog.String("cluster_ref", cluster.Ref),
		slog.String("cluster_id", clusterID),
		slog.String("gateway_ref", gatewayRef),
		slog.Any("fields", updateFields),
	)

	plan.AddChange(change)
}

// planBackendClusterDelete plans a DELETE change for a backend cluster
func (p *Planner) planBackendClusterDelete(
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	clusterID string,
	clusterName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayBackendCluster, clusterName),
		ResourceType: ResourceTypeEventGatewayBackendCluster,
		ResourceRef:  clusterName,
		ResourceID:   clusterID,
		Action:       ActionDelete,
		Fields:       map[string]any{},
	}

	change.Parent = &ParentInfo{
		Ref: gatewayRef,
		ID:  gatewayID,
	}

	p.logger.Debug("Enqueuing backend cluster DELETE",
		slog.String("cluster_name", clusterName),
		slog.String("cluster_id", clusterID),
		slog.String("gateway_ref", gatewayRef),
	)

	plan.AddChange(change)
}

// shouldUpdateBackendCluster compares current and desired state
func (p *Planner) shouldUpdateBackendCluster(
	current state.EventGatewayBackendCluster,
	desired resources.EventGatewayBackendClusterResource,
) (bool, map[string]any) {
	updates := make(map[string]any)
	var needsUpdate bool

	// Compare name
	if current.Name != desired.Name {
		needsUpdate = true
	}

	// Compare description
	currentDesc := getString(current.Description)
	desiredDesc := getString(desired.Description)
	if currentDesc != desiredDesc {
		needsUpdate = true
	}

	// Compare authentication
	if !compareAuthenticationSchemes(current.Authentication, desired.Authentication) {
		needsUpdate = true
	}

	// Compare bootstrap servers
	if !compareStringSlices(current.BootstrapServers, desired.BootstrapServers) {
		needsUpdate = true
	}

	// Compare TLS settings
	if !compareTLSSettings(current.TLS, desired.TLS) {
		needsUpdate = true
	}

	// Compare insecure flag
	if desired.InsecureAllowAnonymousVirtualClusterAuth != nil && !compareBoolPtrs(
		current.InsecureAllowAnonymousVirtualClusterAuth,
		desired.InsecureAllowAnonymousVirtualClusterAuth,
	) {
		needsUpdate = true
	}

	// Compare metadata update interval
	if desired.MetadataUpdateIntervalSeconds != nil &&
		!compareInt64Ptrs(current.MetadataUpdateIntervalSeconds, desired.MetadataUpdateIntervalSeconds) {
		needsUpdate = true
	}

	// Compare labels (user labels only, ignore KONGCTL labels)
	if desired.Labels != nil {
		if !compareStringMaps(current.Labels, desired.Labels) {
			needsUpdate = true
		}
	}

	// If any changes detected, set ALL properties from desired state for PUT request
	if needsUpdate {
		updates["name"] = desired.Name

		if desired.Description != nil {
			updates["description"] = *desired.Description
		}

		updates["authentication"] = desired.Authentication
		updates["bootstrap_servers"] = desired.BootstrapServers
		updates["tls"] = desired.TLS

		if desired.InsecureAllowAnonymousVirtualClusterAuth != nil {
			updates["insecure_allow_anonymous_virtual_cluster_auth"] = *desired.InsecureAllowAnonymousVirtualClusterAuth
		}

		if desired.MetadataUpdateIntervalSeconds != nil {
			updates["metadata_update_interval_seconds"] = *desired.MetadataUpdateIntervalSeconds
		}

		if len(desired.Labels) > 0 {
			updates["labels"] = desired.Labels
		}
	}

	return needsUpdate, updates
}

// Helper functions for comparisons
func compareAuthenticationSchemes(
	a components.BackendClusterAuthenticationSensitiveDataAwareScheme,
	b components.BackendClusterAuthenticationScheme,
) bool {
	if string(a.Type) != string(b.Type) {
		return false
	}

	switch a.Type {
	case components.BackendClusterAuthenticationSensitiveDataAwareSchemeTypeAnonymous:
		// Nothing to compare within anonymous
		return true
	case components.BackendClusterAuthenticationSensitiveDataAwareSchemeTypeSaslPlain:
		if a.BackendClusterAuthenticationSaslPlainSensitiveDataAware == nil ||
			b.BackendClusterAuthenticationSaslPlain == nil {
			return false
		}

		if a.BackendClusterAuthenticationSaslPlainSensitiveDataAware.Password == nil {
			// Password can be omitted in responses if literal value was used. Treat as unequal in that case.
			return false
		}

		plainA := a.BackendClusterAuthenticationSaslPlainSensitiveDataAware
		plainB := b.BackendClusterAuthenticationSaslPlain
		return plainA.Username == plainB.Username &&
			*plainA.Password == plainB.Password
	case components.BackendClusterAuthenticationSensitiveDataAwareSchemeTypeSaslScram:
		if a.BackendClusterAuthenticationSaslScramSensitiveDataAware == nil ||
			b.BackendClusterAuthenticationSaslScram == nil {
			return false
		}

		if a.BackendClusterAuthenticationSaslScramSensitiveDataAware.Password == nil {
			// Password can be omitted in responses if literal value was used. Treat as unequal in that case.
			return false
		}

		scramA := a.BackendClusterAuthenticationSaslScramSensitiveDataAware
		scramB := b.BackendClusterAuthenticationSaslScram
		return scramA.Username == scramB.Username &&
			*scramA.Password == scramB.Password
	}
	return false
}

func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func compareBoolPtrs(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func compareInt64Ptrs(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func compareStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func compareTLSSettings(a, b interface{}) bool {
	// For now, do a simple comparison
	// This can be enhanced to do deep comparison of TLS fields
	return reflect.DeepEqual(a, b)
}
