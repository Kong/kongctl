package planner

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (p *Planner) planAIGatewayTLSChanges(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, gatewayChangeID string,
	plan *Plan,
) error {
	certificateScoped := p.shouldPlanChild(
		plan, resources.ResourceTypeAIGateway, gatewayRef, resources.ResourceTypeAIGatewayCertificate,
	)
	caScoped := p.shouldPlanChild(
		plan, resources.ResourceTypeAIGateway, gatewayRef, resources.ResourceTypeAIGatewayCACertificate,
	)
	sniScoped := p.shouldPlanChild(
		plan, resources.ResourceTypeAIGateway, gatewayRef, resources.ResourceTypeAIGatewaySNI,
	)
	certificates := p.resources.GetAIGatewayCertificatesForGateway(gatewayRef)
	caCertificates := p.resources.GetAIGatewayCACertificatesForGateway(gatewayRef)
	snis := p.resources.GetAIGatewaySNIsForGateway(gatewayRef)

	var pendingDeletes []PlannedChange
	if certificateScoped && (len(certificates) > 0 || plan.Metadata.Mode == PlanModeSync) {
		var err error
		pendingDeletes, err = p.planAIGatewayCertificateUpserts(
			ctx, namespace, gatewayRef, gatewayID, gatewayChangeID, certificates, plan,
		)
		if err != nil {
			return err
		}
	}
	if caScoped && (len(caCertificates) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayCACertificateChanges(
			ctx, namespace, gatewayRef, gatewayID, gatewayChangeID, caCertificates, plan,
		); err != nil {
			return err
		}
	}

	if sniScoped && (len(snis) > 0 || plan.Metadata.Mode == PlanModeSync) {
		err := p.planAIGatewaySNIChanges(
			ctx, namespace, gatewayRef, gatewayID, gatewayChangeID, snis, plan,
		)
		if err != nil {
			return err
		}
	}
	for _, deletion := range pendingDeletes {
		plan.AddChange(deletion)
	}
	return nil
}

func (p *Planner) planAIGatewayCertificateUpserts(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, gatewayChangeID string,
	desired []resources.AIGatewayCertificateResource,
	plan *Plan,
) ([]PlannedChange, error) {
	p.logger.Debug("Planning AI Gateway certificate changes",
		slog.String("gateway_ref", gatewayRef), slog.Int("desired_count", len(desired)))
	if gatewayID == "" {
		for _, certificate := range desired {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewayCertificate, namespace, gatewayRef, gatewayChangeID,
				"", certificate.Ref, certificate.PayloadMap(), plan,
			)
		}
		return nil, nil
	}
	current, err := p.listAIGatewayCertificates(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to list AI Gateway certificates: %w", err)
	}
	byName := indexAIGatewayCertificates(current)
	desiredNames := make(map[string]bool, len(desired))
	for i := range desired {
		certificate := &desired[i]
		matched, exists := byName[certificate.Name]
		desiredNames[certificate.Name] = true
		if !exists {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewayCertificate, namespace, gatewayRef, "",
				gatewayID, certificate.Ref, certificate.PayloadMap(), plan,
			)
			continue
		}
		certificate.SetKonnectID(matched.ID)
		fields, changed := diffAIGatewayCertificate(matched, *certificate)
		if len(changed) > 0 {
			p.planAIGatewayTLSUpdate(
				ResourceTypeAIGatewayCertificate, namespace, gatewayRef, gatewayID,
				matched.ID, certificate.Ref, fields, changed, nil, plan,
			)
		}
	}

	var deletes []PlannedChange
	if plan.Metadata.Mode == PlanModeSync {
		for _, certificate := range current {
			if desiredNames[certificate.Name] {
				continue
			}
			deletes = append(deletes, newAIGatewayTLSDelete(
				p, ResourceTypeAIGatewayCertificate, namespace, gatewayRef, gatewayID,
				certificate.ID, certificate.Name,
			))
		}
	}
	return deletes, nil
}

