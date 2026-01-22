package executor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/internal/declarative/deck"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util"
)

func (e *Executor) executeDeckStep(ctx context.Context, change *planner.PlannedChange, plan *planner.Plan) error {
	if change == nil {
		return fmt.Errorf("deck step change is required")
	}
	if plan == nil {
		return fmt.Errorf("plan is required for deck step execution")
	}
	if e.deckRunner == nil {
		return fmt.Errorf("deck runner not configured")
	}

	logger := deckLoggerFromContext(ctx)

	gatewayRef := stringField(change.Fields, "gateway_service_ref")
	if gatewayRef == "" {
		gatewayRef = change.ResourceRef
	}

	selectorName := selectorNameFromFields(change.Fields)
	if selectorName == "" {
		return fmt.Errorf("deck step %s: selector.matchFields.name is required", gatewayRef)
	}

	cpRef := stringField(change.Fields, "control_plane_ref")
	cpID := stringField(change.Fields, "control_plane_id")
	cpName := stringField(change.Fields, "control_plane_name")

	resolvedID, err := e.resolveDeckControlPlaneID(ctx, cpID, cpRef)
	if err != nil {
		return err
	}
	cpID = resolvedID
	if cpID == "" {
		return fmt.Errorf("deck step %s: control plane ID could not be resolved", gatewayRef)
	}

	if cpName == "" {
		cpName, err = e.resolveDeckControlPlaneName(ctx, cpID, cpRef, plan)
		if err != nil {
			return err
		}
	}

	change.Fields["control_plane_id"] = cpID
	change.Fields["control_plane_name"] = cpName

	mode, err := e.resolveDeckMode(plan)
	if err != nil {
		return err
	}

	files, err := parseDeckFiles(change.Fields["files"])
	if err != nil {
		return fmt.Errorf("deck step %s: %w", gatewayRef, err)
	}
	flags, err := parseDeckFlags(change.Fields["flags"])
	if err != nil {
		return fmt.Errorf("deck step %s: %w", gatewayRef, err)
	}
	flags = ensureDeckOutputFlags(flags)

	logger.Debug("Executing deck gateway",
		slog.String("gateway_service_ref", gatewayRef),
		slog.Int("files", len(files)),
	)

	workDir, err := e.resolveDeckWorkDir(change.Fields)
	if err != nil {
		return err
	}

	args := append([]string{"gateway", mode}, flags...)
	args = append(args, files...)

	result, err := e.deckRunner.Run(ctx, deck.RunOptions{
		Args:                    args,
		Mode:                    mode,
		KonnectToken:            e.konnectToken,
		KonnectControlPlaneName: cpName,
		KonnectAddress:          e.konnectBaseURL,
		WorkDir:                 workDir,
	})
	logDeckRunOutput(logger, gatewayRef, 0, result, err)
	if err != nil {
		return fmt.Errorf("deck gateway for gateway_service %s failed: %w", gatewayRef, err)
	}

	if !planNeedsGatewayServiceResolution(plan, gatewayRef) {
		logger.Debug("Skipping gateway service resolution; no dependent changes",
			slog.String("gateway_service_ref", gatewayRef),
		)
		return nil
	}

	serviceID, err := e.resolveGatewayServiceByName(ctx, cpID, selectorName)
	if err != nil {
		return err
	}

	e.storeGatewayServiceRef(gatewayRef, serviceID)
	e.updateGatewayServiceReferences(plan, gatewayRef, serviceID, cpID)

	logger.Debug("Resolved gateway service after deck execution",
		slog.String("gateway_service_ref", gatewayRef),
		slog.String("gateway_service_id", serviceID),
		slog.String("control_plane_id", cpID),
	)

	return nil
}

func (e *Executor) resolveDeckControlPlaneID(ctx context.Context, cpID, cpRef string) (string, error) {
	cpID = strings.TrimSpace(cpID)
	if cpID != "" {
		return cpID, nil
	}

	cpRef = strings.TrimSpace(cpRef)
	if cpRef == "" {
		return "", fmt.Errorf("deck step requires control_plane_ref or control_plane_id")
	}

	if util.IsValidUUID(cpRef) {
		return cpRef, nil
	}

	return e.resolveControlPlaneRef(ctx, planner.ReferenceInfo{Ref: cpRef})
}

