package planner

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

type pendingAIGatewayCertificateDelete struct {
	certificate state.AIGatewayCertificate
	change      PlannedChange
}

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

	var pendingDeletes []pendingAIGatewayCertificateDelete
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

	var currentSNIs []state.AIGatewaySNI
	if sniScoped && (len(snis) > 0 || plan.Metadata.Mode == PlanModeSync) {
		var err error
		currentSNIs, err = p.planAIGatewaySNIChanges(
			ctx, namespace, gatewayRef, gatewayID, gatewayChangeID, snis, plan,
		)
		if err != nil {
			return err
		}
	} else if len(pendingDeletes) > 0 && gatewayID != "" {
		var err error
		currentSNIs, err = p.client.ListAIGatewaySNIs(ctx, gatewayID)
		if err != nil {
			return fmt.Errorf("failed to inspect AI Gateway SNIs before deleting certificates: %w", err)
		}
	}
	return p.planAIGatewayCertificateDeletes(pendingDeletes, currentSNIs, plan)
}

func (p *Planner) planAIGatewayCertificateUpserts(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, gatewayChangeID string,
	desired []resources.AIGatewayCertificateResource,
	plan *Plan,
) ([]pendingAIGatewayCertificateDelete, error) {
	p.logger.Debug("Planning AI Gateway certificate changes",
		slog.String("gateway_ref", gatewayRef), slog.Int("desired_count", len(desired)))
	if gatewayID == "" {
		for _, certificate := range desired {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewayCertificate, namespace, gatewayRef, gatewayChangeID,
				"", certificate.Ref, certificate.PayloadMap(), nil, plan,
			)
		}
		return nil, nil
	}
	current, err := p.client.ListAIGatewayCertificates(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to list AI Gateway certificates: %w", err)
	}
	byID, byName := indexAIGatewayCertificates(current)
	desiredKeys := make(map[string]bool, len(desired)*2)
	for i := range desired {
		certificate := &desired[i]
		matched, exists := matchAIGatewayCertificate(*certificate, byID, byName)
		desiredKeys[certificate.Name] = true
		if id := certificate.GetKonnectID(); id != "" {
			desiredKeys[id] = true
		}
		if !exists {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewayCertificate, namespace, gatewayRef, "",
				gatewayID, certificate.Ref, certificate.PayloadMap(), nil, plan,
			)
			continue
		}
		if certificate.GetKonnectID() != "" && matched.Name != certificate.Name {
			return nil, immutableAIGatewayTLSNameError(
				"certificate", certificate.Ref, matched.ID, matched.Name, certificate.Name,
			)
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

	var deletes []pendingAIGatewayCertificateDelete
	if plan.Metadata.Mode == PlanModeSync {
		for _, certificate := range current {
			if desiredKeys[certificate.ID] || desiredKeys[certificate.Name] {
				continue
			}
			deletes = append(deletes, pendingAIGatewayCertificateDelete{
				certificate: certificate,
				change: newAIGatewayTLSDelete(
					p, ResourceTypeAIGatewayCertificate, namespace, gatewayRef, gatewayID,
					certificate.ID, certificate.Name,
				),
			})
		}
	}
	return deletes, nil
}

func (p *Planner) planAIGatewayCACertificateChanges(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, gatewayChangeID string,
	desired []resources.AIGatewayCACertificateResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning AI Gateway CA certificate changes",
		slog.String("gateway_ref", gatewayRef), slog.Int("desired_count", len(desired)))
	if gatewayID == "" {
		for _, certificate := range desired {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewayCACertificate, namespace, gatewayRef, gatewayChangeID,
				"", certificate.Ref, certificate.PayloadMap(), nil, plan,
			)
		}
		return nil
	}
	current, err := p.client.ListAIGatewayCACertificates(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway CA certificates: %w", err)
	}
	byID, byName := indexAIGatewayCACertificates(current)
	desiredKeys := make(map[string]bool, len(desired)*2)
	for i := range desired {
		certificate := &desired[i]
		matched, exists := matchAIGatewayCACertificate(*certificate, byID, byName)
		desiredKeys[certificate.Name] = true
		if id := certificate.GetKonnectID(); id != "" {
			desiredKeys[id] = true
		}
		if !exists {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewayCACertificate, namespace, gatewayRef, "",
				gatewayID, certificate.Ref, certificate.PayloadMap(), nil, plan,
			)
			continue
		}
		if certificate.GetKonnectID() != "" && matched.Name != certificate.Name {
			return immutableAIGatewayTLSNameError("CA certificate", certificate.Ref, matched.ID, matched.Name, certificate.Name)
		}
		certificate.SetKonnectID(matched.ID)
		fields, changed := diffAIGatewayCACertificate(matched, *certificate)
		if len(changed) > 0 {
			p.planAIGatewayTLSUpdate(
				ResourceTypeAIGatewayCACertificate, namespace, gatewayRef, gatewayID,
				matched.ID, certificate.Ref, fields, changed, nil, plan,
			)
		}
	}
	if plan.Metadata.Mode == PlanModeSync {
		for _, certificate := range current {
			if !desiredKeys[certificate.ID] && !desiredKeys[certificate.Name] {
				plan.AddChange(newAIGatewayTLSDelete(
					p, ResourceTypeAIGatewayCACertificate, namespace, gatewayRef, gatewayID,
					certificate.ID, certificate.Name,
				))
			}
		}
	}
	return nil
}

