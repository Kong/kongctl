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
)

func (p *Planner) planDeckDependencies(ctx context.Context, rs *resources.ResourceSet, plan *Plan, opts Options) error {
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

		cpCreateID := findChangeIDByRef(plan.Changes, "control_plane", cpRef, ActionCreate)

		if cpName == "" && cpID != "" {
			resolved, err := p.resolveDeckControlPlaneName(ctx, cpID)
			if err != nil {
				return err
			}
			cpName = resolved
		}

		requires := svc.External.Requires.Deck
		deckFiles := cloneStringSlice(requires.Files)
		deckFlags := cloneStringSlice(requires.Flags)

		deckBaseDir := strings.TrimSpace(svc.DeckBaseDir())
		if cpCreateID == "" && cpID != "" {
			changes, err := p.deckDiffHasChanges(ctx, svc.GetRef(), cpName, deckBaseDir, deckFiles, deckFlags, opts)
			if err != nil {
				return err
			}
			if !changes {
				p.logger.Debug("Deck diff reported no changes; skipping deck plan entry",
					slog.String("gateway_service_ref", svc.GetRef()),
				)
				continue
			}
		} else {
			p.logger.Debug("Skipping deck diff; control plane not yet available",
				slog.String("gateway_service_ref", svc.GetRef()),
				slog.String("control_plane_ref", cpRef),
			)
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
				"deck_base_dir":       deckBaseDir,
				"selector": map[string]any{
					"matchFields": map[string]string{
						"name": selectorName,
					},
				},
				"files": deckFiles,
				"flags": deckFlags,
			},
			Namespace: resources.NamespaceExternal,
		}

		if cpCreateID != "" {
			change.DependsOn = appendDependsOn(change.DependsOn, cpCreateID)
		}

		plan.AddChange(change)
		deckChangeIDs[svc.GetRef()] = change.ID

		p.logger.Debug("Planned deck requirements",
			slog.String("gateway_service_ref", svc.GetRef()),
			slog.Int("files", len(deckFiles)),
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
	gatewayRef string,
	controlPlaneName string,
	deckBaseDir string,
	files []string,
	flags []string,
	opts Options,
) (bool, error) {
	mode := opts.Mode
	switch mode {
	case PlanModeApply, PlanModeSync:
	default:
		return false, fmt.Errorf("gateway_service %s: deck diff requires apply or sync mode", gatewayRef)
	}

	if len(files) == 0 {
		return false, fmt.Errorf("gateway_service %s: deck requires at least one state file", gatewayRef)
	}
	if strings.TrimSpace(controlPlaneName) == "" {
		return false, fmt.Errorf("gateway_service %s: control plane name is required for deck diff", gatewayRef)
	}

	token := strings.TrimSpace(opts.Deck.KonnectToken)
	if token == "" {
		return false, fmt.Errorf("gateway_service %s: Konnect token is required for deck diff", gatewayRef)
	}

	address := strings.TrimSpace(opts.Deck.KonnectAddress)
	if address == "" {
		return false, fmt.Errorf("gateway_service %s: Konnect address is required for deck diff", gatewayRef)
	}

	runner := opts.Deck.Runner
	if runner == nil {
		runner = deck.NewRunner()
	}

	args := []string{"gateway", "diff", "--json-output", "--no-color"}
	args = append(args, flags...)
	args = append(args, files...)

	result, err := runner.Run(ctx, deck.RunOptions{
		Args:                    args,
		Mode:                    string(mode),
		KonnectToken:            token,
		KonnectControlPlaneName: controlPlaneName,
		KonnectAddress:          address,
		WorkDir:                 deckBaseDir,
	})
	if err != nil {
		return false, deckDiffRunError(gatewayRef, result, err)
	}
	if result == nil {
		return false, fmt.Errorf("deck diff for gateway_service %s returned no output", gatewayRef)
	}

	stdout := strings.TrimSpace(result.Stdout)
	if stdout == "" {
		return false, fmt.Errorf("deck diff for gateway_service %s returned empty output", gatewayRef)
	}

	var diff deckDiffOutput
	if err := json.Unmarshal([]byte(stdout), &diff); err != nil {
		return false, fmt.Errorf("deck diff for gateway_service %s returned invalid JSON: %w", gatewayRef, err)
	}

	if len(diff.Errors) > 0 {
		return false, fmt.Errorf(
			"deck diff for gateway_service %s reported errors: %s",
			gatewayRef,
			formatDeckDiffErrors(diff.Errors),
		)
	}

	changes := diff.Summary.Creating + diff.Summary.Updating + diff.Summary.Deleting
	if mode == PlanModeApply {
		changes = diff.Summary.Creating + diff.Summary.Updating
	}

	return changes > 0, nil
}

func (p *Planner) resolveDeckControlPlaneName(ctx context.Context, cpID string) (string, error) {
	if strings.TrimSpace(cpID) == "" {
		return "", fmt.Errorf("deck diff requires a control plane ID to resolve name")
	}
	if p.client == nil {
		return "", fmt.Errorf("state client is required to resolve control plane name")
	}

	cp, err := p.client.GetControlPlaneByID(ctx, cpID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve control plane name: %w", err)
	}
	if cp == nil || strings.TrimSpace(cp.Name) == "" {
		return "", fmt.Errorf("control plane %s not found for deck diff", cpID)
	}

	return cp.Name, nil
}

func deckDiffRunError(gatewayRef string, result *deck.RunResult, runErr error) error {
	if result == nil {
		return fmt.Errorf("deck diff for gateway_service %s failed: %w", gatewayRef, runErr)
	}

	stderr := strings.TrimSpace(result.Stderr)
	if stderr == "" {
		return fmt.Errorf("deck diff for gateway_service %s failed: %w", gatewayRef, runErr)
	}

	return fmt.Errorf(
		"deck diff for gateway_service %s failed: %w: deck stderr: %s",
		gatewayRef,
		runErr,
		stderr,
	)
}

func formatDeckDiffErrors(errors []any) string {
	data, err := json.Marshal(errors)
	if err != nil {
		return "unknown errors"
	}
	return string(data)
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}
