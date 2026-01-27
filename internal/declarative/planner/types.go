package planner

import (
	"strings"
	"time"
)

// Plan represents a declarative configuration plan
type Plan struct {
	Metadata       PlanMetadata    `json:"metadata"`
	Changes        []PlannedChange `json:"changes"`
	ExecutionOrder []string        `json:"execution_order"`
	Summary        PlanSummary     `json:"summary"`
	Warnings       []PlanWarning   `json:"warnings,omitempty"`
}

// PlanMode represents the mode of plan generation
type PlanMode string

const (
	PlanModeSync  PlanMode = "sync"
	PlanModeApply PlanMode = "apply"
)

// PlanMetadata contains plan generation information
type PlanMetadata struct {
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Generator   string    `json:"generator"`
	Mode        PlanMode  `json:"mode"`
}

// PlannedChange represents a single resource change
type PlannedChange struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	ResourceRef  string `json:"resource_ref"`
	ResourceID   string `json:"resource_id,omitempty"` // Only for UPDATE/DELETE
	// Human-readable identifiers for resources without config refs
	ResourceMonikers      map[string]string        `json:"resource_monikers,omitempty"`
	Action                ActionType               `json:"action"`
	Fields                map[string]any           `json:"fields"`
	PostResolutionTargets []PostResolutionTarget   `json:"post_resolution_targets,omitempty"`
	References            map[string]ReferenceInfo `json:"references,omitempty"`
	Parent                *ParentInfo              `json:"parent,omitempty"`
	Protection            any                      `json:"protection,omitempty"` // bool or ProtectionChange
	Namespace             string                   `json:"namespace"`
	DependsOn             []string                 `json:"depends_on,omitempty"`
}

// PostResolutionTarget represents a resource that must be resolved after a change executes.
type PostResolutionTarget struct {
	ResourceType     string                `json:"resource_type"`
	ResourceRef      string                `json:"resource_ref"`
	ControlPlaneRef  string                `json:"control_plane_ref,omitempty"`
	ControlPlaneID   string                `json:"control_plane_id,omitempty"`
	ControlPlaneName string                `json:"control_plane_name,omitempty"`
	Selector         *ExternalToolSelector `json:"selector,omitempty"`
}

// ReferenceInfo tracks reference resolution
type ReferenceInfo struct {
	// Existing fields for single references
	Ref          string            `json:"ref,omitempty"`
	ID           string            `json:"id,omitempty"`            // May be "[unknown]" for resources in same plan
	LookupFields map[string]string `json:"lookup_fields,omitempty"` // Resource-specific identifying fields

	// New fields for array references
	Refs         []string            `json:"refs,omitempty"`          // Array of reference strings
	ResolvedIDs  []string            `json:"resolved_ids,omitempty"`  // Array of resolved UUIDs
	LookupArrays map[string][]string `json:"lookup_arrays,omitempty"` // Array lookup fields
	IsArray      bool                `json:"is_array,omitempty"`      // Flag to indicate array reference
}

// ParentInfo tracks parent relationships
type ParentInfo struct {
	Ref string `json:"ref"`
	ID  string `json:"id"` // May be "[unknown]" for parents in same plan
}

// ProtectionChange tracks protection status changes
type ProtectionChange struct {
	Old bool `json:"old"`
	New bool `json:"new"`
}

// FieldChange represents a single field modification (for UPDATE)
type FieldChange struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// ActionType represents the type of change
type ActionType string

const (
	ActionCreate       ActionType = "CREATE"
	ActionUpdate       ActionType = "UPDATE"
	ActionDelete       ActionType = "DELETE"
	ActionExternalTool ActionType = "EXTERNAL_TOOL"
)

// PlanSummary provides overview statistics
type PlanSummary struct {
	TotalChanges      int                                 `json:"total_changes"`
	ByAction          map[ActionType]int                  `json:"by_action"`
	ByResource        map[string]int                      `json:"by_resource"`
	ByExternalTools   map[string][]ExternalToolDependency `json:"by_external_tools,omitempty"`
	ProtectionChanges *ProtectionSummary                  `json:"protection_changes,omitempty"`
}

// ProtectionSummary tracks protection changes
type ProtectionSummary struct {
	Protecting   int `json:"protecting"`
	Unprotecting int `json:"unprotecting"`
}

// PlanWarning represents a warning about the plan
type PlanWarning struct {
	ChangeID string `json:"change_id"`
	Message  string `json:"message"`
}

