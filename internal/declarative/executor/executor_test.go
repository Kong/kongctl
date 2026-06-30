package executor

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProgressReporter for testing
type MockProgressReporter struct {
	mock.Mock
	StartExecutionCalled  int
	FinishExecutionCalled int
	StartChangeCalls      []planner.PlannedChange
	CompleteChangeCalls   []planner.PlannedChange
	SkipChangeCalls       []planner.PlannedChange
	SkipReasons           []string
}

func (m *MockProgressReporter) StartExecution(plan *planner.Plan) {
	m.StartExecutionCalled++
	m.Called(plan)
}

func (m *MockProgressReporter) StartChange(change planner.PlannedChange) {
	m.StartChangeCalls = append(m.StartChangeCalls, change)
	m.Called(change)
}

func (m *MockProgressReporter) CompleteChange(change planner.PlannedChange, err error) {
	m.CompleteChangeCalls = append(m.CompleteChangeCalls, change)
	m.Called(change, err)
}

func (m *MockProgressReporter) SkipChange(change planner.PlannedChange, reason string) {
	m.SkipChangeCalls = append(m.SkipChangeCalls, change)
	m.SkipReasons = append(m.SkipReasons, reason)
	m.Called(change, reason)
}

func (m *MockProgressReporter) FinishExecution(result *ExecutionResult) {
	m.FinishExecutionCalled++
	m.Called(result)
}

func TestExecutor_New(t *testing.T) {
	reporter := &MockProgressReporter{}

	exec := New(nil, reporter, false)

	assert.NotNil(t, exec)
	assert.Nil(t, exec.client)
	assert.Equal(t, reporter, exec.reporter)
	assert.False(t, exec.dryRun)

	// Test with dry-run
	execDryRun := New(nil, reporter, true)
	assert.True(t, execDryRun.dryRun)
}

func TestExecutor_NewWithOptions_ConcurrencyBounds(t *testing.T) {
	tests := []struct {
		name     string
		max      int
		expected int
	}{
		{
			name:     "uses default when unset",
			max:      0,
			expected: DefaultMaxConcurrency,
		},
		{
			name:     "uses default when non-positive",
			max:      -1,
			expected: DefaultMaxConcurrency,
		},
		{
			name:     "keeps value within range",
			max:      7,
			expected: 7,
		},
		{
			name:     "clamps above maximum",
			max:      MaxConcurrency + 1,
			expected: MaxConcurrency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewWithOptions(nil, nil, false, Options{MaxConcurrency: tt.max})
			assert.Equal(t, tt.expected, exec.concurrency)
		})
	}
}

func TestValidateChangePreExecutionAllowsPortalTeamGroupMappingWithoutResourceID(t *testing.T) {
	exec := New(nil, nil, false)
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypePortalTeamGroupMapping,
		ResourceRef:  "team-groups",
		Action:       planner.ActionUpdate,
	}

	err := exec.validateChangePreExecution(context.Background(), change)

	require.NoError(t, err)
}

func TestHydrateKnownReferenceIDsUpdatesNestedField(t *testing.T) {
	exec := New(nil, nil, false)
	exec.createdResources["1-c-static-key"] = "static-key-id"

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1-c-static-key",
		ResourceType: planner.ResourceTypeEventGatewayStaticKey,
		ResourceRef:  "static-key",
		Action:       planner.ActionCreate,
	})

	change := planner.PlannedChange{
		ID:           "2-c-produce-policy",
		ResourceType: planner.ResourceTypeEventGatewayProducePolicy,
		ResourceRef:  "produce-policy",
		Action:       planner.ActionCreate,
		DependsOn:    []string{"1-c-static-key"},
		Fields: map[string]any{
			planner.FieldConfig: map[string]any{
				"encryption_key": map[string]any{
					"key": map[string]any{
						planner.FieldID: "__REF__:static-key#id",
					},
				},
			},
		},
		References: map[string]planner.ReferenceInfo{
			"config.encryption_key.key.id": {
				Ref: "__REF__:static-key#id",
				ID:  resources.UnknownReferenceID,
			},
		},
	}
	plan.AddChange(change)

	exec.hydrateKnownReferenceIDs(&change, plan)

	ref := change.References["config.encryption_key.key.id"]
	assert.Equal(t, "static-key-id", ref.ID)
	config := change.Fields[planner.FieldConfig].(map[string]any)
	encryptionKey := config["encryption_key"].(map[string]any)
	key := encryptionKey["key"].(map[string]any)
	assert.Equal(t, "static-key-id", key[planner.FieldID])
	assert.NotContains(t, change.Fields, "config.encryption_key.key.id")
}

