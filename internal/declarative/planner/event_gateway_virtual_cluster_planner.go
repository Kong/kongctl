package planner

import (
	"context"
	"fmt"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayVirtualClusterChanges plans changes for Event Gateway Virtual Clusters for a specific gateway
func (p *Planner) planEventGatewayVirtualClusterChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewayVirtualClusterResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Virtual Cluster changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		// Gateway exists: full diff
		return p.planVirtualClusterChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist: plan creates only with dependency on gateway creation
	p.planVirtualClusterCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
	return nil
}

// planVirtualClusterChangesForExistingGateway handles full diff for clusters of an existing gateway
func (p *Planner) planVirtualClusterChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewayVirtualClusterResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway virtual clusters",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	// 1. List current virtual clusters for this gateway
	currentClusters, err := p.client.ListEventGatewayVirtualClusters(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list virtual clusters for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current virtual clusters",
		"gateway_id", gatewayID,
		"current_count", len(currentClusters),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayVirtualCluster)
	for _, cluster := range currentClusters {
		currentByName[cluster.Name] = cluster
	}

	// 3. Compare desired vs current
	desiredNames := make(map[string]bool)
	for _, desiredCluster := range desired {
		desiredNames[desiredCluster.Name] = true
		current, exists := currentByName[desiredCluster.Name]
		if !exists {
			// CREATE
			p.logger.Debug("Planning virtual cluster CREATE",
				"cluster_name", desiredCluster.Name,
				"gateway_ref", gatewayRef,
			)
			p.planVirtualClusterCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredCluster, []string{}, plan)
		} else {
			// CHECK UPDATE
			p.logger.Debug("Checking if virtual cluster needs update",
				"cluster_name", desiredCluster.Name,
				"cluster_id", current.ID,
			)

			// Fetch full details if needed
			fullCluster, err := p.client.GetEventGatewayVirtualCluster(ctx, gatewayID, current.ID)
			if err != nil {
				return fmt.Errorf("failed to get virtual cluster %s: %w", current.ID, err)
			}

			needsUpdate, updateFields := p.shouldUpdateVirtualCluster(*fullCluster, desiredCluster)
			if needsUpdate {
				p.logger.Debug("Planning virtual cluster UPDATE",
					"cluster_name", desiredCluster.Name,
					"cluster_id", current.ID,
					"update_fields", updateFields,
				)
				p.planVirtualClusterUpdate(
					namespace, gatewayRef, gatewayName, gatewayID,
					current.ID, desiredCluster, updateFields, plan)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged clusters
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning virtual cluster DELETE (sync mode)",
					"cluster_name", name,
					"cluster_id", current.ID,
				)
				p.planVirtualClusterDelete(gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planVirtualClusterCreatesForNewGateway plans creates for clusters when the gateway doesn't exist yet
func (p *Planner) planVirtualClusterCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	clusters []resources.EventGatewayVirtualClusterResource,
	plan *Plan,
) {
	p.logger.Debug("Planning virtual cluster creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"cluster_count", len(clusters),
	)

	// Build dependencies - virtual clusters depend on gateway being created first
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, cluster := range clusters {
		p.planVirtualClusterCreate(namespace, gatewayRef, gatewayName, "", cluster, dependsOn, plan)
	}
}

// planVirtualClusterCreate plans a CREATE change for a virtual cluster
func (p *Planner) planVirtualClusterCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	cluster resources.EventGatewayVirtualClusterResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := make(map[string]any)
	fields["name"] = cluster.Name
	if cluster.Description != nil {
		fields["description"] = *cluster.Description
	}
	fields["destination"] = cluster.Destination
	fields["authentication"] = cluster.Authentication
	fields["acl_mode"] = cluster.ACLMode
	fields["dns_label"] = cluster.DNSLabel
	if cluster.Namespace != nil {
		fields["namespace"] = cluster.Namespace
	}
	if len(cluster.Labels) > 0 {
		fields["labels"] = cluster.Labels
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayVirtualCluster, cluster.Ref),
		ResourceType: ResourceTypeEventGatewayVirtualCluster,
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
				ID:  "", // to be resolved at runtime
				LookupFields: map[string]string{
					"name": gatewayName,
				},
			},
		}
	}

	p.logger.Debug("Enqueuing virtual cluster CREATE",
		"cluster_ref", cluster.Ref,
		"cluster_name", cluster.Name,
		"gateway_ref", gatewayRef,
	)
	plan.AddChange(change)
}