func (p *Planner) planAIGatewaySNIChanges(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, gatewayChangeID string,
	desired []resources.AIGatewaySNIResource,
	plan *Plan,
) ([]state.AIGatewaySNI, error) {
	p.logger.Debug("Planning AI Gateway SNI changes",
		slog.String("gateway_ref", gatewayRef), slog.Int("desired_count", len(desired)))
	if gatewayID == "" {
		for _, sni := range desired {
			fields := sni.PayloadMap()
			certificate, err := normalizeAIGatewaySNICertificateReference(sni.Certificate, p.resources)
			if err != nil {
				return nil, fmt.Errorf("AI Gateway SNI %q: %w", sni.Ref, err)
			}
			fields[FieldCertificate] = certificate
			dependsOn := certificateChangeDependency(plan, namespace, sni.Certificate)
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewaySNI, namespace, gatewayRef, gatewayChangeID,
				"", sni.Ref, fields, dependsOn, plan,
			)
		}
		return nil, nil
	}
	current, err := p.client.ListAIGatewaySNIs(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to list AI Gateway SNIs: %w", err)
	}
	byID, byName := indexAIGatewaySNIs(current)
	desiredKeys := make(map[string]bool, len(desired)*2)
	for i := range desired {
		sni := &desired[i]
		matched, exists := matchAIGatewaySNI(*sni, byID, byName)
		desiredKeys[sni.Name] = true
		if id := sni.GetKonnectID(); id != "" {
			desiredKeys[id] = true
		}
		fields := sni.PayloadMap()
		certificate, err := normalizeAIGatewaySNICertificateReference(sni.Certificate, p.resources)
		if err != nil {
			return nil, fmt.Errorf("AI Gateway SNI %q: %w", sni.Ref, err)
		}
		fields[FieldCertificate] = certificate
		dependsOn := certificateChangeDependency(plan, namespace, sni.Certificate)
		if !exists {
			p.planAIGatewayTLSCreate(
				ResourceTypeAIGatewaySNI, namespace, gatewayRef, "", gatewayID, sni.Ref, fields, dependsOn, plan,
			)
			continue
		}
		if sni.GetKonnectID() != "" && matched.Name != sni.Name {
			return nil, immutableAIGatewayTLSNameError("SNI", sni.Ref, matched.ID, matched.Name, sni.Name)
		}
		sni.SetKonnectID(matched.ID)
		updateFields, changed := diffAIGatewaySNI(matched, fields)
		if len(changed) > 0 {
			p.planAIGatewayTLSUpdate(
				ResourceTypeAIGatewaySNI, namespace, gatewayRef, gatewayID,
				matched.ID, sni.Ref, updateFields, changed, dependsOn, plan,
			)
		}
	}
	if plan.Metadata.Mode == PlanModeSync {
		for _, sni := range current {
			if !desiredKeys[sni.ID] && !desiredKeys[sni.Name] {
				plan.AddChange(newAIGatewayTLSDelete(
					p, ResourceTypeAIGatewaySNI, namespace, gatewayRef, gatewayID, sni.ID, sni.Name,
				))
			}
		}
	}
	return current, nil
}

func (p *Planner) planAIGatewayCertificateDeletes(
	deletes []pendingAIGatewayCertificateDelete,
	currentSNIs []state.AIGatewaySNI,
	plan *Plan,
) error {
	for _, pending := range deletes {
		for _, sni := range currentSNIs {
			if sni.Certificate != pending.certificate.Name {
				continue
			}
			change := findAIGatewayTLSChange(plan, ResourceTypeAIGatewaySNI, sni.ID)
			if change == nil || (change.Action == ActionUpdate &&
				change.Fields[FieldCertificate] == pending.certificate.Name) {
				return fmt.Errorf(
					"cannot delete AI Gateway certificate %q while SNI %q still references it",
					pending.certificate.Name, sni.Name,
				)
			}
			pending.change.DependsOn = append(pending.change.DependsOn, change.ID)
		}
		slices.Sort(pending.change.DependsOn)
		plan.AddChange(pending.change)
	}
	return nil
}

