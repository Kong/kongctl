package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayDataPlaneCertificateChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayDataPlaneCertificateResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway data plane certificate changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayDataPlaneCertificateCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			desired,
			plan,
		)
		return nil
	}

	currentCerts, err := p.client.ListAIGatewayDataPlaneCertificates(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway data plane certificates for gateway %s: %w", gatewayID, err)
	}

	currentByTitle := indexAIGatewayDataPlaneCertificatesByTitle(gatewayRef, gatewayName, currentCerts, plan)
	desiredTitles := make(map[string]bool, len(desired))

	for _, desiredCert := range desired {
		desiredTitles[desiredCert.Title] = true
		current, exists := currentByTitle[desiredCert.Title]
		if !exists {
			p.planAIGatewayDataPlaneCertificateCreate(
				namespace,
				gatewayRef,
				gatewayName,
				gatewayID,
				desiredCert,
				nil,
				plan,
			)
			continue
		}

		needsReplace, changedFields := shouldReplaceAIGatewayDataPlaneCertificate(current, desiredCert)
		if !needsReplace {
			continue
		}

		certificateID := resources.AIGatewayDataPlaneCertificateID(current.AIGatewayDataPlaneClientCertificate)
		deleteChangeID := p.planAIGatewayDataPlaneCertificateDelete(
			gatewayRef,
			gatewayID,
			certificateID,
			desiredCert.Ref,
			desiredCert.Title,
			changedFields,
			plan,
		)
		p.planAIGatewayDataPlaneCertificateCreate(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayID,
			desiredCert,
			[]string{deleteChangeID},
			plan,
		)
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentCerts {
			title := resources.AIGatewayDataPlaneCertificateTitle(current.AIGatewayDataPlaneClientCertificate)
			if desiredTitles[title] {
				continue
			}
			certificateID := resources.AIGatewayDataPlaneCertificateID(current.AIGatewayDataPlaneClientCertificate)
			resourceRef := title
			if resourceRef == "" {
				resourceRef = certificateID
			}
			p.planAIGatewayDataPlaneCertificateDelete(
				gatewayRef,
				gatewayID,
				certificateID,
				resourceRef,
				title,
				nil,
				plan,
			)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayDataPlaneCertificateCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	certs []resources.AIGatewayDataPlaneCertificateResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, cert := range certs {
		p.planAIGatewayDataPlaneCertificateCreate(namespace, gatewayRef, gatewayName, "", cert, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayDataPlaneCertificateCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	cert resources.AIGatewayDataPlaneCertificateResource,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayDataPlaneCertificate, cert.Ref),
		ResourceType: ResourceTypeAIGatewayDataPlaneCertificate,
		ResourceRef:  cert.Ref,
		Action:       ActionCreate,
		Fields:       cert.PayloadMap(),
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}
	if gatewayID != "" {
		change.Parent = &ParentInfo{Ref: gatewayRef, ID: gatewayID}
	} else {
		change.References = map[string]ReferenceInfo{
			FieldAIGatewayID: {
				Ref: gatewayRef,
				LookupFields: map[string]string{
					FieldName: gatewayRef,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAIGatewayDataPlaneCertificateDelete(
	gatewayRef string,
	gatewayID string,
	certificateID string,
	resourceRef string,
	title string,
	changedFields map[string]FieldChange,
	plan *Plan,
) string {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionDelete, ResourceTypeAIGatewayDataPlaneCertificate, resourceRef),
		ResourceType:  ResourceTypeAIGatewayDataPlaneCertificate,
		ResourceRef:   resourceRef,
		ResourceID:    certificateID,
		Action:        ActionDelete,
		ChangedFields: changedFields,
		Fields: map[string]any{
			FieldTitle: title,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
	return change.ID
}

func shouldReplaceAIGatewayDataPlaneCertificate(
	current state.AIGatewayDataPlaneCertificate,
	desired resources.AIGatewayDataPlaneCertificateResource,
) (bool, map[string]FieldChange) {
	changedFields := make(map[string]FieldChange)
	if current.Cert != desired.Cert {
		changedFields[FieldCert] = FieldChange{Old: current.Cert, New: desired.Cert}
	}
	if descriptionPlanValue(current.Description) != descriptionPlanValue(desired.Description) {
		changedFields[FieldDescription] = FieldChange{
			Old: descriptionPlanValue(current.Description),
			New: descriptionPlanValue(desired.Description),
		}
	}
	if len(changedFields) == 0 {
		return false, nil
	}
	return true, changedFields
}

func descriptionPlanValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func indexAIGatewayDataPlaneCertificatesByTitle(
	gatewayRef string,
	gatewayName string,
	certs []state.AIGatewayDataPlaneCertificate,
	plan *Plan,
) map[string]state.AIGatewayDataPlaneCertificate {
	byTitle := make(map[string]state.AIGatewayDataPlaneCertificate)
	for _, cert := range certs {
		title := resources.AIGatewayDataPlaneCertificateTitle(cert.AIGatewayDataPlaneClientCertificate)
		if title == "" {
			continue
		}
		if _, exists := byTitle[title]; exists {
			plan.AddWarning(gatewayRef, fmt.Sprintf(
				"multiple AI Gateway data plane certificates with title %q exist on AI Gateway %q; using one match",
				title,
				gatewayName,
			))
			continue
		}
		byTitle[title] = cert
	}
	return byTitle
}