func TestHydrateKnownReferenceIDsUpdatesEncryptFieldsStaticKey(t *testing.T) {
	exec := New(nil, nil, false)
	exec.createdResources["1-c-static-key"] = "static-key-id"

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1-c-static-key",
		ResourceType: planner.ResourceTypeEventGatewayStaticKey,
		ResourceRef:  "static-key",
		Action:       planner.ActionCreate,
	})

	change := planner.PlannedChange{
		ID:           "2-c-produce-policy",
		ResourceType: planner.ResourceTypeEventGatewayProducePolicy,
		ResourceRef:  "produce-policy",
		Action:       planner.ActionCreate,
		DependsOn:    []string{"1-c-static-key"},
		Fields: map[string]any{
			planner.FieldConfig: map[string]any{
				"encrypt_fields": []any{
					map[string]any{
						"encryption_key": map[string]any{
							"key": map[string]any{
								planner.FieldID: "__REF__:static-key#id",
							},
						},
					},
				},
			},
		},
		References: map[string]planner.ReferenceInfo{
			"config.encrypt_fields.0.encryption_key.key.id": {
				Ref: "__REF__:static-key#id",
				ID:  resources.UnknownReferenceID,
			},
		},
	}
	plan.AddChange(change)

	exec.hydrateKnownReferenceIDs(&change, plan)

	ref := change.References["config.encrypt_fields.0.encryption_key.key.id"]
	assert.Equal(t, "static-key-id", ref.ID)
	config := change.Fields[planner.FieldConfig].(map[string]any)
	encryptFields := config["encrypt_fields"].([]any)
	firstField := encryptFields[0].(map[string]any)
	encryptionKey := firstField["encryption_key"].(map[string]any)
	key := encryptionKey["key"].(map[string]any)
	assert.Equal(t, "static-key-id", key[planner.FieldID])
}

func TestHydrateKnownReferenceIDsUpdatesPolicyParentID(t *testing.T) {
	exec := New(nil, nil, false)
	exec.createdResources["1-c-schema-validation"] = "schema-validation-id"

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1-c-schema-validation",
		ResourceType: planner.ResourceTypeEventGatewayProducePolicy,
		ResourceRef:  "schema-validation",
		Action:       planner.ActionCreate,
	})

	change := planner.PlannedChange{
		ID:           "2-c-encrypt-fields",
		ResourceType: planner.ResourceTypeEventGatewayProducePolicy,
		ResourceRef:  "encrypt-fields",
		Action:       planner.ActionCreate,
		DependsOn:    []string{"1-c-schema-validation"},
		Fields: map[string]any{
			planner.FieldParentPolicyID: "__REF__:schema-validation#id",
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldParentPolicyID: {
				Ref: "__REF__:schema-validation#id",
				ID:  resources.UnknownReferenceID,
			},
		},
	}
	plan.AddChange(change)

	exec.hydrateKnownReferenceIDs(&change, plan)

	ref := change.References[planner.FieldParentPolicyID]
	assert.Equal(t, "schema-validation-id", ref.ID)
	assert.Equal(t, "schema-validation-id", change.Fields[planner.FieldParentPolicyID])
}

