package executor

import (
	"context"
	"encoding/json"
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

	cpRef := stringField(change.Fields, "control_plane_ref")
	if cpRef == "" {
		cpRef = change.ResourceRef
	}
	cpID := stringField(change.Fields, "control_plane_id")
	cpName := stringField(change.Fields, "control_plane_name")

	resolvedID, err := e.resolveDeckControlPlaneID(ctx, cpID, cpRef)
	if err != nil {
		return err
	}
	cpID = resolvedID
	if cpID == "" {
		return fmt.Errorf("deck step %s: control plane ID could not be resolved", cpRef)
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
		return fmt.Errorf("deck step %s: %w", cpRef, err)
	}
	flags, err := parseDeckFlags(change.Fields["flags"])
	if err != nil {
		return fmt.Errorf("deck step %s: %w", cpRef, err)
	}
	flags = ensureDeckOutputFlags(flags)

	logger.Debug("Executing deck gateway",
		slog.String("control_plane_ref", cpRef),
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
	logDeckRunOutput(logger, cpRef, 0, result, err)
	if err != nil {
		return fmt.Errorf("deck gateway for control_plane %s failed: %w%s",
			cpRef,
			err,
			deckRunErrorSuffix(result),
		)
	}

	services, err := deckGatewayServicesFromChange(change)
	if err != nil {
		return err
	}

	if len(services) == 0 {
		return nil
	}

	for _, svc := range services {
		if !planNeedsGatewayServiceResolution(plan, svc.Ref) {
			logger.Debug("Skipping gateway service resolution; no dependent changes",
				slog.String("gateway_service_ref", svc.Ref),
			)
			continue
		}
		if svc.SelectorName == "" {
			return fmt.Errorf("deck step %s: selector.matchFields.name is required for gateway_service %s",
				cpRef, svc.Ref)
		}

		serviceID, err := e.resolveGatewayServiceByName(ctx, cpID, svc.SelectorName)
		if err != nil {
			return err
		}

		e.storeGatewayServiceRef(svc.Ref, serviceID)
		e.updateGatewayServiceReferences(plan, svc.Ref, serviceID, cpID)

		logger.Debug("Resolved gateway service after deck execution",
			slog.String("gateway_service_ref", svc.Ref),
			slog.String("gateway_service_id", serviceID),
			slog.String("control_plane_id", cpID),
		)
	}

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
	files, ok := util.StringSliceFromAny(raw)
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

type deckGatewayServiceRef struct {
	Ref          string
	SelectorName string
}

func deckGatewayServicesFromChange(change *planner.PlannedChange) ([]deckGatewayServiceRef, error) {
	if change == nil {
		return nil, nil
	}
	if len(change.PostResolutionTargets) > 0 {
		return deckGatewayServicesFromTargets(change.PostResolutionTargets), nil
	}
	return deckGatewayServicesFromFields(change.Fields)
}

func deckGatewayServicesFromTargets(targets []planner.PostResolutionTarget) []deckGatewayServiceRef {
	if len(targets) == 0 {
		return nil
	}

	services := make([]deckGatewayServiceRef, 0, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.ResourceRef) == "" {
			continue
		}
		if target.ResourceType != "" && target.ResourceType != "gateway_service" {
			continue
		}
		name := ""
		if target.Selector != nil {
			name = target.Selector.MatchFields["name"]
		}
		services = append(services, deckGatewayServiceRef{
			Ref:          target.ResourceRef,
			SelectorName: name,
		})
	}

	if len(services) == 0 {
		return nil
	}

	return services
}

func deckGatewayServicesFromFields(fields map[string]any) ([]deckGatewayServiceRef, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	raw, ok := fields["gateway_services"]
	if !ok || raw == nil {
		return nil, nil
	}

	services := make([]deckGatewayServiceRef, 0)

	switch v := raw.(type) {
	case []map[string]any:
		for _, entry := range v {
			services = append(services, deckGatewayServiceFromEntry(entry))
		}
	case []any:
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			services = append(services, deckGatewayServiceFromEntry(entry))
		}
	default:
		return nil, fmt.Errorf("gateway_services must be an array of objects")
	}

	cleaned := services[:0]
	for _, svc := range services {
		if strings.TrimSpace(svc.Ref) == "" {
			continue
		}
		cleaned = append(cleaned, svc)
	}

	return cleaned, nil
}

func deckGatewayServiceFromEntry(entry map[string]any) deckGatewayServiceRef {
	svc := deckGatewayServiceRef{
		Ref: stringField(entry, "ref"),
	}

	if name := stringField(entry, "selector_name"); name != "" {
		svc.SelectorName = name
		return svc
	}

	raw, ok := entry["selector"]
	if !ok || raw == nil {
		return svc
	}

	switch v := raw.(type) {
	case map[string]any:
		if matchFields, ok := v["matchFields"].(map[string]string); ok {
			svc.SelectorName = matchFields["name"]
		} else if matchFields, ok := v["matchFields"].(map[string]any); ok {
			if name, ok := matchFields["name"].(string); ok {
				svc.SelectorName = name
			}
		}
	case map[string]string:
		svc.SelectorName = v["name"]
	}

	return svc
}

