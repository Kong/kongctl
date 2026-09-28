package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// planEventGatewayBackendClusterChanges plans changes for Event Gateway Backend Clusters for a specific gateway
func (p *Planner) planEventGatewayBackendClusterChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewayBackendClusterResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning Event Gateway Backend Cluster changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		// Gateway exists: full diff
		return p.planBackendClusterChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist: plan creates only with dependency on gateway creation
	p.planBackendClusterCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
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
	p.logger.Debug(
		"Planning changes for existing gateway backend clusters",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	// 1. List current backend clusters for this gateway
	currentClusters, err := p.client.ListEventGatewayBackendClusters(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list backend clusters for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug(
		"Fetched current backend clusters",
		"gateway_id", gatewayID,
		"current_count", len(currentClusters),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayBackendCluster)
	for _, cluster := range currentClusters {
		currentByName[cluster.Name] = cluster
	}

	return reconcileMappedChildren(p, desired, currentByName,
		mappedChildOperations[resources.EventGatewayBackendClusterResource, state.EventGatewayBackendCluster]{
			desiredName: func(desired resources.EventGatewayBackendClusterResource) string { return desired.Name },
			create: func(desiredCluster resources.EventGatewayBackendClusterResource) error {
				// CREATE
				p.logger.Debug(
					"Planning backend cluster CREATE",
					"cluster_name", desiredCluster.Name,
					"gateway_ref", gatewayRef,
				)
				p.planBackendClusterCreate(
					namespace,
					gatewayRef,
					gatewayName,
					gatewayID,
					desiredCluster,
					[]string{},
					plan,
				)
				return nil
			},
			matched: func(
				desiredCluster resources.EventGatewayBackendClusterResource,
				current state.EventGatewayBackendCluster,
			) error {
				// CHECK UPDATE
				p.logger.Debug(
					"Checking if backend cluster needs update",
					"cluster_name", desiredCluster.Name,
					"cluster_id", current.ID,
				)

				// Fetch full details if needed
				fullCluster, err := p.client.GetEventGatewayBackendCluster(ctx, gatewayID, current.ID)
				if err != nil {
					return fmt.Errorf("failed to get backend cluster %s: %w", current.ID, err)
				}
				if fullCluster == nil {
					return fmt.Errorf("backend cluster %s (%s) not found", desiredCluster.Name, current.ID)
				}

				needsUpdate, updateFields, changedFields := p.shouldUpdateBackendCluster(*fullCluster, desiredCluster)
				if needsUpdate {
					p.logger.Debug(
						"Planning backend cluster UPDATE",
						"cluster_name", desiredCluster.Name,
						"cluster_id", current.ID,
						"update_field_count", len(updateFields),
						"changed_field_count", len(changedFields),
					)
					p.planBackendClusterUpdate(
						namespace, gatewayRef, gatewayName, gatewayID,
						current.ID, desiredCluster, updateFields, changedFields, plan,
					)
				}

				return nil
			},
			remove: func(name string, current state.EventGatewayBackendCluster) {
				p.logger.Debug(
					"Planning backend cluster DELETE (sync mode)",
					"cluster_name", name,
					"cluster_id", current.ID,
				)
				p.planBackendClusterDelete(namespace, gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			},
		}, plan.Metadata.Mode)
}

// planBackendClusterCreatesForNewGateway plans creates for clusters when the gateway doesn't exist yet
func (p *Planner) planBackendClusterCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	clusters []resources.EventGatewayBackendClusterResource,
	plan *Plan,
) {
	p.logger.Debug(
		"Planning backend cluster creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"cluster_count", len(clusters),
	)

	// Build dependencies - backend clusters depend on gateway being created first
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, cluster := range clusters {
		p.planBackendClusterCreate(namespace, gatewayRef, gatewayName, "", cluster, dependsOn, plan)
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
	fields[FieldName] = cluster.Name

	if cluster.Description != nil {
		fields[FieldDescription] = *cluster.Description
	}

	fields[FieldAuthentication] = backendClusterAuthenticationFields(cluster.Authentication)
	fields[FieldBootstrapServers] = cluster.BootstrapServers
	fields[FieldTLS] = cluster.TLS

	if cluster.InsecureAllowAnonymousVirtualClusterAuth != nil {
		fields[FieldInsecureAllowAnonymousVirtualClusterAuth] = *cluster.InsecureAllowAnonymousVirtualClusterAuth
	}

	if cluster.MetadataUpdateIntervalSeconds != nil {
		fields[FieldMetadataUpdateIntervalSeconds] = *cluster.MetadataUpdateIntervalSeconds
	}

	if len(cluster.Labels) > 0 {
		fields[FieldLabels] = cluster.Labels
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
			FieldEventGatewayID: {
				Ref: gatewayRef,
				ID:  "", // to be resolved at runtime
				LookupFields: map[string]string{
					FieldName: gatewayName,
				},
			},
		}
	}

	p.logger.Debug(
		"Enqueuing backend cluster CREATE",
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
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayBackendCluster, cluster.Ref),
		ResourceType:  ResourceTypeEventGatewayBackendCluster,
		ResourceRef:   cluster.Ref,
		ResourceID:    clusterID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
	}

	change.Parent = &ParentInfo{
		Ref: gatewayRef,
		ID:  gatewayID,
	}

	p.logger.Debug(
		"Enqueuing backend cluster UPDATE",
		slog.String("cluster_ref", cluster.Ref),
		slog.String("cluster_id", clusterID),
		slog.String("gateway_ref", gatewayRef),
		slog.Int("field_count", len(updateFields)),
	)

	plan.AddChange(change)
}