func TestSyncResolvedEventGatewayStaticKeyConfigRefListsStaticKeysOnce(t *testing.T) {
	api := &stubEventGatewayStaticKeyAPI{
		keys: []kkComps.EventGatewayStaticKey{
			{Name: "key-one", ID: "key-one-id"},
			{Name: "key-two", ID: "key-two-id"},
		},
	}
	exec := New(state.NewClient(state.ClientConfig{EventGatewayStaticKeyAPI: api}), nil, false)
	change := planner.PlannedChange{
		Fields: map[string]any{
			planner.FieldConfig: map[string]any{
				"encrypt_fields": []any{
					map[string]any{
						"encryption_key": map[string]any{
							"key": map[string]any{planner.FieldID: "__REF__:key-one#id"},
						},
					},
					map[string]any{
						"encryption_key": map[string]any{
							"key": map[string]any{planner.FieldID: "__REF__:key-two#id"},
						},
					},
				},
			},
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldEventGatewayID: {ID: "gateway-id"},
			"config.encrypt_fields.0.encryption_key.key.id": {
				Ref:          "__REF__:key-one#id",
				ID:           resources.UnknownReferenceID,
				LookupFields: map[string]string{planner.FieldName: "key-one"},
			},
			"config.encrypt_fields.1.encryption_key.key.id": {
				Ref:          "__REF__:key-two#id",
				ID:           resources.UnknownReferenceID,
				LookupFields: map[string]string{planner.FieldName: "key-two"},
			},
		},
	}

	err := exec.syncResolvedEventGatewayStaticKeyConfigRef(context.Background(), &change)

	require.NoError(t, err)
	assert.Equal(t, 1, api.listCalls)
	assert.Equal(t, "key-one-id", change.References["config.encrypt_fields.0.encryption_key.key.id"].ID)
	assert.Equal(t, "key-two-id", change.References["config.encrypt_fields.1.encryption_key.key.id"].ID)

	config := change.Fields[planner.FieldConfig].(map[string]any)
	encryptFields := config["encrypt_fields"].([]any)
	firstKey := encryptFields[0].(map[string]any)["encryption_key"].(map[string]any)["key"].(map[string]any)
	secondKey := encryptFields[1].(map[string]any)["encryption_key"].(map[string]any)["key"].(map[string]any)
	assert.Equal(t, "key-one-id", firstKey[planner.FieldID])
	assert.Equal(t, "key-two-id", secondKey[planner.FieldID])
}

func TestHydrateKnownReferenceIDsAppliesResolvedNestedField(t *testing.T) {
	exec := New(nil, nil, false)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	change := planner.PlannedChange{
		ID:           "1-c-produce-policy",
		ResourceType: planner.ResourceTypeEventGatewayProducePolicy,
		ResourceRef:  "produce-policy",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			planner.FieldConfig: map[string]any{
				"schema_registry": map[string]any{
					planner.FieldID: "__REF__:schema-registry#id",
				},
			},
		},
		References: map[string]planner.ReferenceInfo{
			"config.schema_registry.id": {
				Ref: "__REF__:schema-registry#id",
				ID:  "schema-registry-id",
			},
		},
	}

	exec.hydrateKnownReferenceIDs(&change, plan)

	config := change.Fields[planner.FieldConfig].(map[string]any)
	schemaRegistry := config["schema_registry"].(map[string]any)
	assert.Equal(t, "schema-registry-id", schemaRegistry[planner.FieldID])
	assert.NotContains(t, change.Fields, "config.schema_registry.id")
}

type stubEventGatewayStaticKeyAPI struct {
	keys      []kkComps.EventGatewayStaticKey
	listCalls int
}

var _ helpers.EventGatewayStaticKeyAPI = (*stubEventGatewayStaticKeyAPI)(nil)

func (s *stubEventGatewayStaticKeyAPI) ListEventGatewayStaticKeys(
	context.Context,
	kkOps.ListEventGatewayStaticKeysRequest,
	...kkOps.Option,
) (*kkOps.ListEventGatewayStaticKeysResponse, error) {
	s.listCalls++
	return &kkOps.ListEventGatewayStaticKeysResponse{
		ListEventGatewayStaticKeysResponse: &kkComps.ListEventGatewayStaticKeysResponse{
			Data: s.keys,
		},
	}, nil
}

func (s *stubEventGatewayStaticKeyAPI) GetEventGatewayStaticKey(
	context.Context,
	kkOps.GetEventGatewayStaticKeyRequest,
	...kkOps.Option,
) (*kkOps.GetEventGatewayStaticKeyResponse, error) {
	return nil, fmt.Errorf("GetEventGatewayStaticKey not implemented")
}