func (p *Planner) planAIGatewayCACertificateChanges(
	ctx context.Context,
	_, gatewayRef, gatewayID, gatewayChangeID string,
	desired []resources.AIGatewayCACertificateResource,
	plan *Plan,
) error {
	create := func(child resources.AIGatewayCACertificateResource) {
		fields := child.PayloadMap()
		fields[FieldCert] = strings.TrimSpace(child.Cert)
		fields[FieldManagedBy] = resources.BuildChildManagedBy(
			nil,
			child.ManagedBy,
			resources.GetNamespace(child.Kongctl),
			child.Protected(),
			"",
		)
		before := len(plan.Changes)
		p.planAIGatewayTLSCreate(
			ResourceTypeAIGatewayCACertificate,
			resources.GetNamespace(child.Kongctl),
			gatewayRef,
			gatewayChangeID,
			gatewayID,
			child.Ref,
			fields,
			plan,
		)
		plan.Changes[before].Protection = child.Protected()
	}
	if gatewayID == "" {
		for _, child := range desired {
			create(child)
		}
		return nil
	}
	current, err := p.listAIGatewayCACertificates(ctx, gatewayID)
	if err != nil {
		return err
	}
	return reconcileNameMatchedChildren(
		p,
		ResourceTypeAIGatewayCACertificate,
		desired,
		current,
		nameMatchedChildOperations[resources.AIGatewayCACertificateResource, state.AIGatewayCACertificate]{
			ownership:   &ownedChildCollectionScope{gatewayRef, gatewayID},
			desiredName: func(child resources.AIGatewayCACertificateResource) string { return child.Name },
			currentName: func(current state.AIGatewayCACertificate) string { return current.Name },
			fetch: func(_ resources.AIGatewayCACertificateResource,
				current state.AIGatewayCACertificate) (*state.AIGatewayCACertificate,
				error,
			) {
				return p.client.GetAIGatewayCACertificate(ctx, gatewayID, current.ID)
			},
			create: create,
			diff: func(current state.AIGatewayCACertificate,
				child resources.AIGatewayCACertificateResource) (bool,
				map[string]any,
				map[string]FieldChange,
				error,
			) {
				fields := child.PayloadMap()
				fields[FieldCert] = strings.TrimSpace(child.Cert)
				observed := map[string]any{
					FieldName:      current.Name,
					FieldCert:      current.Cert,
					FieldLabels:    current.Labels,
					FieldManagedBy: current.ManagedBy,
				}
				if err := prepareOwnedChildDiff(observed, fields, &child, child.Labels); err != nil {
					return false, nil, nil, err
				}
				changed := diffAIGatewayTLSPayloads(observed, fields)
				return len(changed) > 0, fields, changed, nil
			},
			update: func(current state.AIGatewayCACertificate,
				child resources.AIGatewayCACertificateResource,
				fields map[string]any,
				changed map[string]FieldChange,
			) {
				p.planAIGatewayTLSUpdate(
					ResourceTypeAIGatewayCACertificate,
					resources.GetNamespace(child.Kongctl),
					gatewayRef,
					gatewayID,
					current.ID,
					child.Ref, fields, changed, nil, plan,
				)
			},
			remove: func(current state.AIGatewayCACertificate) {
				plan.AddChange(
					newAIGatewayTLSDelete(
						p, ResourceTypeAIGatewayCACertificate,
						current.ManagedBy[resources.ChildNamespaceKey],
						gatewayRef,
						gatewayID,
						current.ID,
						current.Name,
					),
				)
			},
		},
		plan,
	)
}

