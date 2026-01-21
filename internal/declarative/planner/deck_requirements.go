package planner

import (
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (p *Planner) planDeckDependencies(rs *resources.ResourceSet, plan *Plan) error {
	if rs == nil || plan == nil {
		return nil
	}

	deckChangeIDs := make(map[string]string)
	deckCount := 0

	for i := range rs.GatewayServices {
		svc := rs.GatewayServices[i]
		if !svc.HasDeckRequires() {
			continue
		}
		deckCount++

		selectorName := ""
		if svc.External != nil && svc.External.Selector != nil {
			selectorName = svc.External.Selector.MatchFields["name"]
		}
		if err := ensureDeckSelectorName(selectorName, svc.GetRef()); err != nil {
			return err
		}

		cpRef := normalizeControlPlaneRef(svc.ControlPlane)
		cpID := svc.ResolvedControlPlaneID()
		cpName := ""
		if cp := rs.GetControlPlaneByRef(cpRef); cp != nil {
			cpName = cp.Name
			if cpID == "" {
				cpID = cp.GetKonnectID()
			}
			if cpName == "" && cp.External != nil && cp.External.Selector != nil {
				cpName = cp.External.Selector.MatchFields["name"]
			}
		}

		steps := make([]DeckDependencyStep, 0, len(svc.External.Requires.Deck))
		for _, step := range svc.External.Requires.Deck {
			steps = append(steps, DeckDependencyStep{Args: append([]string{}, step.Args...)})
		}

		change := PlannedChange{
			ID:           p.nextChangeID(ActionExternalTool, ResourceTypeDeck, svc.GetRef()),
			ResourceType: ResourceTypeDeck,
			ResourceRef:  svc.GetRef(),
			Action:       ActionExternalTool,
			Fields: map[string]any{
				"gateway_service_ref": svc.GetRef(),
				"control_plane_ref":   cpRef,
				"control_plane_id":    cpID,
				"control_plane_name":  cpName,
				"selector": map[string]any{
					"matchFields": map[string]string{
						"name": selectorName,
					},
				},
				"steps": steps,
			},
			Namespace: resources.NamespaceExternal,
		}

		if cpCreateID := findChangeIDByRef(plan.Changes, "control_plane", cpRef, ActionCreate); cpCreateID != "" {
			change.DependsOn = appendDependsOn(change.DependsOn, cpCreateID)
		}

		plan.AddChange(change)
		deckChangeIDs[svc.GetRef()] = change.ID

		p.logger.Debug("Planned deck requirements",
			slog.String("gateway_service_ref", svc.GetRef()),
			slog.Int("steps", len(steps)),
		)
	}

	if len(deckChangeIDs) == 0 {
		return nil
	}

	p.logger.Debug("Linking deck dependencies to api_implementation changes",
		slog.Int("deck_requirements", deckCount),
	)

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != "api_implementation" || change.Action != ActionCreate {
			continue
		}
		ref := deckServiceRefFromFields(change.Fields, deckChangeIDs)
		if ref == "" {
			continue
		}
		change.DependsOn = appendDependsOn(change.DependsOn, deckChangeIDs[ref])

		p.logger.Debug("Added deck dependency to api_implementation",
			slog.String("api_implementation_ref", change.ResourceRef),
			slog.String("gateway_service_ref", ref),
		)
	}

	return nil
}

func normalizeControlPlaneRef(raw string) string {
	if tags.IsRefPlaceholder(raw) {
		ref, field, ok := tags.ParseRefPlaceholder(raw)
		if ok && field == "id" {
			return ref
		}
	}
	return raw
}

func findChangeIDByRef(changes []PlannedChange, resourceType, resourceRef string, action ActionType) string {
	for _, change := range changes {
		if change.ResourceType == resourceType && change.ResourceRef == resourceRef && change.Action == action {
			return change.ID
		}
	}
	return ""
}

func deckServiceRefFromFields(fields map[string]any, deckChangeIDs map[string]string) string {
	if len(fields) == 0 {
		return ""
	}

	svcValue, ok := fields["service"]
	if !ok {
		return ""
	}

	svcMap, ok := svcValue.(map[string]any)
	if !ok {
		return ""
	}

	idValue, ok := svcMap["id"].(string)
	if !ok || idValue == "" {
		return ""
	}

	if tags.IsRefPlaceholder(idValue) {
		ref, field, ok := tags.ParseRefPlaceholder(idValue)
		if ok && field == "id" {
			if _, exists := deckChangeIDs[ref]; exists {
				return ref
			}
		}
		return ""
	}

	if _, exists := deckChangeIDs[idValue]; exists {
		return idValue
	}

	return ""
}

func ensureDeckSelectorName(name string, ref string) error {
	if name == "" {
		return fmt.Errorf("gateway_service %s: _external selector.matchFields.name is required", ref)
	}
	return nil
}