func (s *stubEventGatewayStaticKeyAPI) CreateEventGatewayStaticKey(
	context.Context,
	kkOps.CreateEventGatewayStaticKeyRequest,
	...kkOps.Option,
) (*kkOps.CreateEventGatewayStaticKeyResponse, error) {
	return nil, fmt.Errorf("CreateEventGatewayStaticKey not implemented")
}

func (s *stubEventGatewayStaticKeyAPI) DeleteEventGatewayStaticKey(
	context.Context,
	kkOps.DeleteEventGatewayStaticKeyRequest,
	...kkOps.Option,
) (*kkOps.DeleteEventGatewayStaticKeyResponse, error) {
	return nil, fmt.Errorf("DeleteEventGatewayStaticKey not implemented")
}

func TestExecutor_Execute_EmptyPlan(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, false)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Equal(t, 0, result.SkippedCount)
	assert.Empty(t, result.Errors)
	assert.False(t, result.DryRun)

	// Verify reporter was called
	assert.Equal(t, 1, reporter.StartExecutionCalled)
	assert.Equal(t, 1, reporter.FinishExecutionCalled)
}

func TestExecutor_Execute_DryRun(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("SkipChange", mock.Anything, "dry-run mode").Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, true) // dry-run enabled

	// Create a plan with a CREATE change
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	change := planner.PlannedChange{
		ID:           "1-c-portal",
		ResourceType: "portal",
		ResourceRef:  "dev-portal",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			"name":        "Developer Portal",
			"description": "Main developer portal",
		},
	}
	plan.AddChange(change)
	plan.SetExecutionOrder([]string{"1-c-portal"})

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Equal(t, 1, result.SkippedCount)
	assert.Empty(t, result.Errors)
	assert.True(t, result.DryRun)

	// Check validation results
	require.Len(t, result.ValidationResults, 1)
	assert.Equal(t, "1-c-portal", result.ValidationResults[0].ChangeID)
	assert.Equal(t, "would_succeed", result.ValidationResults[0].Status)
	assert.Equal(t, "passed", result.ValidationResults[0].Validation)

	// Verify reporter was called correctly
	assert.Equal(t, 1, len(reporter.SkipChangeCalls))
	assert.Equal(t, "dry-run mode", reporter.SkipReasons[0])
}

func TestExecutor_Execute_WithErrors(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("CompleteChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, false)

	// Create a plan with a CREATE change for an unimplemented resource type
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	change := planner.PlannedChange{
		ID:           "1-c-service",
		ResourceType: "service", // Not yet implemented
		ResourceRef:  "test-service",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			"name": "Test Service",
		},
	}
	plan.AddChange(change)
	plan.SetExecutionOrder([]string{"1-c-service"})

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
	assert.Equal(t, 0, result.SkippedCount)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "not yet implemented")

	// Verify error details
	assert.Equal(t, "1-c-service", result.Errors[0].ChangeID)
	assert.Equal(t, "service", result.Errors[0].ResourceType)
	assert.Equal(t, "Test Service", result.Errors[0].ResourceName)
}

func TestExecutor_Execute_NilReporter(t *testing.T) {
	// Execute with nil reporter should not panic
	exec := New(nil, nil, false)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
}

func TestExecutor_resolveAuthStrategyRef_WithPlaceholder(t *testing.T) {
	exec := New(nil, nil, false)
	exec.refToID["application_auth_strategy"] = map[string]string{
		"key-auth": "abc-123",
	}

	id, err := exec.resolveAuthStrategyRef(context.Background(), planner.ReferenceInfo{Ref: "__REF__:key-auth#id"})
	require.NoError(t, err)
	assert.Equal(t, "abc-123", id)
}

func TestExecutor_syncResolvedDCRProviderID_UpdatesFieldsFromResolvedReference(t *testing.T) {
	exec := New(nil, nil, false)
	exec.refToID["dcr_provider"] = map[string]string{
		"okta-dcr": "6f211020-9ffb-4f64-b351-9ca7282fe451",
	}

	change := &planner.PlannedChange{
		Fields: map[string]any{
			planner.FieldDCRProviderID: "__REF__:okta-dcr#id",
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldDCRProviderID: {
				Ref: "__REF__:okta-dcr#id",
				ID:  "[unknown]",
			},
		},
	}

	err := exec.syncResolvedDCRProviderID(context.Background(), change)
	require.NoError(t, err)

	assert.Equal(
		t,
		"6f211020-9ffb-4f64-b351-9ca7282fe451",
		change.Fields[planner.FieldDCRProviderID],
	)
	assert.Equal(
		t,
		"6f211020-9ffb-4f64-b351-9ca7282fe451",
		change.References[planner.FieldDCRProviderID].ID,
	)
}

