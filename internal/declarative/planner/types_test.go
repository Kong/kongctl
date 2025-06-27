package planner

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewPlan(t *testing.T) {
	plan := NewPlan("1.0", "kongctl/test")

	if plan.Metadata.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", plan.Metadata.Version)
	}

	if plan.Metadata.Generator != "kongctl/test" {
		t.Errorf("Expected generator kongctl/test, got %s", plan.Metadata.Generator)
	}

	if len(plan.Changes) != 0 {
		t.Errorf("Expected empty changes, got %d", len(plan.Changes))
	}

	if !plan.IsEmpty() {
		t.Error("Expected plan to be empty")
	}
}

func TestPlanAddChange(t *testing.T) {
	plan := NewPlan("1.0", "kongctl/test")

	change1 := PlannedChange{
		ID:           "1-c-portal1",
		ResourceType: "portal",
		ResourceRef:  "portal1",
		Action:       ActionCreate,
		Fields:       map[string]interface{}{"name": "Portal 1"},
		ConfigHash:   "hash1",
	}

	change2 := PlannedChange{
		ID:           "2-u-portal2",
		ResourceType: "portal",
		ResourceRef:  "portal2",
		ResourceID:   "existing-id",
		Action:       ActionUpdate,
		Fields:       map[string]interface{}{"description": FieldChange{Old: "old", New: "new"}},
		ConfigHash:   "hash2",
	}

	plan.AddChange(change1)
	plan.AddChange(change2)

	if len(plan.Changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(plan.Changes))
	}

	if plan.IsEmpty() {
		t.Error("Expected plan to not be empty")
	}

	if plan.Summary.TotalChanges != 2 {
		t.Errorf("Expected total changes 2, got %d", plan.Summary.TotalChanges)
	}

	if plan.Summary.ByAction[ActionCreate] != 1 {
		t.Errorf("Expected 1 CREATE action, got %d", plan.Summary.ByAction[ActionCreate])
	}

	if plan.Summary.ByAction[ActionUpdate] != 1 {
		t.Errorf("Expected 1 UPDATE action, got %d", plan.Summary.ByAction[ActionUpdate])
	}

	if plan.Summary.ByResource["portal"] != 2 {
		t.Errorf("Expected 2 portal resources, got %d", plan.Summary.ByResource["portal"])
	}
}

func TestPlanProtectionTracking(t *testing.T) {
	plan := NewPlan("1.0", "kongctl/test")

	// Test CREATE with protection
	change1 := PlannedChange{
		ID:           "1-c-portal1",
		ResourceType: "portal",
		ResourceRef:  "portal1",
		Action:       ActionCreate,
		Protection:   true,
		ConfigHash:   "hash1",
	}
	plan.AddChange(change1)

	if plan.Summary.ProtectionChanges == nil {
		t.Fatal("Expected protection changes to be tracked")
	}
	if plan.Summary.ProtectionChanges.Protecting != 1 {
		t.Errorf("Expected 1 protecting, got %d", plan.Summary.ProtectionChanges.Protecting)
	}

	// Test UPDATE enabling protection
	change2 := PlannedChange{
		ID:           "2-u-portal2",
		ResourceType: "portal",
		ResourceRef:  "portal2",
		Action:       ActionUpdate,
		Protection:   ProtectionChange{Old: false, New: true},
		ConfigHash:   "hash2",
	}
	plan.AddChange(change2)

	if plan.Summary.ProtectionChanges.Protecting != 2 {
		t.Errorf("Expected 2 protecting, got %d", plan.Summary.ProtectionChanges.Protecting)
	}

	// Test UPDATE disabling protection
	change3 := PlannedChange{
		ID:           "3-u-portal3",
		ResourceType: "portal",
		ResourceRef:  "portal3",
		Action:       ActionUpdate,
		Protection:   ProtectionChange{Old: true, New: false},
		ConfigHash:   "hash3",
	}
	plan.AddChange(change3)

	if plan.Summary.ProtectionChanges.Unprotecting != 1 {
		t.Errorf("Expected 1 unprotecting, got %d", plan.Summary.ProtectionChanges.Unprotecting)
	}
}

func TestPlanSetExecutionOrder(t *testing.T) {
	plan := NewPlan("1.0", "kongctl/test")

	order := []string{"1-c-auth", "2-c-portal"}
	plan.SetExecutionOrder(order)

	if len(plan.ExecutionOrder) != 2 {
		t.Errorf("Expected 2 items in execution order, got %d", len(plan.ExecutionOrder))
	}

	if plan.ExecutionOrder[0] != "1-c-auth" {
		t.Errorf("Expected first item to be 1-c-auth, got %s", plan.ExecutionOrder[0])
	}
}

func TestPlanAddWarning(t *testing.T) {
	plan := NewPlan("1.0", "kongctl/test")

	plan.AddWarning("1-c-portal", "Reference will be resolved during execution")

	if len(plan.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(plan.Warnings))
	}

	if plan.Warnings[0].ChangeID != "1-c-portal" {
		t.Errorf("Expected warning for change 1-c-portal, got %s", plan.Warnings[0].ChangeID)
	}
}

func TestPlanJSONSerialization(t *testing.T) {
	plan := NewPlan("1.0", "kongctl/test")

	change := PlannedChange{
		ID:           "1-c-portal",
		ResourceType: "portal",
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
		Fields: map[string]interface{}{
			"name":        "My Portal",
			"description": "Test portal",
		},
		References: map[string]ReferenceInfo{
			"default_application_auth_strategy_id": {
				Ref: "basic-auth",
				ID:  "auth-123",
			},
		},
		ConfigHash: "hash123",
		DependsOn:  []string{"0-c-auth"},
	}

	plan.AddChange(change)
	plan.SetExecutionOrder([]string{"0-c-auth", "1-c-portal"})

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal plan: %v", err)
	}

	// Unmarshal back
	var decoded Plan
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal plan: %v", err)
	}

	// Verify key fields
	if decoded.Metadata.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", decoded.Metadata.Version)
	}

	if len(decoded.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(decoded.Changes))
	}

	if decoded.Changes[0].ID != "1-c-portal" {
		t.Errorf("Expected change ID 1-c-portal, got %s", decoded.Changes[0].ID)
	}

	if len(decoded.ExecutionOrder) != 2 {
		t.Errorf("Expected 2 items in execution order, got %d", len(decoded.ExecutionOrder))
	}
}

func TestFieldChange(t *testing.T) {
	fc := FieldChange{
		Old: "old value",
		New: "new value",
	}

	// Test JSON serialization
	data, err := json.Marshal(fc)
	if err != nil {
		t.Fatalf("Failed to marshal FieldChange: %v", err)
	}

	var decoded FieldChange
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FieldChange: %v", err)
	}

	if decoded.Old != "old value" {
		t.Errorf("Expected old value 'old value', got %v", decoded.Old)
	}

	if decoded.New != "new value" {
		t.Errorf("Expected new value 'new value', got %v", decoded.New)
	}
}

func TestPlanMetadataTimestamp(t *testing.T) {
	before := time.Now().UTC()
	plan := NewPlan("1.0", "kongctl/test")
	after := time.Now().UTC()

	if plan.Metadata.GeneratedAt.Before(before) {
		t.Error("Generated timestamp is before test start time")
	}

	if plan.Metadata.GeneratedAt.After(after) {
		t.Error("Generated timestamp is after test end time")
	}
}