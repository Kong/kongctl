package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planControlPlaneDataPlaneCertificateChanges(
	ctx context.Context,
	namespace string,
	controlPlaneName string,
	controlPlaneID string,
	controlPlaneRef string,
	controlPlaneChangeID string,
	desired []resources.ControlPlaneDataPlaneCertificateResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning control plane data plane certificate changes",
		"control_plane_name", controlPlaneName,
		"control_plane_id", controlPlaneID,
		"control_plane_ref", controlPlaneRef,
		"control_plane_change_id", controlPlaneChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if controlPlaneID == "" {
		p.planControlPlaneDataPlaneCertificateCreatesForNewControlPlane(
			namespace, controlPlaneRef, controlPlaneName, controlPlaneChangeID, desired, plan,
		)
		return nil
	}

	return p.planControlPlaneDataPlaneCertificateDiff(
		ctx, namespace, controlPlaneID, controlPlaneRef, controlPlaneName, desired, plan,
	)
}

func (p *Planner) planControlPlaneDataPlaneCertificateDiff(
	ctx context.Context,
	namespace string,
	controlPlaneID string,
	controlPlaneRef string,
	controlPlaneName string,
	desired []resources.ControlPlaneDataPlaneCertificateResource,
	plan *Plan,
) error {
	currentCerts, err := p.client.ListControlPlaneDataPlaneCertificates(ctx, controlPlaneID)
	if err != nil {
		return fmt.Errorf("failed to list data plane certificates for control plane %s: %w", controlPlaneID, err)
	}

	p.logger.Debug("Fetched current control plane data plane certificates",
		"control_plane_id", controlPlaneID,
		"current_count", len(currentCerts),
	)

	currentByIdentity := make(map[string]state.ControlPlaneDataPlaneCertificate)
	for _, cert := range currentCerts {
		certValue := stringPointerValue(cert.Cert)
		id := stringPointerValue(cert.ID)
		if certValue == "" || id == "" {
			continue
		}
		identity := resources.ControlPlaneDataPlaneCertificateIdentity(certValue)
		if _, exists := currentByIdentity[identity]; exists {
			plan.AddWarning(
				"",
				fmt.Sprintf(
					"multiple data plane certificates with fingerprint %s exist on control plane %q; using one match",
					resources.ShortControlPlaneDataPlaneCertificateIdentity(certValue),
					controlPlaneName,
				),
			)
			continue
		}
		currentByIdentity[identity] = cert
	}

	desiredIdentities := make(map[string]bool)
	var createChangeIDs []string
	for _, desiredCert := range desired {
		identity := resources.ControlPlaneDataPlaneCertificateIdentity(desiredCert.Cert)
		desiredIdentities[identity] = true
		if _, exists := currentByIdentity[identity]; exists {
			p.logger.Debug("Data plane certificate already exists",
				"cert_ref", desiredCert.Ref,
				"cert_fingerprint", desiredCert.GetMoniker(),
				"control_plane_ref", controlPlaneRef,
			)
			continue
		}

		changeID := p.planControlPlaneDataPlaneCertificateCreate(
			namespace,
			controlPlaneRef,
			controlPlaneName,
			controlPlaneID,
			desiredCert,
			nil,
			plan,
		)
		createChangeIDs = append(createChangeIDs, changeID)
	}

	if plan.Metadata.Mode == PlanModeSync {
		for identity, current := range currentByIdentity {
			if desiredIdentities[identity] {
				continue
			}

			certValue := stringPointerValue(current.Cert)
			p.planControlPlaneDataPlaneCertificateDelete(
				controlPlaneRef,
				controlPlaneID,
				stringPointerValue(current.ID),
				resources.ShortControlPlaneDataPlaneCertificateIdentity(certValue),
				createChangeIDs,
				plan,
			)
		}
	}

	return nil
}

func (p *Planner) planControlPlaneDataPlaneCertificateCreatesForNewControlPlane(
	namespace string,
	controlPlaneRef string,
	controlPlaneName string,
	controlPlaneChangeID string,
	certificates []resources.ControlPlaneDataPlaneCertificateResource,
	plan *Plan,
) {
	var dependsOn []string
	if controlPlaneChangeID != "" {
		dependsOn = []string{controlPlaneChangeID}
	}

	for _, cert := range certificates {
		p.planControlPlaneDataPlaneCertificateCreate(
			namespace, controlPlaneRef, controlPlaneName, "", cert, dependsOn, plan,
		)
	}
}

func (p *Planner) planControlPlaneDataPlaneCertificateCreate(
	namespace string,
	controlPlaneRef string,
	controlPlaneName string,
	controlPlaneID string,
	cert resources.ControlPlaneDataPlaneCertificateResource,
	dependsOn []string,
	plan *Plan,
) string {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeControlPlaneDataPlaneCertificate, cert.Ref),
		ResourceType: ResourceTypeControlPlaneDataPlaneCertificate,
		ResourceRef:  cert.Ref,
		Action:       ActionCreate,
		Fields: map[string]any{
			FieldCert: cert.Cert,
		},
		Namespace: namespace,
		DependsOn: dependsOn,
	}

	if controlPlaneID != "" {
		change.Parent = &ParentInfo{
			Ref: controlPlaneRef,
			ID:  controlPlaneID,
		}
	} else {
		change.References = map[string]ReferenceInfo{
			FieldControlPlaneID: {
				Ref: controlPlaneRef,
				LookupFields: map[string]string{
					FieldName: controlPlaneName,
				},
			},
		}
	}

	p.logger.Debug("Enqueuing control plane data plane certificate CREATE",
		"cert_ref", cert.Ref,
		"cert_fingerprint", cert.GetMoniker(),
		"control_plane_ref", controlPlaneRef,
	)
	plan.AddChange(change)
	return change.ID
}

func (p *Planner) planControlPlaneDataPlaneCertificateDelete(
	controlPlaneRef string,
	controlPlaneID string,
	certID string,
	certFingerprint string,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeControlPlaneDataPlaneCertificate, certFingerprint),
		ResourceType: ResourceTypeControlPlaneDataPlaneCertificate,
		ResourceRef:  certFingerprint,
		ResourceID:   certID,
		Action:       ActionDelete,
		DependsOn:    dependsOn,
		Parent: &ParentInfo{
			Ref: controlPlaneRef,
			ID:  controlPlaneID,
		},
	}

	p.logger.Debug("Enqueuing control plane data plane certificate DELETE",
		"cert_fingerprint", certFingerprint,
		"cert_id", certID,
		"control_plane_ref", controlPlaneRef,
	)
	plan.AddChange(change)
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