// planBackendClusterDelete plans a DELETE change for a backend cluster
func (p *Planner) planBackendClusterDelete(
	namespace string,
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
		Namespace:    namespace,
	}

	change.Parent = &ParentInfo{
		Ref: gatewayRef,
		ID:  gatewayID,
	}

	p.logger.Debug(
		"Enqueuing backend cluster DELETE",
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
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changes := make(map[string]FieldChange)
	var needsUpdate bool

	// Compare name
	if current.Name != desired.Name {
		needsUpdate = true
		changes[FieldName] = FieldChange{
			Old: current.Name,
			New: desired.Name,
		}
	}

	// Compare description
	currentDesc := getString(current.Description)
	desiredDesc := getString(desired.Description)
	if currentDesc != desiredDesc {
		needsUpdate = true
		changes[FieldDescription] = FieldChange{
			Old: currentDesc,
			New: desiredDesc,
		}
	}

	// Compare authentication
	if !compareAuthenticationSchemes(current.Authentication, desired.Authentication) {
		needsUpdate = true
		changes[FieldAuthentication] = FieldChange{
			Old: current.Authentication,
			New: backendClusterAuthenticationFields(desired.Authentication),
		}
	}

	// Compare bootstrap servers
	if !slices.Equal(current.BootstrapServers, desired.BootstrapServers) {
		needsUpdate = true
		changes[FieldBootstrapServers] = FieldChange{
			Old: current.BootstrapServers,
			New: desired.BootstrapServers,
		}
	}

	// Compare TLS settings
	if !compareTLSSettings(current.TLS, desired.TLS) {
		needsUpdate = true
		changes[FieldTLS] = FieldChange{
			Old: current.TLS,
			New: desired.TLS,
		}
	}

	// Compare insecure flag
	if desired.InsecureAllowAnonymousVirtualClusterAuth != nil && !compareBoolPtrs(
		current.InsecureAllowAnonymousVirtualClusterAuth,
		desired.InsecureAllowAnonymousVirtualClusterAuth,
	) {
		needsUpdate = true
		changes[FieldInsecureAllowAnonymousVirtualClusterAuth] = FieldChange{
			Old: current.InsecureAllowAnonymousVirtualClusterAuth,
			New: *desired.InsecureAllowAnonymousVirtualClusterAuth,
		}
	}

	// Compare metadata update interval
	if desired.MetadataUpdateIntervalSeconds != nil &&
		!compareInt64Ptrs(current.MetadataUpdateIntervalSeconds, desired.MetadataUpdateIntervalSeconds) {
		needsUpdate = true
		changes[FieldMetadataUpdateIntervalSeconds] = FieldChange{
			Old: current.MetadataUpdateIntervalSeconds,
			New: *desired.MetadataUpdateIntervalSeconds,
		}
	}

	// Compare labels (user labels only, ignore KONGCTL labels)
	if desired.Labels != nil {
		if !maps.Equal(current.Labels, desired.Labels) {
			needsUpdate = true
			changes[FieldLabels] = FieldChange{
				Old: current.Labels,
				New: desired.Labels,
			}
		}
	}

	// If any changes detected, set ALL properties from desired state for PUT request
	if needsUpdate {
		updates[FieldName] = desired.Name

		if desired.Description != nil {
			updates[FieldDescription] = *desired.Description
		}

		updates[FieldAuthentication] = backendClusterAuthenticationFields(desired.Authentication)
		updates[FieldBootstrapServers] = desired.BootstrapServers
		updates[FieldTLS] = desired.TLS

		if desired.InsecureAllowAnonymousVirtualClusterAuth != nil {
			updates[FieldInsecureAllowAnonymousVirtualClusterAuth] = *desired.InsecureAllowAnonymousVirtualClusterAuth
		}

		if desired.MetadataUpdateIntervalSeconds != nil {
			updates[FieldMetadataUpdateIntervalSeconds] = *desired.MetadataUpdateIntervalSeconds
		}

		if len(desired.Labels) > 0 {
			updates[FieldLabels] = desired.Labels
		}
	}

	return needsUpdate, updates, changes
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

		plainA := a.BackendClusterAuthenticationSaslPlainSensitiveDataAware
		plainB := b.BackendClusterAuthenticationSaslPlain
		return plainA.Username == plainB.Username &&
			sameBackendPassword(plainA.Password, plainB.Password)
	case components.BackendClusterAuthenticationSensitiveDataAwareSchemeTypeSaslScram:
		if a.BackendClusterAuthenticationSaslScramSensitiveDataAware == nil ||
			b.BackendClusterAuthenticationSaslScram == nil {
			return false
		}

		scramA := a.BackendClusterAuthenticationSaslScramSensitiveDataAware
		scramB := b.BackendClusterAuthenticationSaslScram
		return scramA.Username == scramB.Username &&
			sameBackendPassword(scramA.Password, scramB.Password)
	}
	return false
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