func TestExecutor_syncResolvedPortalDefaultAuthStrategyID_UpdatesFieldsFromResolvedReference(t *testing.T) {
	exec := New(nil, nil, false)
	exec.refToID[planner.ResourceTypeApplicationAuthStrategy] = map[string]string{
		"portal-default-strategy": "11111111-1111-4111-8111-111111111111",
	}

	change := &planner.PlannedChange{
		Fields: map[string]any{
			planner.FieldDefaultApplicationStrategyID: "__REF__:portal-default-strategy#id",
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldDefaultApplicationStrategyID: {
				Ref: "__REF__:portal-default-strategy#id",
				ID:  "[unknown]",
			},
		},
	}

	err := exec.syncResolvedPortalDefaultAuthStrategyID(context.Background(), change)
	require.NoError(t, err)

	assert.Equal(
		t,
		"11111111-1111-4111-8111-111111111111",
		change.Fields[planner.FieldDefaultApplicationStrategyID],
	)
	assert.Equal(
		t,
		"11111111-1111-4111-8111-111111111111",
		change.References[planner.FieldDefaultApplicationStrategyID].ID,
	)
}

func TestExecutor_hydrateKnownReferenceIDs_PopulatesScalarRefAndParent(t *testing.T) {
	exec := New(nil, nil, false)
	exec.createdResources["1:c:api:my-api"] = "api-id-123"

	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:          "1:c:api:my-api",
				Action:      planner.ActionCreate,
				ResourceRef: "my-api",
			},
			{
				ID:          "2:c:api_version:v1",
				Action:      planner.ActionCreate,
				ResourceRef: "my-api-v1",
				DependsOn:   []string{"1:c:api:my-api"},
				Fields: map[string]any{
					planner.FieldAPIID: "__REF__:my-api#id",
				},
				References: map[string]planner.ReferenceInfo{
					planner.FieldAPIID: {
						Ref: "__REF__:my-api#id",
						ID:  "[unknown]",
					},
				},
				Parent: &planner.ParentInfo{
					Ref: "my-api",
					ID:  "[unknown]",
				},
			},
		},
	}

	change := &plan.Changes[1]
	exec.hydrateKnownReferenceIDs(change, plan)

	assert.Equal(t, "api-id-123", change.References[planner.FieldAPIID].ID)
	assert.Equal(t, "api-id-123", change.Fields[planner.FieldAPIID])
	require.NotNil(t, change.Parent)
	assert.Equal(t, "api-id-123", change.Parent.ID)
}

func TestExecutor_hydrateKnownReferenceIDs_PopulatesArrayResolvedIDs(t *testing.T) {
	exec := New(nil, nil, false)
	exec.createdResources["1:c:cp:member-a"] = "cp-id-a"

	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:          "1:c:cp:member-a",
				Action:      planner.ActionCreate,
				ResourceRef: "member-a",
			},
			{
				ID:          "2:u:cp_group:group-1",
				Action:      planner.ActionUpdate,
				ResourceRef: "group-1",
				DependsOn:   []string{"1:c:cp:member-a"},
				References: map[string]planner.ReferenceInfo{
					planner.FieldMembers: {
						IsArray: true,
						Refs:    []string{"__REF__:member-a#id", "literal-id"},
					},
				},
			},
		},
	}

	change := &plan.Changes[1]
	exec.hydrateKnownReferenceIDs(change, plan)

	refInfo := change.References[planner.FieldMembers]
	require.Len(t, refInfo.ResolvedIDs, 2)
	assert.Equal(t, "cp-id-a", refInfo.ResolvedIDs[0])
	assert.Equal(t, "", refInfo.ResolvedIDs[1])
}