func (e *Executor) resolveDeckControlPlaneName(
	ctx context.Context,
	cpID string,
	cpRef string,
	plan *planner.Plan,
) (string, error) {
	if name := controlPlaneNameFromPlan(plan, cpRef); name != "" {
		return name, nil
	}

	if cpID == "" {
		return "", fmt.Errorf("control plane ID is required to resolve name")
	}
	if e.client == nil {
		return "", fmt.Errorf("state client is required to resolve control plane name")
	}

	cp, err := e.client.GetControlPlaneByID(ctx, cpID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve control plane name: %w", err)
	}
	if cp == nil || strings.TrimSpace(cp.Name) == "" {
		return "", fmt.Errorf("control plane %s not found for deck execution", cpID)
	}

	return cp.Name, nil
}

func (e *Executor) resolveDeckMode(plan *planner.Plan) (string, error) {
	mode := e.executionMode
	if mode == "" && plan != nil {
		mode = plan.Metadata.Mode
	}

	switch mode {
	case planner.PlanModeApply:
		return "apply", nil
	case planner.PlanModeSync:
		return "sync", nil
	default:
		return "", fmt.Errorf("deck gateway requires apply or sync mode")
	}
}

func (e *Executor) resolveDeckWorkDir(fields map[string]any) (string, error) {
	raw := stringField(fields, "deck_base_dir")
	if raw == "" {
		return "", nil
	}

	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}

	if e.planBaseDir != "" {
		return filepath.Clean(filepath.Join(e.planBaseDir, raw)), nil
	}

	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve deck base dir %q: %w", raw, err)
	}
	return abs, nil
}

func parseDeckFiles(raw any) ([]string, error) {
	if raw == nil {
		return nil, fmt.Errorf("files are required")
	}
	files, ok := parseStringSlice(raw)
	if !ok {
		return nil, fmt.Errorf("files must be an array of strings")
	}
	cleaned := make([]string, 0, len(files))
	for i, file := range files {
		value := strings.TrimSpace(file)
		if value == "" {
			return nil, fmt.Errorf("files[%d] cannot be empty", i)
		}
		if strings.HasPrefix(value, "-") {
			return nil, fmt.Errorf("files[%d] must be a file path, not a flag", i)
		}
		cleaned = append(cleaned, value)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("files are required")
	}
	return cleaned, nil
}

func parseDeckFlags(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	flags, ok := parseStringSlice(raw)
	if !ok {
		return nil, fmt.Errorf("flags must be an array of strings")
	}
	cleaned := make([]string, 0, len(flags))
	for i, flag := range flags {
		value := strings.TrimSpace(flag)
		if value == "" {
			return nil, fmt.Errorf("flags[%d] cannot be empty", i)
		}
		if !strings.HasPrefix(value, "-") {
			return nil, fmt.Errorf("flags[%d] must be a flag", i)
		}
		cleaned = append(cleaned, value)
	}
	if len(cleaned) == 0 {
		return nil, nil
	}
	return cleaned, nil
}

func parseStringSlice(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return append([]string{}, v...), true
	case []any:
		values := make([]string, len(v))
		for i, item := range v {
			value, ok := item.(string)
			if !ok {
				return nil, false
			}
			values[i] = value
		}
		return values, true
	default:
		return nil, false
	}
}

func ensureDeckOutputFlags(flags []string) []string {
	if !containsDeckFlag(flags, "--json-output") {
		flags = append(flags, "--json-output")
	}
	if !containsDeckFlag(flags, "--no-color") {
		flags = append(flags, "--no-color")
	}
	return flags
}

func containsDeckFlag(flags []string, flag string) bool {
	for _, value := range flags {
		if value == flag || strings.HasPrefix(value, flag+"=") {
			return true
		}
	}
	return false
}

func (e *Executor) resolveGatewayServiceByName(
	ctx context.Context,
	controlPlaneID string,
	selectorName string,
) (string, error) {
	if strings.TrimSpace(controlPlaneID) == "" {
		return "", fmt.Errorf("control plane ID is required to resolve gateway services")
	}
	if e.client == nil {
		return "", fmt.Errorf("state client is required to resolve gateway services")
	}

	services, err := e.client.ListGatewayServices(ctx, controlPlaneID)
	if err != nil {
		return "", fmt.Errorf("failed to list gateway services: %w", err)
	}

	matchID := ""
	for _, svc := range services {
		if svc.Name != selectorName {
			continue
		}
		if matchID != "" {
			return "", fmt.Errorf("gateway_service selector matched multiple services for name %q", selectorName)
		}
		matchID = svc.ID
	}

	if matchID == "" {
		return "", fmt.Errorf("gateway_service not found with name %q in control plane %s", selectorName, controlPlaneID)
	}

	return matchID, nil
}

func (e *Executor) storeGatewayServiceRef(ref, id string) {
	if e.refToID["gateway_service"] == nil {
		e.refToID["gateway_service"] = make(map[string]string)
	}
	e.refToID["gateway_service"][ref] = id
}