func (p *Planner) planAIGatewayTLSCreate(
	resourceType, namespace, gatewayRef, gatewayChangeID, gatewayID, resourceRef string,
	fields map[string]any,
	dependsOn []string,
	plan *Plan,
) {
	if gatewayChangeID != "" {
		dependsOn = append(slices.Clone(dependsOn), gatewayChangeID)
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
	changed := diffAIGatewayTLSPayloads(
		map[string]any{
			FieldName: current.Name, FieldCert: current.Cert, FieldCertAlt: current.CertAlt,
			FieldLabels: current.Labels, FieldManagedBy: current.ManagedBy,
		},
		fields,
	)
	return fields, changed
}

func diffAIGatewayCACertificate(
	current state.AIGatewayCACertificate,
	desired resources.AIGatewayCACertificateResource,
) (map[string]any, map[string]FieldChange) {
	fields := desired.PayloadMap()
	return fields, diffAIGatewayTLSPayloads(map[string]any{
		FieldName: current.Name, FieldCert: current.Cert,
		FieldLabels: current.Labels, FieldManagedBy: current.ManagedBy,
	}, fields)
}

func diffAIGatewaySNI(
	current state.AIGatewaySNI,
	fields map[string]any,
) (map[string]any, map[string]FieldChange) {
	return fields, diffAIGatewayTLSPayloads(map[string]any{
		FieldName: current.Name, FieldDisplayName: current.DisplayName, FieldHostname: current.Hostname,
		FieldCertificate: current.Certificate, FieldLabels: current.Labels, FieldManagedBy: current.ManagedBy,
	}, fields)
}

func diffAIGatewayTLSPayloads(current, desired map[string]any) map[string]FieldChange {
	changed := make(map[string]FieldChange)
	for field, desiredValue := range desired {
		if !reflect.DeepEqual(current[field], desiredValue) {
			changed[field] = FieldChange{Old: current[field], New: desiredValue}
		}
	}
	return changed
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

func certificateChangeDependency(plan *Plan, namespace, reference string) []string {
	targetRef := resources.NormalizeResourceRef(reference)
	if parsed, _, ok := tags.ParseRefPlaceholder(reference); ok {
		targetRef = parsed
	}
	for _, change := range plan.Changes {
		if change.Namespace == namespace && change.ResourceType == ResourceTypeAIGatewayCertificate &&
			change.ResourceRef == targetRef && change.Action != ActionDelete {
			return []string{change.ID}
		}
	}
	return nil
}

func findAIGatewayTLSChange(plan *Plan, resourceType, resourceID string) *PlannedChange {
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType == resourceType && change.ResourceID == resourceID {
			return change
		}
	}
	return nil
}

func indexAIGatewayCertificates(
	current []state.AIGatewayCertificate,
) (map[string]state.AIGatewayCertificate, map[string]state.AIGatewayCertificate) {
	byID := make(map[string]state.AIGatewayCertificate, len(current))
	byName := make(map[string]state.AIGatewayCertificate, len(current))
	for _, certificate := range current {
		byID[certificate.ID], byName[certificate.Name] = certificate, certificate
	}
	return byID, byName
}

func indexAIGatewayCACertificates(
	current []state.AIGatewayCACertificate,
) (map[string]state.AIGatewayCACertificate, map[string]state.AIGatewayCACertificate) {
	byID := make(map[string]state.AIGatewayCACertificate, len(current))
	byName := make(map[string]state.AIGatewayCACertificate, len(current))
	for _, certificate := range current {
		byID[certificate.ID], byName[certificate.Name] = certificate, certificate
	}
	return byID, byName
}

func indexAIGatewaySNIs(current []state.AIGatewaySNI) (map[string]state.AIGatewaySNI, map[string]state.AIGatewaySNI) {
	byID := make(map[string]state.AIGatewaySNI, len(current))
	byName := make(map[string]state.AIGatewaySNI, len(current))
	for _, sni := range current {
		byID[sni.ID], byName[sni.Name] = sni, sni
	}
	return byID, byName
}

func matchAIGatewayCertificate(
	desired resources.AIGatewayCertificateResource,
	byID, byName map[string]state.AIGatewayCertificate,
) (state.AIGatewayCertificate, bool) {
	if id := desired.GetKonnectID(); id != "" {
		current, ok := byID[id]
		return current, ok
	}
	current, ok := byName[desired.Name]
	return current, ok
}

func matchAIGatewayCACertificate(
	desired resources.AIGatewayCACertificateResource,
	byID, byName map[string]state.AIGatewayCACertificate,
) (state.AIGatewayCACertificate, bool) {
	if id := desired.GetKonnectID(); id != "" {
		current, ok := byID[id]
		return current, ok
	}
	current, ok := byName[desired.Name]
	return current, ok
}

func matchAIGatewaySNI(
	desired resources.AIGatewaySNIResource,
	byID, byName map[string]state.AIGatewaySNI,
) (state.AIGatewaySNI, bool) {
	if id := desired.GetKonnectID(); id != "" {
		current, ok := byID[id]
		return current, ok
	}
	current, ok := byName[desired.Name]
	return current, ok
}

func immutableAIGatewayTLSNameError(kind, ref, id, currentName, desiredName string) error {
	return fmt.Errorf(
		"AI Gateway %s %q is matched by ID %s but its immutable name is %q; delete and recreate it to use name %q",
		kind, ref, id, currentName, desiredName,
	)
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
				change.ResourceRef, change.Action,
			)
		}
		if _, hasAlternativeCertificate := change.Fields[FieldCertAlt]; hasAlternativeCertificate && !selected["/key_alt"] {
			return fmt.Errorf(
				"AI Gateway certificate %q requires private key /key_alt when cert_alt is configured",
				change.ResourceRef,
			)
		}
	}
	return nil
}
