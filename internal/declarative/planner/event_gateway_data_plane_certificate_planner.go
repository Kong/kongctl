package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayDataPlaneCertificateChanges plans changes for Event Gateway Data Plane Certificates
// for a specific gateway
func (p *Planner) planEventGatewayDataPlaneCertificateChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewayDataPlaneCertificateResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Data Plane Certificate changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		// Gateway exists: full diff
		return p.planDataPlaneCertificateChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist: plan creates only with dependency on gateway creation
	p.planDataPlaneCertificateCreatesForNewGateway(
		namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
	return nil
}

// planDataPlaneCertificateChangesForExistingGateway handles full diff for certificates
// of an existing gateway
func (p *Planner) planDataPlaneCertificateChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewayDataPlaneCertificateResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway data plane certificates",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	// 1. List current data plane certificates for this gateway
	currentCerts, err := p.listDataPlaneCertificates(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list data plane certificates for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current data plane certificates",
		"gateway_id", gatewayID,
		"current_count", len(currentCerts),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayDataPlaneCertificate)
	for _, cert := range currentCerts {
		if cert.Name != nil {
			currentByName[*cert.Name] = cert
		}
	}

	// 3. Compare desired vs current
	desiredNames := make(map[string]bool)
	for _, desiredCert := range desired {
		certName := desiredCert.GetMoniker()
		desiredNames[certName] = true
		current, exists := currentByName[certName]
		if !exists {
			// CREATE
			p.logger.Debug("Planning data plane certificate CREATE",
				"cert_name", certName,
				"gateway_ref", gatewayRef,
			)
			p.planDataPlaneCertificateCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredCert, []string{}, plan,
			)
		} else {
			// CHECK UPDATE
			p.logger.Debug("Checking if data plane certificate needs update",
				"cert_name", certName,
				"cert_id", current.ID,
			)

			needsUpdate, updateFields := p.shouldUpdateDataPlaneCertificate(current, desiredCert)
			if needsUpdate {
				p.logger.Debug("Planning data plane certificate UPDATE",
					"cert_name", certName,
					"cert_id", current.ID,
					"update_fields", updateFields,
				)
				p.planDataPlaneCertificateUpdate(
					namespace, gatewayRef, gatewayName, gatewayID,
					current.ID, desiredCert, updateFields, plan)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged certificates
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning data plane certificate DELETE (sync mode)",
					"cert_name", name,
					"cert_id", current.ID,
				)
				p.planDataPlaneCertificateDelete(gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planDataPlaneCertificateCreatesForNewGateway plans creates for certificates when the gateway
// doesn't exist yet
func (p *Planner) planDataPlaneCertificateCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	certificates []resources.EventGatewayDataPlaneCertificateResource,
	plan *Plan,
) {
	p.logger.Debug("Planning data plane certificate creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"cert_count", len(certificates),
	)

	// Build dependencies - certificates depend on gateway being created first
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, cert := range certificates {
		p.planDataPlaneCertificateCreate(
			namespace, gatewayRef, gatewayName, "", cert, dependsOn, plan,
		)
	}
}

// planDataPlaneCertificateCreate plans a CREATE change for a data plane certificate
func (p *Planner) planDataPlaneCertificateCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	cert resources.EventGatewayDataPlaneCertificateResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := make(map[string]any)
	fields["certificate"] = cert.Certificate
	if cert.Name != nil {
		fields["name"] = *cert.Name
	}
	if cert.Description != nil {
		fields["description"] = *cert.Description
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayDataPlaneCertificate, cert.Ref),
		ResourceType: ResourceTypeEventGatewayDataPlaneCertificate,
		ResourceRef:  cert.Ref,
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

	p.logger.Debug("Enqueuing data plane certificate CREATE",
		"cert_ref", cert.Ref,
		"cert_name", cert.GetMoniker(),
		"gateway_ref", gatewayRef,
	)
	plan.AddChange(change)
}

// planDataPlaneCertificateUpdate plans an UPDATE change for a data plane certificate
func (p *Planner) planDataPlaneCertificateUpdate(
	namespace string,
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	certID string,
	cert resources.EventGatewayDataPlaneCertificateResource,
	updateFields map[string]any,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayDataPlaneCertificate, cert.Ref),
		ResourceType: ResourceTypeEventGatewayDataPlaneCertificate,
		ResourceRef:  cert.Ref,
		ResourceID:   certID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing data plane certificate UPDATE",
		"cert_ref", cert.Ref,
		"cert_name", cert.GetMoniker(),
		"cert_id", certID,
		"fields", updateFields,
	)
	plan.AddChange(change)
}

// planDataPlaneCertificateDelete plans a DELETE change for a data plane certificate
func (p *Planner) planDataPlaneCertificateDelete(
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	certID string,
	certName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayDataPlaneCertificate, certName),
		ResourceType: ResourceTypeEventGatewayDataPlaneCertificate,
		ResourceRef:  certName,
		ResourceID:   certID,
		Action:       ActionDelete,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing data plane certificate DELETE",
		"cert_name", certName,
		"cert_id", certID,
	)
	plan.AddChange(change)
}

// shouldUpdateDataPlaneCertificate compares current and desired certificate state
func (p *Planner) shouldUpdateDataPlaneCertificate(
	current state.EventGatewayDataPlaneCertificate,
	desired resources.EventGatewayDataPlaneCertificateResource,
) (bool, map[string]any) {
	updates := make(map[string]any)
	var needsUpdate bool

	// Compare certificate content
	if current.Certificate != desired.Certificate {
		needsUpdate = true
	}

	// Compare name
	currentName := ""
	if current.Name != nil {
		currentName = *current.Name
	}
	desiredName := ""
	if desired.Name != nil {
		desiredName = *desired.Name
	}
	if currentName != desiredName {
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

	// If any changes detected, set ALL properties from desired state for PUT request
	if needsUpdate {
		updates["certificate"] = desired.Certificate

		if desired.Name != nil {
			updates["name"] = *desired.Name
		}

		if desired.Description != nil {
			updates["description"] = *desired.Description
		}
	}

	return needsUpdate, updates
}

// listDataPlaneCertificates lists data plane certificates for a gateway using its own paginated fetch
// since the state client already has paginated implementation
func (p *Planner) listDataPlaneCertificates(
	ctx context.Context,
	gatewayID string,
) ([]state.EventGatewayDataPlaneCertificate, error) {
	return p.client.ListEventGatewayDataPlaneCertificates(ctx, gatewayID)
}