func parseDeckFlags(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	flags, ok := util.StringSliceFromAny(raw)
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
		return "", fmt.Errorf(
			"gateway_service not found with name %q in control plane %s",
			selectorName,
			controlPlaneID,
		)
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

func logDeckRunOutput(logger *slog.Logger, controlPlaneRef string, step int, result *deck.RunResult, runErr error) {
	if logger == nil || result == nil {
		return
	}

	stdout := strings.TrimSpace(result.Stdout)
	stderr := strings.TrimSpace(result.Stderr)

	if stdout != "" {
		if summary, ok := deckSummaryFromJSON(stdout); ok {
			logDeckSummary(logger, controlPlaneRef, step, summary)
		} else if !looksLikeJSON(stdout) {
			logger.Debug("deck stdout",
				slog.String("control_plane_ref", controlPlaneRef),
				slog.Int("step", step),
				slog.String("stdout", truncateDeckOutput(stdout, 4096)),
			)
		} else {
			logger.Debug("deck stdout omitted (json output)",
				slog.String("control_plane_ref", controlPlaneRef),
				slog.Int("step", step),
			)
		}
	}

	if stderr == "" {
		return
	}

	if runErr != nil {
		logger.Error("deck stderr",
			slog.String("control_plane_ref", controlPlaneRef),
			slog.Int("step", step),
			slog.String("stderr", stderr),
		)
		return
	}

	logger.Debug("deck stderr",
		slog.String("control_plane_ref", controlPlaneRef),
		slog.Int("step", step),
		slog.String("stderr", truncateDeckOutput(stderr, 4096)),
	)
}

func deckLoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}

type deckSummary struct {
	Kind        string
	Created     int
	Updated     int
	Deleted     int
	Creating    int
	Updating    int
	Deleting    int
	Total       int
	Warnings    int
	Errors      int
	HasSummary  bool
	HasWarnings bool
	HasErrors   bool
}

func deckSummaryFromJSON(stdout string) (deckSummary, bool) {
	var payload struct {
		Summary  map[string]any `json:"summary"`
		Warnings []any          `json:"warnings"`
		Errors   []any          `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return deckSummary{}, false
	}
	if payload.Summary == nil {
		return deckSummary{}, false
	}

	summary := deckSummary{
		Warnings: len(payload.Warnings),
		Errors:   len(payload.Errors),
	}
	if summary.Warnings > 0 {
		summary.HasWarnings = true
	}
	if summary.Errors > 0 {
		summary.HasErrors = true
	}

	if created, ok := intFromAny(payload.Summary["created"]); ok {
		summary.Kind = "apply"
		summary.Created = created
		summary.Updated = intFromAnyDefault(payload.Summary["updated"])
		summary.Deleted = intFromAnyDefault(payload.Summary["deleted"])
		summary.HasSummary = true
		return summary, true
	}
	if creating, ok := intFromAny(payload.Summary["creating"]); ok {
		summary.Kind = "diff"
		summary.Creating = creating
		summary.Updating = intFromAnyDefault(payload.Summary["updating"])
		summary.Deleting = intFromAnyDefault(payload.Summary["deleting"])
		summary.Total = intFromAnyDefault(payload.Summary["total"])
		summary.HasSummary = true
		return summary, true
	}

	return deckSummary{}, false
}

func logDeckSummary(logger *slog.Logger, controlPlaneRef string, step int, summary deckSummary) {
	if logger == nil || !summary.HasSummary {
		return
	}
	if summary.Kind == "diff" {
		logger.Debug("deck diff summary",
			slog.String("control_plane_ref", controlPlaneRef),
			slog.Int("step", step),
			slog.Int("creating", summary.Creating),
			slog.Int("updating", summary.Updating),
			slog.Int("deleting", summary.Deleting),
			slog.Int("total", summary.Total),
			slog.Int("warnings", summary.Warnings),
			slog.Int("errors", summary.Errors),
		)
		return
	}

	logger.Debug("deck summary",
		slog.String("control_plane_ref", controlPlaneRef),
		slog.Int("step", step),
		slog.Int("created", summary.Created),
		slog.Int("updated", summary.Updated),
		slog.Int("deleted", summary.Deleted),
		slog.Int("warnings", summary.Warnings),
		slog.Int("errors", summary.Errors),
	)
}

func looksLikeJSON(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

func intFromAnyDefault(value any) int {
	if val, ok := intFromAny(value); ok {
		return val
	}
	return 0
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
