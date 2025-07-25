package planner

import (
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
	ID               string                    `json:"id"`
	ResourceType     string                    `json:"resource_type"`
	ResourceRef      string                    `json:"resource_ref"`
	ResourceID       string                    `json:"resource_id,omitempty"` // Only for UPDATE/DELETE
	// Human-readable identifiers for resources without config refs
	ResourceMonikers map[string]string         `json:"resource_monikers,omitempty"`
	Action           ActionType                `json:"action"`
	Fields           map[string]interface{}    `json:"fields"`
	References       map[string]ReferenceInfo  `json:"references,omitempty"`
	Parent           *ParentInfo               `json:"parent,omitempty"`
	Protection       interface{}               `json:"protection,omitempty"` // bool or ProtectionChange
	Namespace        string                    `json:"namespace"`
	DependsOn        []string                  `json:"depends_on,omitempty"`
}

// ReferenceInfo tracks reference resolution
type ReferenceInfo struct {
	Ref          string            `json:"ref"`
	ID           string            `json:"id"` // May be "[unknown]" for resources in same plan
	LookupFields map[string]string `json:"lookup_fields,omitempty"` // Resource-specific identifying fields
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
	Old interface{} `json:"old"`
	New interface{} `json:"new"`
}

// ActionType represents the type of change
type ActionType string

const (
	ActionCreate ActionType = "CREATE"
	ActionUpdate ActionType = "UPDATE"
	ActionDelete ActionType = "DELETE" // Future
)

// PlanSummary provides overview statistics
type PlanSummary struct {
	TotalChanges      int                `json:"total_changes"`
	ByAction          map[ActionType]int `json:"by_action"`
	ByResource        map[string]int     `json:"by_resource"`
	ProtectionChanges *ProtectionSummary `json:"protection_changes,omitempty"`
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
	protectionSummary := &ProtectionSummary{}
	
	// Count by action and resource type
	for _, change := range p.Changes {
		p.Summary.ByAction[change.Action]++
		p.Summary.ByResource[change.ResourceType]++
		
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