func TestExecutor_ValidateChangePreExecution_Basic(t *testing.T) {
	tests := []struct {
		name          string
		change        planner.PlannedChange
		expectError   bool
		errorContains string
	}{
		{
			name: "create action - no validation",
			change: planner.PlannedChange{
				Action:       planner.ActionCreate,
				ResourceType: "portal",
			},
			expectError: false,
		},
		{
			name: "update without resource ID",
			change: planner.PlannedChange{
				Action:       planner.ActionUpdate,
				ResourceType: "portal",
			},
			expectError:   true,
			errorContains: "resource ID required",
		},
		{
			name: "delete without resource ID",
			change: planner.PlannedChange{
				Action:       planner.ActionDelete,
				ResourceType: "portal",
			},
			expectError:   true,
			errorContains: "resource ID required",
		},
		{
			name: "update non-portal resource - TODO",
			change: planner.PlannedChange{
				Action:       planner.ActionUpdate,
				ResourceType: "auth_strategy",
				ResourceID:   "auth-123",
			},
			expectError: false, // No validation for non-portal resources yet
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := New(nil, nil, false)

			err := exec.validateChangePreExecution(context.Background(), tt.change)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExecutionResult_Methods(t *testing.T) {
	// Test Message() method
	t.Run("dry-run messages", func(t *testing.T) {
		result := &ExecutionResult{DryRun: true}
		assert.Equal(t, "Dry-run complete. No changes were made.", result.Message())

		result.FailureCount = 1
		assert.Equal(t, "Dry-run complete with errors. No changes were made.", result.Message())
	})

	t.Run("real execution messages", func(t *testing.T) {
		result := &ExecutionResult{DryRun: false}
		assert.Equal(t, "Execution completed successfully.", result.Message())

		result.FailureCount = 1
		assert.Equal(t, "Execution completed with errors.", result.Message())
	})

	// Test HasErrors() method
	t.Run("has errors", func(t *testing.T) {
		result := &ExecutionResult{}
		assert.False(t, result.HasErrors())

		result.FailureCount = 1
		assert.True(t, result.HasErrors())

		result.FailureCount = 0
		result.Errors = []ExecutionError{{Error: "test"}}
		assert.True(t, result.HasErrors())
	})

	// Test TotalChanges() method
	t.Run("total changes", func(t *testing.T) {
		result := &ExecutionResult{
			SuccessCount: 2,
			FailureCount: 1,
			SkippedCount: 3,
		}
		assert.Equal(t, 6, result.TotalChanges())
	})
}

func TestExecutor_ExecutionOrder(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Track the order of StartChange calls
	var executionOrder []string
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Run(func(args mock.Arguments) {
		change := args.Get(0).(planner.PlannedChange)
		executionOrder = append(executionOrder, change.ID)
	}).Return()
	reporter.On("SkipChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, true) // dry-run

	// Create a plan with multiple changes
	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)

	// Add changes in one order
	changes := []planner.PlannedChange{
		{
			ID:           "3-d-portal",
			Action:       planner.ActionDelete,
			ResourceType: "portal",
			ResourceID:   "portal-3", // Required for DELETE
			Fields:       map[string]any{"name": "Portal 3-d-portal"},
		},
		{
			ID:           "1-c-portal",
			Action:       planner.ActionCreate,
			ResourceType: "portal",
			Fields:       map[string]any{"name": "Portal 1-c-portal"},
		},
		{
			ID:           "2-u-portal",
			Action:       planner.ActionUpdate,
			ResourceType: "portal",
			ResourceID:   "portal-2", // Required for UPDATE
			Fields:       map[string]any{"name": "Portal 2-u-portal"},
		},
	}

	for _, change := range changes {
		plan.AddChange(change)
	}

	// Set specific execution order
	plan.SetExecutionOrder([]string{"1-c-portal", "2-u-portal", "3-d-portal"})

	_ = exec.Execute(context.Background(), plan)

	// Verify execution followed the specified order
	assert.Equal(t, []string{"1-c-portal", "2-u-portal", "3-d-portal"}, executionOrder)
}

func TestExecutor_ContinuesOnError(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("CompleteChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, false)

	// Create a plan with multiple changes (all will fail due to not implemented)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)

	for i := 1; i <= 3; i++ {
		change := planner.PlannedChange{
			ID:           fmt.Sprintf("%d-c-route", i),
			ResourceType: "route", // Not yet implemented
			ResourceRef:  fmt.Sprintf("route-%d", i),
			Action:       planner.ActionCreate,
			Fields: map[string]any{
				"name": fmt.Sprintf("Route %d", i),
			},
		}
		plan.AddChange(change)
	}
	plan.SetExecutionOrder([]string{"1-c-route", "2-c-route", "3-c-route"})

	result := exec.Execute(context.Background(), plan)

	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 3, result.FailureCount) // All failed
	assert.Equal(t, 0, result.SkippedCount)
	assert.Len(t, result.Errors, 3) // All errors collected

	// Verify all changes were attempted
	assert.Len(t, reporter.CompleteChangeCalls, 3)
}

func TestExecutor_ExecuteGroupsConcurrent_DryRun_IsConcurrencySafe(t *testing.T) {
	exec := NewWithOptions(nil, nil, true, Options{
		MaxConcurrency: 32,
	})

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

	const n = 200
	group := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("%d-c-portal", i)
		group = append(group, id)

		plan.AddChange(planner.PlannedChange{
			ID:           id,
			ResourceType: "portal",
			ResourceRef:  fmt.Sprintf("portal-%d", i),
			Action:       planner.ActionCreate,
			Fields: map[string]any{
				"name": fmt.Sprintf("Portal %d", i),
			},
		})
	}

	plan.SetExecutionGroups([][]string{group})

	result := exec.Execute(context.Background(), plan)

	require.NotNil(t, result)
	assert.Equal(t, 0, result.FailureCount)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, n, result.SkippedCount)
	assert.Len(t, result.ValidationResults, n)
	assert.Empty(t, result.Errors)
}

func TestExecutor_ExecuteGroupsConcurrent_BlocksDependentsInNextGroups(t *testing.T) {
	reporter := &MockProgressReporter{}
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("SkipChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := NewWithOptions(nil, reporter, true, Options{
		MaxConcurrency: 5,
	})

	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)

	missingChangeID := "1-c-missing"
	dependentID := "2-c-portal-dependent"
	independentID := "3-c-portal-independent"

	plan.AddChange(planner.PlannedChange{
		ID:           dependentID,
		ResourceType: "portal",
		ResourceRef:  "portal-dependent",
		Action:       planner.ActionCreate,
		DependsOn:    []string{missingChangeID},
		Fields: map[string]any{
			"name": "Portal Dependent",
		},
	})

	plan.AddChange(planner.PlannedChange{
		ID:           independentID,
		ResourceType: "portal",
		ResourceRef:  "portal-independent",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			"name": "Portal Independent",
		},
	})

	plan.SetExecutionGroups([][]string{
		{missingChangeID},
		{dependentID, independentID},
	})

	result := exec.Execute(context.Background(), plan)

	require.NotNil(t, result)
	assert.Equal(t, 1, result.FailureCount)
	assert.Equal(t, 2, result.SkippedCount)
	assert.Equal(t, 0, result.SuccessCount)

	require.Len(t, result.Errors, 1)
	assert.Equal(t, missingChangeID, result.Errors[0].ChangeID)

	// Proves dependent was blocked/skipped, while non-dependent in same group executed.
	assert.Len(t, reporter.StartChangeCalls, 1)
	started := map[string]bool{}
	for _, change := range reporter.StartChangeCalls {
		started[change.ID] = true
	}
	assert.True(t, started[independentID])
	assert.False(t, started[dependentID])

	assert.Len(t, reporter.SkipChangeCalls, 2)

	skipReasonByID := map[string]string{}
	for i, change := range reporter.SkipChangeCalls {
		skipReasonByID[change.ID] = reporter.SkipReasons[i]
	}

	assert.Contains(t, skipReasonByID[dependentID], "blocked by failed dependencies")
	assert.Contains(t, skipReasonByID[dependentID], missingChangeID)
	assert.Equal(t, "dry-run mode", skipReasonByID[independentID])
}