func compareTLSSettings(a, b any) bool {
	// Type assert to the expected TLS types
	tlsA, okA := a.(components.BackendClusterTLS)
	tlsB, okB := b.(components.BackendClusterTLS)

	if !okA || !okB {
		// If types don't match, fall back to reflect.DeepEqual
		return reflect.DeepEqual(a, b)
	}

	if tlsA.Enabled != tlsB.Enabled {
		return false
	}

	// The API materializes false when insecure_skip_verify is omitted.
	if boolValueOrDefault(tlsA.InsecureSkipVerify, false) != boolValueOrDefault(tlsB.InsecureSkipVerify, false) {
		return false
	}

	if !compareStringPtrs(tlsA.CaBundle, tlsB.CaBundle) {
		return false
	}

	// The API materializes its supported TLS versions when the field is omitted.
	apiDefaultTLSVersions := []components.TLSVersions{
		components.TLSVersionsTls12,
		components.TLSVersionsTls13,
	}
	currentTLSVersions := tlsA.TLSVersions
	if currentTLSVersions == nil {
		currentTLSVersions = apiDefaultTLSVersions
	}
	desiredTLSVersions := tlsB.TLSVersions
	if desiredTLSVersions == nil {
		desiredTLSVersions = apiDefaultTLSVersions
	}
	return slices.Equal(currentTLSVersions, desiredTLSVersions)
}

func boolValueOrDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func compareStringPtrs(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func sameBackendPassword(current *string, desired string) bool {
	if tags.IsSecretPlaceholder(desired) || tags.IsEnvPlaceholder(desired) {
		return true
	}
	return current != nil && *current == desired
}

// Keep authentication traversable so secret-write preparation can remove deferred
// passwords from both request fields and changed-field metadata.
func backendClusterAuthenticationFields(auth components.BackendClusterAuthenticationScheme) map[string]any {
	fields := map[string]any{FieldType: string(auth.Type)}
	switch auth.Type {
	case components.BackendClusterAuthenticationSchemeTypeAnonymous:
	case components.BackendClusterAuthenticationSchemeTypeSaslPlain:
		if auth.BackendClusterAuthenticationSaslPlain != nil {
			fields["username"] = auth.BackendClusterAuthenticationSaslPlain.Username
			fields["password"] = auth.BackendClusterAuthenticationSaslPlain.Password
		}
	case components.BackendClusterAuthenticationSchemeTypeSaslScram:
		if auth.BackendClusterAuthenticationSaslScram != nil {
			fields["username"] = auth.BackendClusterAuthenticationSaslScram.Username
			fields["password"] = auth.BackendClusterAuthenticationSaslScram.Password
			fields["algorithm"] = string(auth.BackendClusterAuthenticationSaslScram.Algorithm)
		}
	}
	return fields
}
