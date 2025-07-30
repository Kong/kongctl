package planner

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/labels"
)

// GenericPlanner provides common planning operations for all resource types
type GenericPlanner struct {
	planner *Planner
}

// NewGenericPlanner creates a new generic planner instance
func NewGenericPlanner(p *Planner) *GenericPlanner {
	return &GenericPlanner{planner: p}
}

// CreateConfig defines configuration for generic create operations
type CreateConfig struct {
	ResourceType    string
	ResourceName    string
	ResourceRef     string
	RequiredFields  []string
	FieldExtractor  func(resource interface{}) map[string]interface{}
	Namespace       string
	DependsOn       []string
	References      map[string]ReferenceInfo
	Parent          *ParentInfo
}

// UpdateConfig defines configuration for generic update operations
type UpdateConfig struct {
	ResourceType      string
	ResourceName      string
	ResourceID        string
	CurrentFields     map[string]interface{}
	DesiredFields     map[string]interface{}
	CurrentLabels     map[string]string
	DesiredLabels     map[string]string
	RequiredFields    []string
	FieldComparator   func(current, desired map[string]interface{}) bool
	Namespace         string
	References        map[string]ReferenceInfo
}

// DeleteConfig defines configuration for generic delete operations
type DeleteConfig struct {
	ResourceType string
	ResourceName string
	ResourceRef  string
	ResourceID   string
	Namespace    string
}

// ProtectionChangeConfig defines configuration for protection change operations
type ProtectionChangeConfig struct {
	ResourceType  string
	ResourceName  string
	ResourceRef   string
	ResourceID    string
	OldProtected  bool
	NewProtected  bool
	Namespace     string
}

// PlanCreate creates a planned change for resource creation
func (g *GenericPlanner) PlanCreate(_ context.Context, config CreateConfig) (PlannedChange, error) {
	// Create fields map
	fields := config.FieldExtractor(nil)
	
	// Validate required fields
	for _, field := range config.RequiredFields {
		if _, ok := fields[field]; !ok {
			return PlannedChange{}, fmt.Errorf("required field %s not found in %s resource", 
				field, config.ResourceType)
		}
	}
	
	// Create the planned change
	changeID := g.planner.nextChangeID(ActionCreate, config.ResourceType, config.ResourceRef)
	
	change := PlannedChange{
		ID:           changeID,
		Action:       ActionCreate,
		ResourceType: config.ResourceType,
		ResourceRef:  config.ResourceRef,
		Fields:       fields,
		Namespace:    config.Namespace,
		DependsOn:    config.DependsOn,
		References:   config.References,
		Parent:       config.Parent,
	}
	
	return change, nil
}

// ShouldUpdate determines if an update is needed by comparing current and desired state
func (g *GenericPlanner) ShouldUpdate(config UpdateConfig) bool {
	// Use custom comparator if provided
	if config.FieldComparator != nil {
		return config.FieldComparator(config.CurrentFields, config.DesiredFields)
	}
	
	// Default field comparison
	if !reflect.DeepEqual(config.CurrentFields, config.DesiredFields) {
		return true
	}
	
	// Compare labels if provided
	if config.CurrentLabels != nil && config.DesiredLabels != nil {
		return !LabelsEqual(config.CurrentLabels, config.DesiredLabels)
	}
	
	return false
}

// PlanUpdate creates a planned change for resource update
func (g *GenericPlanner) PlanUpdate(_ context.Context, config UpdateConfig) (PlannedChange, error) {
	// Validate required fields
	for _, field := range config.RequiredFields {
		if _, ok := config.DesiredFields[field]; !ok {
			return PlannedChange{}, fmt.Errorf("required field %s not found in %s resource", 
				field, config.ResourceType)
		}
	}
	
	// Create the planned change
	changeID := g.planner.nextChangeID(ActionUpdate, config.ResourceType, config.ResourceName)
	
	change := PlannedChange{
		ID:           changeID,
		Action:       ActionUpdate,
		ResourceType: config.ResourceType,
		ResourceRef:  config.ResourceName,
		ResourceID:   config.ResourceID,
		Fields:       config.DesiredFields,
		Namespace:    config.Namespace,
		References:   config.References,
	}
	
	return change, nil
}

// PlanDelete creates a planned change for resource deletion
func (g *GenericPlanner) PlanDelete(_ context.Context, config DeleteConfig) PlannedChange {
	changeID := g.planner.nextChangeID(ActionDelete, config.ResourceType, config.ResourceRef)
	
	return PlannedChange{
		ID:           changeID,
		Action:       ActionDelete,
		ResourceType: config.ResourceType,
		ResourceRef:  config.ResourceRef,
		ResourceID:   config.ResourceID,
		Namespace:    config.Namespace,
	}
}

// PlanProtectionChange creates a planned change for protection status update
func (g *GenericPlanner) PlanProtectionChange(_ context.Context, config ProtectionChangeConfig) PlannedChange {
	changeID := g.planner.nextChangeID(ActionUpdate, config.ResourceType, config.ResourceRef)
	
	return PlannedChange{
		ID:           changeID,
		Action:       ActionUpdate,
		ResourceType: config.ResourceType,
		ResourceRef:  config.ResourceRef,
		ResourceID:   config.ResourceID,
		Protection: ProtectionChange{
			Old: config.OldProtected,
			New: config.NewProtected,
		},
		Namespace: config.Namespace,
	}
}

// LabelsEqual compares two label maps for equality
func LabelsEqual(current, desired map[string]string) bool {
	// Handle kongctl-managed labels specially
	currentManaged := normalizeLabels(current)
	desiredManaged := normalizeLabels(desired)
	
	return reflect.DeepEqual(currentManaged, desiredManaged)
}

// LabelsEqualPtr compares two label maps with pointer values
func LabelsEqualPtr(current map[string]*string, desired map[string]string) bool {
	// Convert pointer map to regular map
	currentMap := make(map[string]string)
	for k, v := range current {
		if v != nil {
			currentMap[k] = *v
		}
	}
	
	return LabelsEqual(currentMap, desired)
}

// normalizeLabels extracts only kongctl-managed labels
func normalizeLabels(labelMap map[string]string) map[string]string {
	result := make(map[string]string)
	
	// Only include kongctl-managed labels
	for k, v := range labelMap {
		if k == labels.NamespaceKey || k == labels.ProtectedKey || k == labels.ManagedKey {
			result[k] = v
		}
	}
	
	return result
}

// ExtractFields is a helper to create field extractors for resources with SDK types
func ExtractFields(resource interface{}, fieldMapping func(interface{}) map[string]interface{}) map[string]interface{} {
	if resource == nil {
		return make(map[string]interface{})
	}
	return fieldMapping(resource)
}