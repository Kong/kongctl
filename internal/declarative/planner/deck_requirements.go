package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/declarative/deck"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

type deckGatewayService struct {
	Ref      string
	Selector *resources.ExternalSelector
}

func (p *Planner) planDeckDependencies(ctx context.Context, rs *resources.ResourceSet, plan *Plan, opts Options) error {
	if rs == nil || plan == nil {
		return nil
	}

	// In delete mode, skip deck dependency planning entirely.
	// Deleting the control plane cascades removal of all core entities
	// that deck would otherwise manage, so running deck diff is unnecessary.
	if opts.Mode == PlanModeDelete {
		return nil
	}

	deckChangeIDs := make(map[string]string)
	serviceToDeckChange := make(map[string]string)
	neededGatewayServices := referencedGatewayServiceRefs(plan.Changes)
	deckCount := 0

	for i := range rs.ControlPlanes {
		cp := &rs.ControlPlanes[i]
		if cp.Deck == nil {
			continue
		}
		deckCount++

		cpRef := cp.GetRef()
		cpID := cp.GetKonnectID()
		cpName := cp.Name
		if cpName == "" && cp.External != nil && cp.External.Selector != nil {
			cpName = cp.External.Selector.MatchFields["name"]
		}

		cpCreateID := findChangeIDByRef(plan.Changes, "control_plane", cpRef, ActionCreate)

		if cpName == "" && cpID != "" {
			resolved, err := p.resolveDeckControlPlaneName(ctx, cpID)
			if err != nil {
				return err
			}
			cpName = resolved
		}

		deckFiles := cloneStringSlice(cp.Deck.Files)
		deckFlags := cloneStringSlice(cp.Deck.Flags)
		deckBaseDir := strings.TrimSpace(cp.DeckBaseDir())

		if cpCreateID == "" && cpID != "" {
			changes, err := p.deckDiffHasChanges(ctx, cpRef, cpName, deckBaseDir, deckFiles, deckFlags, opts)
			if err != nil {
				return err
			}
			if !changes {
				p.logger.Debug("Deck diff reported no changes; skipping deck plan entry",
					slog.String("control_plane_ref", cpRef),
				)
				continue
			}
		} else {
			p.logger.Debug("Skipping deck diff; control plane not yet available",
				slog.String("control_plane_ref", cpRef),
			)
		}

		gatewayServices, err := collectDeckGatewayServices(rs.GatewayServices, cpRef, neededGatewayServices)
		if err != nil {
			return err
		}

		postResolutionTargets := deckPostResolutionTargets(gatewayServices, cpRef, cpID, cpName)

		change := PlannedChange{
			ID:           p.nextChangeID(ActionExternalTool, ResourceTypeDeck, cpRef),
			ResourceType: ResourceTypeDeck,
			ResourceRef:  cpRef,
			Action:       ActionExternalTool,
			Fields: map[string]any{
				"control_plane_ref":  cpRef,
				"control_plane_id":   cpID,
				"control_plane_name": cpName,
				"deck_base_dir":      deckBaseDir,
				"files":              deckFiles,
				"flags":              deckFlags,
			},
			PostResolutionTargets: postResolutionTargets,
			Namespace:             resources.NamespaceExternal,
		}

		if cpCreateID != "" {
			change.DependsOn = appendDependsOn(change.DependsOn, cpCreateID)
		}

		plan.AddChange(change)
		deckChangeIDs[cpRef] = change.ID

		for _, svc := range postResolutionTargets {
			if svc.ResourceRef == "" {
				continue
			}
			serviceToDeckChange[svc.ResourceRef] = change.ID
		}

		p.logger.Debug("Planned deck config",
			slog.String("control_plane_ref", cpRef),
			slog.Int("files", len(deckFiles)),
		)
	}

	if len(deckChangeIDs) == 0 {
		return nil
	}

	p.logger.Debug("Linking deck dependencies to api_implementation changes",
		slog.Int("deck_configs", deckCount),
	)

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != "api_implementation" ||
			(change.Action != ActionCreate && change.Action != ActionUpdate) {
			continue
		}
		ref := deckServiceRefFromFields(change.Fields, serviceToDeckChange)
		if ref == "" {
			continue
		}
		change.DependsOn = appendDependsOn(change.DependsOn, serviceToDeckChange[ref])

		p.logger.Debug("Added deck dependency to api_implementation",
			slog.String("api_implementation_ref", change.ResourceRef),
			slog.String("gateway_service_ref", ref),
		)
	}

	return nil
}