func (p *Planner) planAIGatewaySNIChanges(
	ctx context.Context,
	_, gatewayRef, gatewayID, gatewayChangeID string,
	desired []resources.AIGatewaySNIResource,
	plan *Plan,
) error {
	payload := func(child resources.AIGatewaySNIResource) (map[string]any, error) {
		fields := child.PayloadMap()
		certificate, err := normalizeAIGatewaySNICertificateReference(
			child.Certificate, p.resources,
		)
		if err != nil {
			return nil, err
		}
		fields[FieldCertificate] = certificate
		return fields, nil
	}
	create := func(child resources.AIGatewaySNIResource) error {
		fields, err := payload(child)
		if err != nil {
			return fmt.Errorf("failed to build AI Gateway SNI %q create payload: %w", child.Ref, err)
		}
		fields[FieldManagedBy] = resources.BuildChildManagedBy(
			nil,
			child.ManagedBy,
			resources.GetNamespace(child.Kongctl),
			child.Protected(),
			"",
		)
		before := len(plan.Changes)
		p.planAIGatewayTLSCreate(
			ResourceTypeAIGatewaySNI,
			resources.GetNamespace(child.Kongctl),
			gatewayRef,
			gatewayChangeID,
			gatewayID,
			child.Ref,
			fields,
			plan,
		)
		plan.Changes[before].Protection = child.Protected()
		return nil
	}
	if gatewayID == "" {
		for _, child := range desired {
			if err := create(child); err != nil {
				return err
			}
		}
		return nil
	}
	current, err := p.listAIGatewaySNIs(ctx, gatewayID)
	if err != nil {
		return err
	}
	return reconcileNameMatchedChildren(
		p,
		ResourceTypeAIGatewaySNI,
		desired,
		current,
		nameMatchedChildOperations[resources.AIGatewaySNIResource, state.AIGatewaySNI]{
			ownership:   &ownedChildCollectionScope{gatewayRef, gatewayID},
			desiredName: func(child resources.AIGatewaySNIResource) string { return child.Name },
			currentName: func(current state.AIGatewaySNI) string { return current.Name },
			fetch: func(_ resources.AIGatewaySNIResource, current state.AIGatewaySNI) (*state.AIGatewaySNI, error) {
				return p.client.GetAIGatewaySNI(ctx, gatewayID, current.ID)
			},
			createWithError: create,
			diff: func(current state.AIGatewaySNI,
				child resources.AIGatewaySNIResource) (bool,
				map[string]any,
				map[string]FieldChange,
				error,
			) {
				fields, err := payload(child)
				if err != nil {
					return false, nil, nil, err
				}
				observed := map[string]any{
					FieldName:        current.Name,
					FieldDisplayName: current.DisplayName,
					FieldHostname:    current.Hostname,
					FieldCertificate: current.Certificate,
					FieldLabels:      current.Labels,
					FieldManagedBy:   current.ManagedBy,
				}
				if err := prepareOwnedChildDiff(observed, fields, &child, child.Labels); err != nil {
					return false, nil, nil, err
				}
				changed := diffAIGatewayTLSPayloads(observed, fields)
				return len(changed) > 0, fields, changed, nil
			},
			update: func(current state.AIGatewaySNI,
				child resources.AIGatewaySNIResource,
				fields map[string]any,
				changed map[string]FieldChange,
			) {
				p.planAIGatewayTLSUpdate(
					ResourceTypeAIGatewaySNI,
					resources.GetNamespace(child.Kongctl),
					gatewayRef,
					gatewayID,
					current.ID,
					child.Ref,
					fields,
					changed,
					nil,
					plan,
				)
			},
			remove: func(current state.AIGatewaySNI) {
				plan.AddChange(
					newAIGatewayTLSDelete(
						p, ResourceTypeAIGatewaySNI,
						current.ManagedBy[resources.ChildNamespaceKey],
						gatewayRef,
						gatewayID,
						current.ID,
						current.Name,
					),
				)
			},
		},
		plan,
	)
}