func (e *Executor) updateGatewayServiceReferences(
	plan *planner.Plan,
	gatewayRef string,
	serviceID string,
	controlPlaneID string,
) {
	if plan == nil {
		return
	}

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != "api_implementation" || change.Action != planner.ActionCreate {
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

		if !matchesGatewayServiceRef(serviceMap["id"], gatewayRef) {
			continue
		}

		serviceMap["id"] = serviceID
		if strings.TrimSpace(controlPlaneID) != "" {
			serviceMap["control_plane_id"] = controlPlaneID
		}
		change.Fields["service"] = serviceMap
	}
}

func matchesGatewayServiceRef(raw any, gatewayRef string) bool {
	id, ok := raw.(string)
	if !ok {
		return false
	}
	if tags.IsRefPlaceholder(id) {
		ref, field, ok := tags.ParseRefPlaceholder(id)
		return ok && field == "id" && ref == gatewayRef
	}
	return id == gatewayRef
}

func controlPlaneNameFromPlan(plan *planner.Plan, cpRef string) string {
	if plan == nil || cpRef == "" {
		return ""
	}
	for i := range plan.Changes {
		change := plan.Changes[i]
		if change.ResourceType != "control_plane" || change.ResourceRef != cpRef {
			continue
		}
		if name, ok := change.Fields["name"].(string); ok && strings.TrimSpace(name) != "" {
			return name
		}
	}
	return ""
}

func planNeedsGatewayServiceResolution(plan *planner.Plan, gatewayRef string) bool {
	if plan == nil || strings.TrimSpace(gatewayRef) == "" {
		return false
	}

	for i := range plan.Changes {
		change := plan.Changes[i]
		if change.ResourceType != "api_implementation" {
			continue
		}
		if change.Action != planner.ActionCreate && change.Action != planner.ActionUpdate {
			continue
		}
		if gatewayRefMatches(change.Fields, gatewayRef) {
			return true
		}
	}

	return false
}

func gatewayRefMatches(fields map[string]any, gatewayRef string) bool {
	if len(fields) == 0 {
		return false
	}

	svcValue, ok := fields["service"]
	if !ok {
		return false
	}

	svcMap, ok := svcValue.(map[string]any)
	if !ok {
		return false
	}

	idValue, ok := svcMap["id"].(string)
	if !ok || strings.TrimSpace(idValue) == "" {
		return false
	}

	if tags.IsRefPlaceholder(idValue) {
		ref, field, ok := tags.ParseRefPlaceholder(idValue)
		if ok && field == "id" {
			return ref == gatewayRef
		}
		return false
	}

	return idValue == gatewayRef
}

func stringField(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	value, ok := fields[key]
	if !ok {
		return ""
	}
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func logDeckRunOutput(logger *slog.Logger, gatewayRef string, step int, result *deck.RunResult, runErr error) {
	if logger == nil || result == nil {
		return
	}

	stdout := strings.TrimSpace(result.Stdout)
	stderr := strings.TrimSpace(result.Stderr)

	if stdout != "" {
		logger.Debug("deck stdout",
			slog.String("gateway_service_ref", gatewayRef),
			slog.Int("step", step),
			slog.String("stdout", stdout),
		)
	}

	if stderr == "" {
		return
	}

	if runErr != nil {
		logger.Error("deck stderr",
			slog.String("gateway_service_ref", gatewayRef),
			slog.Int("step", step),
			slog.String("stderr", stderr),
		)
		return
	}

	logger.Debug("deck stderr",
		slog.String("gateway_service_ref", gatewayRef),
		slog.Int("step", step),
		slog.String("stderr", stderr),
	)
}

func selectorNameFromFields(fields map[string]any) string {
	if name := stringField(fields, "selector_name"); name != "" {
		return name
	}

	raw, ok := fields["selector"]
	if !ok || raw == nil {
		return ""
	}

	switch selector := raw.(type) {
	case map[string]any:
		return selectorNameFromSelectorMap(selector)
	case map[string]string:
		return selectorNameFromMatchFieldsMap(selector)
	default:
		return ""
	}
}

func selectorNameFromSelectorMap(selector map[string]any) string {
	raw := selector["matchFields"]
	if raw == nil {
		raw = selector["match_fields"]
	}

	switch matchFields := raw.(type) {
	case map[string]any:
		if name, ok := matchFields["name"].(string); ok {
			return strings.TrimSpace(name)
		}
	case map[string]string:
		return strings.TrimSpace(matchFields["name"])
	}

	return ""
}

func selectorNameFromMatchFieldsMap(matchFields map[string]string) string {
	if len(matchFields) == 0 {
		return ""
	}
	return strings.TrimSpace(matchFields["name"])
}

func deckLoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}