// planVirtualClusterUpdate plans an UPDATE change for a virtual cluster
func (p *Planner) planVirtualClusterUpdate(
	namespace string,
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	clusterID string,
	cluster resources.EventGatewayVirtualClusterResource,
	updateFields map[string]any,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayVirtualCluster, cluster.Ref),
		ResourceType: ResourceTypeEventGatewayVirtualCluster,
		ResourceRef:  cluster.Ref,
		ResourceID:   clusterID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing virtual cluster UPDATE",
		"cluster_ref", cluster.Ref,
		"cluster_name", cluster.Name,
		"cluster_id", clusterID,
		"fields", updateFields,
	)
	plan.AddChange(change)
}

// planVirtualClusterDelete plans a DELETE change for a virtual cluster
func (p *Planner) planVirtualClusterDelete(
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	clusterID string,
	clusterName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayVirtualCluster, clusterName),
		ResourceType: ResourceTypeEventGatewayVirtualCluster,
		ResourceRef:  clusterName,
		ResourceID:   clusterID,
		Action:       ActionDelete,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing virtual cluster DELETE",
		"cluster_name", clusterName,
		"cluster_id", clusterID,
	)
	plan.AddChange(change)
}

// shouldUpdateVirtualCluster compares current and desired virtual cluster state
func (p *Planner) shouldUpdateVirtualCluster(
	current state.EventGatewayVirtualCluster,
	desired resources.EventGatewayVirtualClusterResource,
) (bool, map[string]any) {
	updates := make(map[string]any)
	var needsUpdate bool

	// Compare name
	if current.Name != desired.Name {
		needsUpdate = true
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := ""
	if desired.Description != nil {
		desiredDesc = *desired.Description
	}
	if currentDesc != desiredDesc {
		needsUpdate = true
	}

	// Compare destination
	if !compareBackendClusterReferences(current.Destination, desired.Destination) {
		needsUpdate = true
	}

	// Compare authentication
	if !compareAuthentication(current.Authentication, desired.Authentication) {
		needsUpdate = true
	}

	// Compare ACL mode
	if current.ACLMode != desired.ACLMode {
		needsUpdate = true
	}

	// Compare DNS label
	if current.DNSLabel != desired.DNSLabel {
		needsUpdate = true
	}

	// Compare namespace
	if !compareVirtualClusterNamespaces(current.Namespace, desired.Namespace) {
		needsUpdate = true
	}

	// Compare labels
	if desired.Labels != nil {
		if !compareMaps(current.Labels, desired.Labels) {
			needsUpdate = true
		}
	}

	// If any changes detected, set ALL properties from desired state for PUT request
	if needsUpdate {
		updates["name"] = desired.Name

		if desired.Description != nil {
			updates["description"] = *desired.Description
		}

		updates["destination"] = desired.Destination
		updates["authentication"] = desired.Authentication
		updates["acl_mode"] = desired.ACLMode
		updates["dns_label"] = desired.DNSLabel

		if desired.Namespace != nil {
			updates["namespace"] = desired.Namespace
		}

		if len(desired.Labels) > 0 {
			updates["labels"] = desired.Labels
		}
	}

	return needsUpdate, updates
}

// compareBackendClusterReferences compares backend cluster references
func compareBackendClusterReferences(
	current components.BackendClusterReference,
	desired components.BackendClusterReferenceModify,
) bool {
	// Compare based on type
	switch desired.Type {
	case components.BackendClusterReferenceModifyTypeBackendClusterReferenceByID:
		if desired.BackendClusterReferenceByID != nil {
			return current.ID == desired.BackendClusterReferenceByID.ID
		}
		return false
	case components.BackendClusterReferenceModifyTypeBackendClusterReferenceByName:
		if desired.BackendClusterReferenceByName != nil {
			return current.Name == desired.BackendClusterReferenceByName.Name
		}
		return false
	default:
		return false
	}
}

// compareAuthentication compares authentication configurations
func compareAuthentication(
	current []components.VirtualClusterAuthenticationSensitiveDataAwareScheme,
	desired []components.VirtualClusterAuthenticationScheme,
) bool {
	if len(current) != len(desired) {
		return false
	}

	// Compare each authentication scheme
	for i := range current {
		if string(current[i].Type) != string(desired[i].Type) {
			return false
		}

		// Compare type-specific fields
		switch current[i].Type {
		case components.VirtualClusterAuthenticationSensitiveDataAwareSchemeTypeAnonymous:
			// No additional fields to compare for anonymous
			continue

		case components.VirtualClusterAuthenticationSensitiveDataAwareSchemeTypeSaslPlain:
			// Compare mediation and principals for sasl_plain
			if current[i].VirtualClusterAuthenticationSaslPlainSensitiveDataAware == nil ||
				desired[i].VirtualClusterAuthenticationSaslPlain == nil {
				return false
			}

			currPlain := current[i].VirtualClusterAuthenticationSaslPlainSensitiveDataAware
			desiredPlain := desired[i].VirtualClusterAuthenticationSaslPlain

			// Compare mediation
			if string(currPlain.Mediation) != string(desiredPlain.Mediation) {
				return false
			}

			// Compare principals (username comparison only, password is sensitive)
			if len(currPlain.Principals) != len(desiredPlain.Principals) {
				return false
			}
			for j := range currPlain.Principals {
				if currPlain.Principals[j].Username != desiredPlain.Principals[j].Username {
					return false
				}

				if *currPlain.Principals[j].Password != desiredPlain.Principals[j].Password {
					return false
				}
			}

		case components.VirtualClusterAuthenticationSensitiveDataAwareSchemeTypeSaslScram:
			// Compare algorithm for sasl_scram
			if current[i].VirtualClusterAuthenticationSaslScram == nil ||
				desired[i].VirtualClusterAuthenticationSaslScram == nil {
				return false
			}
			if current[i].VirtualClusterAuthenticationSaslScram.Algorithm !=
				desired[i].VirtualClusterAuthenticationSaslScram.Algorithm {
				return false
			}

		case components.VirtualClusterAuthenticationSensitiveDataAwareSchemeTypeOauthBearer:
			// Compare oauth_bearer fields
			if current[i].VirtualClusterAuthenticationOauthBearer == nil ||
				desired[i].VirtualClusterAuthenticationOauthBearer == nil {
				return false
			}

			currOAuth := current[i].VirtualClusterAuthenticationOauthBearer
			desiredOAuth := desired[i].VirtualClusterAuthenticationOauthBearer

			// Compare mediation
			if string(currOAuth.Mediation) != string(desiredOAuth.Mediation) {
				return false
			}

			// Compare claims_mapping
			if (currOAuth.ClaimsMapping == nil) != (desiredOAuth.ClaimsMapping == nil) {
				return false
			}
			if currOAuth.ClaimsMapping != nil {
				if !stringPtrEqual(currOAuth.ClaimsMapping.Sub, desiredOAuth.ClaimsMapping.Sub) ||
					!stringPtrEqual(currOAuth.ClaimsMapping.Scope, desiredOAuth.ClaimsMapping.Scope) {
					return false
				}
			}

			// Compare jwks
			if (currOAuth.Jwks == nil) != (desiredOAuth.Jwks == nil) {
				return false
			}
			if currOAuth.Jwks != nil {
				if currOAuth.Jwks.Endpoint != desiredOAuth.Jwks.Endpoint ||
					!stringPtrEqual(currOAuth.Jwks.Timeout, desiredOAuth.Jwks.Timeout) ||
					!stringPtrEqual(currOAuth.Jwks.CacheExpiration, desiredOAuth.Jwks.CacheExpiration) {
					return false
				}
			}

			// Compare validate
			if (currOAuth.Validate == nil) != (desiredOAuth.Validate == nil) {
				return false
			}
			if currOAuth.Validate != nil {
				if !stringPtrEqual(currOAuth.Validate.Issuer, desiredOAuth.Validate.Issuer) {
					return false
				}
				if len(currOAuth.Validate.Audiences) != len(desiredOAuth.Validate.Audiences) {
					return false
				}
				for j := range currOAuth.Validate.Audiences {
					if currOAuth.Validate.Audiences[j].Name != desiredOAuth.Validate.Audiences[j].Name {
						return false
					}
				}
			}
		}
	}

	return true
}

// compareVirtualClusterNamespaces compares namespace configurations
func compareVirtualClusterNamespaces(
	current *components.VirtualClusterNamespace,
	desired *components.VirtualClusterNamespace,
) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}

	if current.Mode != desired.Mode {
		return false
	}

	if current.Prefix != desired.Prefix {
		return false
	}

	return true
}

// compareMaps compares two string maps
func compareMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}

	return true
}

// stringPtrEqual compares two string pointers for equality
func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