func (p *Planner) planAIGatewayTLSCreate(
	resourceType, namespace, gatewayRef, gatewayChangeID, gatewayID, resourceRef string,
	fields map[string]any,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = append(dependsOn, gatewayChangeID)
	}
	change := PlannedChange{
		ID: p.nextChangeID(ActionCreate, resourceType, resourceRef), ResourceType: resourceType,
		ResourceRef: resourceRef, Action: ActionCreate, Fields: fields, Namespace: namespace,
		DependsOn: dependsOn,
	}
	if gatewayChangeID == "" {
		change.Parent = &ParentInfo{Ref: gatewayRef, ID: gatewayID}
	}
	if change.Parent == nil || change.Parent.ID == "" {
		change.Parent = nil
		change.References = map[string]ReferenceInfo{FieldAIGatewayID: {
			Ref: gatewayRef, LookupFields: map[string]string{FieldName: gatewayRef},
		}}
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayTLSUpdate(
	resourceType, namespace, gatewayRef, gatewayID, resourceID, resourceRef string,
	fields map[string]any,
	changed map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	plan.AddChange(PlannedChange{
		ID: p.nextChangeID(ActionUpdate, resourceType, resourceRef), ResourceType: resourceType,
		ResourceRef: resourceRef, ResourceID: resourceID, Action: ActionUpdate,
		Fields: fields, ChangedFields: changed, Namespace: namespace, DependsOn: dependsOn,
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	})
}

func newAIGatewayTLSDelete(
	p *Planner,
	resourceType, namespace, gatewayRef, gatewayID, resourceID, name string,
) PlannedChange {
	return PlannedChange{
		ID: p.nextChangeID(ActionDelete, resourceType, name), ResourceType: resourceType,
		ResourceRef: name, ResourceID: resourceID, Action: ActionDelete,
		Fields: map[string]any{FieldName: name}, Namespace: namespace,
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
}

func diffAIGatewayCertificate(
	current state.AIGatewayCertificate,
	desired resources.AIGatewayCertificateResource,
) (map[string]any, map[string]FieldChange) {
	fields := desired.PayloadMap()
	currentFields := map[string]any{FieldName: current.Name, FieldCert: current.Cert}
	if current.CertAlt != nil {
		currentFields[FieldCertAlt] = *current.CertAlt
	}
	addAIGatewayTLSCurrentMetadata(currentFields, current.Labels, current.ManagedBy)
	changed := diffAIGatewayTLSPayloads(currentFields, fields)
	return fields, changed
}

func addAIGatewayTLSCurrentMetadata(fields map[string]any, labels, managedBy map[string]string) {
	if labels != nil {
		fields[FieldLabels] = labels
	}
	if managedBy != nil {
		fields[FieldManagedBy] = managedBy
	}
}

func diffAIGatewayTLSPayloads(current, desired map[string]any) map[string]FieldChange {
	changed := make(map[string]FieldChange)
	for field, desiredValue := range desired {
		if field == FieldManagedBy && managedByValuesEqual(current[field], desiredValue) {
			continue
		}
		if !aigatewayTLSValuesEqual(current[field], desiredValue) {
			changed[field] = FieldChange{Old: current[field], New: desiredValue}
		}
	}
	for field, currentValue := range current {
		if field == FieldManagedBy && managedByValuesEqual(currentValue, desired[field]) {
			continue
		}
		if _, exists := desired[field]; !exists && !emptyAIGatewayTLSMap(currentValue) {
			changed[field] = FieldChange{Old: currentValue, New: nil}
		}
	}
	return changed
}

func aigatewayTLSValuesEqual(current, desired any) bool {
	if emptyAIGatewayTLSMap(current) && (desired == nil || emptyAIGatewayTLSMap(desired)) {
		return true
	}
	if emptyAIGatewayTLSMap(desired) && current == nil {
		return true
	}
	return reflect.DeepEqual(current, desired)
}

func emptyAIGatewayTLSMap(value any) bool {
	field, ok := value.(map[string]string)
	return ok && len(field) == 0
}

func normalizeAIGatewaySNICertificateReference(value string, rs *resources.ResourceSet) (string, error) {
	ref, field, ok := tags.ParseRefPlaceholder(value)
	if !ok {
		return value, nil
	}
	if field != FieldName {
		return "", fmt.Errorf("certificate reference must use #name, got #%s", field)
	}
	if rs != nil {
		for _, certificate := range rs.AIGatewayCertificates {
			if certificate.Ref == ref {
				return certificate.Name, nil
			}
		}
	}
	return value, nil
}

func indexAIGatewayCertificates(
	current []state.AIGatewayCertificate,
) map[string]state.AIGatewayCertificate {
	byName := make(map[string]state.AIGatewayCertificate, len(current))
	for _, certificate := range current {
		byName[certificate.Name] = certificate
	}
	return byName
}

func validateAIGatewayCertificateSecretWrites(plan *Plan) error {
	if plan == nil {
		return nil
	}
	for _, change := range plan.Changes {
		if change.ResourceType != ResourceTypeAIGatewayCertificate ||
			(change.Action != ActionCreate && change.Action != ActionUpdate) {
			continue
		}
		selected := make(map[string]bool, len(change.SecretWrites))
		for _, intent := range change.SecretWrites {
			selected[intent.Field] = true
		}
		if !selected["/key"] {
			return fmt.Errorf(
				"AI Gateway certificate %q requires private key /key for %s; configure !secret and select it for updates",
				change.ResourceRef,
				change.Action,
			)
		}
		if _, hasAlternativeCertificate := change.Fields[FieldCertAlt]; hasAlternativeCertificate &&
			!selected["/key_alt"] {
			return fmt.Errorf(
				"AI Gateway certificate %q requires private key /key_alt when cert_alt is configured",
				change.ResourceRef,
			)
		}
	}
	return nil
}