func collectDeckGatewayServices(
	services []resources.GatewayServiceResource,
	controlPlaneRef string,
	needed map[string]bool,
) ([]deckGatewayService, error) {
	if controlPlaneRef == "" || len(services) == 0 || len(needed) == 0 {
		return nil, nil
	}

	var selected []deckGatewayService
	for i := range services {
		svc := &services[i]
		cpRef := normalizeControlPlaneRef(svc.ControlPlane)
		if cpRef != controlPlaneRef {
			continue
		}
		if !needed[svc.GetRef()] {
			continue
		}
		if svc.External == nil || svc.External.Selector == nil {
			continue
		}
		selectorName := svc.External.Selector.MatchFields["name"]
		if err := ensureDeckSelectorName(selectorName, svc.GetRef()); err != nil {
			return nil, err
		}
		selected = append(selected, deckGatewayService{Ref: svc.GetRef(), Selector: svc.External.Selector})
	}

	return selected, nil
}

func deckPostResolutionTargets(
	services []deckGatewayService,
	controlPlaneRef string,
	controlPlaneID string,
	controlPlaneName string,
) []PostResolutionTarget {
	if len(services) == 0 {
		return nil
	}

	result := make([]PostResolutionTarget, 0, len(services))
	for _, svc := range services {
		target := PostResolutionTarget{
			ResourceType:     "gateway_service",
			ResourceRef:      svc.Ref,
			ControlPlaneRef:  controlPlaneRef,
			ControlPlaneID:   controlPlaneID,
			ControlPlaneName: controlPlaneName,
		}
		if svc.Selector != nil {
			target.Selector = &ExternalToolSelector{MatchFields: svc.Selector.MatchFields}
		}
		result = append(result, target)
	}

	return result
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

func referencedGatewayServiceRefs(changes []PlannedChange) map[string]bool {
	if len(changes) == 0 {
		return nil
	}

	refs := make(map[string]bool)
	for _, change := range changes {
		if change.ResourceType != "api_implementation" {
			continue
		}
		if change.Action != ActionCreate && change.Action != ActionUpdate {
			continue
		}
		serviceValue, ok := change.Fields["service"]
		if !ok {
			continue
		}
		serviceMap, ok := serviceValue.(map[string]any)
		if !ok {
			continue
		}
		ref := gatewayServiceRefFromServiceID(serviceMap["id"])
		if ref != "" {
			refs[ref] = true
		}
	}

	if len(refs) == 0 {
		return nil
	}
	return refs
}

func gatewayServiceRefFromServiceID(value any) string {
	id, ok := value.(string)
	if !ok || strings.TrimSpace(id) == "" {
		return ""
	}
	if tags.IsRefPlaceholder(id) {
		ref, field, ok := tags.ParseRefPlaceholder(id)
		if ok && field == "id" {
			return ref
		}
		return ""
	}
	if util.IsValidUUID(id) {
		return ""
	}
	return id
}

type deckDiffOutput struct {
	Summary deckDiffSummary `json:"summary"`
	Errors  []any           `json:"errors"`
}

type deckDiffSummary struct {
	Creating int `json:"creating"`
	Updating int `json:"updating"`
	Deleting int `json:"deleting"`
	Total    int `json:"total"`
}

func (p *Planner) deckDiffHasChanges(
	ctx context.Context,
	controlPlaneRef string,
	controlPlaneName string,
	deckBaseDir string,
	files []string,
	flags []string,
	opts Options,
) (bool, error) {
	mode := opts.Mode
	switch mode {
	case PlanModeApply, PlanModeSync:
		// valid modes for deck operations
	case PlanModeDelete:
		// Delete mode skips deck operations; the control plane deletion
		// cascades removal of all core entities deck would manage.
		return false, nil
	default:
		return false, fmt.Errorf("control_plane %s: deck diff requires apply or sync mode", controlPlaneRef)
	}

	if len(files) == 0 {
		return false, fmt.Errorf("control_plane %s: _deck requires at least one state file", controlPlaneRef)
	}

	runner := opts.Deck.Runner
	if runner == nil {
		runner = deck.NewRunner()
	}
	if strings.TrimSpace(controlPlaneName) == "" {
		return false, fmt.Errorf("control_plane %s: deck requires a control plane name", controlPlaneRef)
	}

	args := append([]string{"gateway", "diff"}, flags...)
	args = append(args, "--json-output", "--no-color")
	args = append(args, files...)

	result, err := runner.Run(ctx, deck.RunOptions{
		Args:                    args,
		Mode:                    string(mode),
		KonnectToken:            opts.Deck.KonnectToken,
		KonnectControlPlaneName: controlPlaneName,
		KonnectAddress:          opts.Deck.KonnectAddress,
		WorkDir:                 deckBaseDir,
	})
	if err != nil {
		return false, fmt.Errorf("control_plane %s: deck diff failed: %w%s",
			controlPlaneRef,
			err,
			deckRunErrorSuffix(result),
		)
	}

	if result == nil || strings.TrimSpace(result.Stdout) == "" {
		return false, fmt.Errorf("control_plane %s: deck diff returned no output", controlPlaneRef)
	}

	var output deckDiffOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return false, fmt.Errorf("control_plane %s: decode deck diff output: %w", controlPlaneRef, err)
	}

	if len(output.Errors) > 0 {
		return false, fmt.Errorf("control_plane %s: deck diff reported errors: %s",
			controlPlaneRef,
			deckErrorsSummary(output.Errors),
		)
	}

	if mode == PlanModeApply {
		return (output.Summary.Creating + output.Summary.Updating) > 0, nil
	}

	return output.Summary.Total > 0, nil
}