// ExternalToolDependency captures external tool execution requirements for summary output.
type ExternalToolDependency struct {
	ControlPlaneRef  string                      `json:"control_plane_ref,omitempty"`
	ControlPlaneID   string                      `json:"control_plane_id,omitempty"`
	ControlPlaneName string                      `json:"control_plane_name,omitempty"`
	GatewayServices  []ExternalToolGatewayTarget `json:"gateway_services,omitempty"`
	Files            []string                    `json:"files,omitempty"`
	Flags            []string                    `json:"flags,omitempty"`
	DeckBaseDir      string                      `json:"deck_base_dir,omitempty"`
}

// ExternalToolGatewayTarget represents a gateway service selector tied to a deck run.
type ExternalToolGatewayTarget struct {
	Ref      string                `json:"ref"`
	Selector *ExternalToolSelector `json:"selector,omitempty"`
}

// ExternalToolSelector represents selector match fields for external tool dependencies.
type ExternalToolSelector struct {
	MatchFields map[string]string `json:"matchFields"`
}

// NewPlan creates a new plan with metadata
func NewPlan(version, generator string, mode PlanMode) *Plan {
	return &Plan{
		Metadata: PlanMetadata{
			Version:     version,
			GeneratedAt: time.Now().UTC(),
			Generator:   generator,
			Mode:        mode,
		},
		Changes:        []PlannedChange{},
		ExecutionOrder: []string{},
		Summary: PlanSummary{
			ByAction:   make(map[ActionType]int),
			ByResource: make(map[string]int),
		},
		Warnings: []PlanWarning{},
	}
}

// AddChange adds a change to the plan
func (p *Plan) AddChange(change PlannedChange) {
	p.Changes = append(p.Changes, change)
	p.UpdateSummary()
}

// HasChange returns true if the plan already contains a change for the given resource type and ref.
func (p *Plan) HasChange(resourceType, resourceRef string) bool {
	for _, change := range p.Changes {
		if change.ResourceType == resourceType && change.ResourceRef == resourceRef {
			return true
		}
	}
	return false
}

// SetExecutionOrder sets the calculated execution order
func (p *Plan) SetExecutionOrder(order []string) {
	p.ExecutionOrder = order
}

// AddWarning adds a warning to the plan
func (p *Plan) AddWarning(changeID, message string) {
	p.Warnings = append(p.Warnings, PlanWarning{
		ChangeID: changeID,
		Message:  message,
	})
}

// UpdateSummary recalculates plan statistics
func (p *Plan) UpdateSummary() {
	p.Summary.TotalChanges = len(p.Changes)

	// Reset counts
	p.Summary.ByAction = make(map[ActionType]int)
	p.Summary.ByResource = make(map[string]int)
	p.Summary.ByExternalTools = nil
	protectionSummary := &ProtectionSummary{}
	var externalTools map[string][]ExternalToolDependency

	// Count by action and resource type
	for _, change := range p.Changes {
		p.Summary.ByAction[change.Action]++
		p.Summary.ByResource[change.ResourceType]++
		if change.Action == ActionExternalTool {
			dependency := externalToolDependencyFromChange(change)
			if externalTools == nil {
				externalTools = make(map[string][]ExternalToolDependency)
			}
			externalTools[change.ResourceType] = append(externalTools[change.ResourceType], dependency)
		}

		// Track protection changes
		switch v := change.Protection.(type) {
		case bool:
			if v && change.Action == ActionCreate {
				protectionSummary.Protecting++
			}
		case ProtectionChange:
			if !v.Old && v.New {
				protectionSummary.Protecting++
			} else if v.Old && !v.New {
				protectionSummary.Unprotecting++
			}
		}
	}

	if protectionSummary.Protecting > 0 || protectionSummary.Unprotecting > 0 {
		p.Summary.ProtectionChanges = protectionSummary
	}

	if len(externalTools) > 0 {
		p.Summary.ByExternalTools = externalTools
	}
}

func externalToolDependencyFromChange(change PlannedChange) ExternalToolDependency {
	fields := change.Fields
	dependency := ExternalToolDependency{
		ControlPlaneRef:  stringFromField(fields, "control_plane_ref"),
		ControlPlaneID:   stringFromField(fields, "control_plane_id"),
		ControlPlaneName: stringFromField(fields, "control_plane_name"),
		GatewayServices:  gatewayTargetsFromPostResolutionTargets(change.PostResolutionTargets),
		Files:            stringSliceFromField(fields, "files"),
		Flags:            stringSliceFromField(fields, "flags"),
		DeckBaseDir:      stringFromField(fields, "deck_base_dir"),
	}

	if len(dependency.GatewayServices) == 0 {
		dependency.GatewayServices = gatewayTargetsFromFields(fields)
	}

	return dependency
}

