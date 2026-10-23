package planner

import (
	"context"
	"fmt"
	"slices"
)

func (p *Planner) resolveAIGatewayCertificateDeletes(ctx context.Context,
	namespace,
	gatewayRef,
	gatewayID string,
	plan *Plan,
) error {
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.ContainsFunc(values, func(value string) bool {
			value = canonicalAIGatewayReference(value, nil)
			return value != "" && (value == name || value == deletion.ResourceID)
		})
	}
	return resolveObservedReferenceDeletes(ctx, namespace, gatewayRef, plan, observedReferenceDeletePolicy{
		gatewayID: gatewayID, targetType: ResourceTypeAIGatewayCertificate,
		usesTarget: func(kind string) bool { return kind == ResourceTypeAIGatewaySNI },
		observe: func(ctx context.Context) ([]observedReferenceUser, error) {
			snis, err := p.listAIGatewaySNIs(ctx, gatewayID)
			if err != nil {
				return nil, err
			}
			users := make([]observedReferenceUser, 0, len(snis))
			for _, sni := range snis {
				users = append(users, observedReferenceUser{ResourceTypeAIGatewaySNI, sni.ID, sni.Name, []string{sni.Certificate}})
			}
			return users, nil
		},
		observedReferences: references,
		plannedReferences: func(deletion, change PlannedChange) bool {
			value, _ := change.Fields[FieldCertificate].(string)
			return references(deletion, []string{value})
		},
		updateDetaches: func(deletion, change PlannedChange) bool {
			value, valid := change.Fields[FieldCertificate].(string)
			return valid && value != "" && !references(deletion, []string{value})
		},
		conflict: func(deletion PlannedChange, user observedReferenceUser, planned bool) error {
			qualifier := ""
			if planned {
				qualifier = "planned "
			}
			return fmt.Errorf("cannot delete AI Gateway certificate %q while %sSNI %q still references it",
				deletion.Fields[FieldName],
				qualifier,
				user.name)
		},
	})
}