func (p *Planner) resolveDeckControlPlaneName(ctx context.Context, controlPlaneID string) (string, error) {
	if strings.TrimSpace(controlPlaneID) == "" {
		return "", fmt.Errorf("control plane ID is required to resolve name")
	}
	if p.client == nil {
		return "", fmt.Errorf("state client is required to resolve control plane name")
	}

	cp, err := p.client.GetControlPlaneByID(ctx, controlPlaneID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve control plane name: %w", err)
	}
	if cp == nil || strings.TrimSpace(cp.Name) == "" {
		return "", fmt.Errorf("control_plane %s not found for deck planning", controlPlaneID)
	}

	return cp.Name, nil
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func deckRunErrorSuffix(result *deck.RunResult) string {
	if result == nil {
		return ""
	}
	stderr := strings.TrimSpace(result.Stderr)
	stdout := strings.TrimSpace(result.Stdout)
	detail := ""
	if stderr != "" {
		detail = stderr
	} else if stdout != "" {
		detail = stdout
	}
	if detail == "" {
		return ""
	}
	return fmt.Sprintf(": %s", truncateDeckOutput(detail, 2048))
}

func deckErrorsSummary(errors []any) string {
	if len(errors) == 0 {
		return ""
	}
	first := deckErrorDetail(errors[0])
	if len(errors) == 1 {
		return first
	}
	return fmt.Sprintf("%s (and %d more)", first, len(errors)-1)
}

func deckErrorDetail(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		if msg, ok := v["message"].(string); ok && msg != "" {
			return msg
		}
		if msg, ok := v["error"].(string); ok && msg != "" {
			return msg
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}

func truncateDeckOutput(value string, maxLen int) string {
	if maxLen <= 0 {
		return value
	}
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "...(truncated)"
}