func gatewayTargetsFromPostResolutionTargets(targets []PostResolutionTarget) []ExternalToolGatewayTarget {
	if len(targets) == 0 {
		return nil
	}

	out := make([]ExternalToolGatewayTarget, 0, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.ResourceRef) == "" {
			continue
		}
		if target.ResourceType != "" && target.ResourceType != "gateway_service" {
			continue
		}
		out = append(out, ExternalToolGatewayTarget{
			Ref:      target.ResourceRef,
			Selector: cloneExternalToolSelector(target.Selector),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func gatewayTargetsFromFields(fields map[string]any) []ExternalToolGatewayTarget {
	if len(fields) == 0 {
		return nil
	}

	raw, ok := fields["gateway_services"]
	if !ok || raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []ExternalToolGatewayTarget:
		return v
	case []map[string]any:
		out := make([]ExternalToolGatewayTarget, 0, len(v))
		for _, entry := range v {
			ref, _ := entry["ref"].(string)
			selector := selectorFromEntry(entry)
			if ref == "" {
				continue
			}
			out = append(out, ExternalToolGatewayTarget{Ref: ref, Selector: selector})
		}
		return out
	case []any:
		out := make([]ExternalToolGatewayTarget, 0, len(v))
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ref, _ := entry["ref"].(string)
			selector := selectorFromEntry(entry)
			if ref == "" {
				continue
			}
			out = append(out, ExternalToolGatewayTarget{Ref: ref, Selector: selector})
		}
		return out
	}

	return nil
}

func selectorFromEntry(entry map[string]any) *ExternalToolSelector {
	if len(entry) == 0 {
		return nil
	}
	if name, ok := entry["selector_name"].(string); ok && name != "" {
		return &ExternalToolSelector{MatchFields: map[string]string{"name": name}}
	}
	raw, ok := entry["selector"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case ExternalToolSelector:
		return cloneExternalToolSelector(&v)
	case *ExternalToolSelector:
		return cloneExternalToolSelector(v)
	case map[string]any:
		return selectorFromSelectorMap(v)
	case map[string]string:
		matchFields := selectorFromMatchFieldsMap(v)
		if matchFields == nil {
			return nil
		}
		return &ExternalToolSelector{MatchFields: matchFields}
	default:
		return nil
	}
}

func cloneExternalToolSelector(selector *ExternalToolSelector) *ExternalToolSelector {
	if selector == nil {
		return nil
	}
	matchFields := selectorFromMatchFieldsMap(selector.MatchFields)
	if matchFields == nil {
		return nil
	}
	return &ExternalToolSelector{MatchFields: matchFields}
}

func selectorFromSelectorMap(selector map[string]any) *ExternalToolSelector {
	raw := selector["matchFields"]
	if raw == nil {
		raw = selector["match_fields"]
	}

	switch v := raw.(type) {
	case map[string]string:
		matchFields := selectorFromMatchFieldsMap(v)
		if matchFields == nil {
			return nil
		}
		return &ExternalToolSelector{MatchFields: matchFields}
	case map[string]any:
		matchFields := selectorMatchFieldsFromAny(v)
		if matchFields == nil {
			return nil
		}
		return &ExternalToolSelector{MatchFields: matchFields}
	default:
		return nil
	}
}

func selectorFromMatchFieldsMap(matchFields map[string]string) map[string]string {
	if len(matchFields) == 0 {
		return nil
	}
	clone := make(map[string]string, len(matchFields))
	for key, value := range matchFields {
		if value == "" {
			continue
		}
		clone[key] = value
	}
	if len(clone) == 0 {
		return nil
	}
	return clone
}

func selectorMatchFieldsFromAny(matchFields map[string]any) map[string]string {
	if len(matchFields) == 0 {
		return nil
	}
	converted := make(map[string]string, len(matchFields))
	for key, value := range matchFields {
		str, ok := value.(string)
		if !ok || str == "" {
			continue
		}
		converted[key] = str
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func stringFromField(fields map[string]any, key string) string {
	if len(fields) == 0 {
		return ""
	}
	if value, ok := fields[key].(string); ok {
		return value
	}
	return ""
}

func stringSliceFromField(fields map[string]any, key string) []string {
	if len(fields) == 0 {
		return nil
	}
	value, ok := fields[key]
	if !ok {
		return nil
	}
	items, ok := asStringSlice(value)
	if !ok {
		return nil
	}
	clone := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		clone = append(clone, item)
	}
	if len(clone) == 0 {
		return nil
	}
	return clone
}

// IsEmpty returns true if plan has no changes
func (p *Plan) IsEmpty() bool {
	return len(p.Changes) == 0
}

// ContainsDeletes returns true if plan contains any DELETE operations
func (p *Plan) ContainsDeletes() bool {
	for _, change := range p.Changes {
		if change.Action == ActionDelete {
			return true
		}
	}
	return false
}
