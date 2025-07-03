package resources

import (
	"testing"
)

func TestAPIResourceInterface(t *testing.T) {
	api := &APIResource{
		Ref:  "test-api",
		Name: "Test API",
	}

	// Test Resource interface methods
	if got := api.GetKind(); got != "api" {
		t.Errorf("GetKind() = %v, want %v", got, "api")
	}

	if got := api.GetRef(); got != "test-api" {
		t.Errorf("GetRef() = %v, want %v", got, "test-api")
	}

	if got := api.GetName(); got != "Test API" {
		t.Errorf("GetName() = %v, want %v", got, "Test API")
	}

	// GetDependencies should return empty for APIs
	if deps := api.GetDependencies(); len(deps) != 0 {
		t.Errorf("GetDependencies() = %v, want empty", deps)
	}
}

func TestAPIResourceLabels(t *testing.T) {
	api := &APIResource{
		Ref: "test-api",
	}

	// Test setting and getting labels
	labels := map[string]string{
		"env":   "production",
		"team":  "platform",
		"owner": "api-team",
	}
	api.SetLabels(labels)

	// Get labels back
	gotLabels := api.GetLabels()
	if len(gotLabels) != len(labels) {
		t.Errorf("GetLabels() returned %d labels, want %d", len(gotLabels), len(labels))
	}

	for k, v := range labels {
		if gotLabels[k] != v {
			t.Errorf("GetLabels()[%q] = %v, want %v", k, gotLabels[k], v)
		}
	}

	// Test nil labels
	api.SetLabels(nil)
	if gotLabels := api.GetLabels(); gotLabels != nil {
		t.Errorf("GetLabels() = %v, want nil after setting nil", gotLabels)
	}

	// Test getting labels when not set
	api2 := &APIResource{Ref: "test-api-2"}
	if gotLabels := api2.GetLabels(); gotLabels != nil {
		t.Errorf("GetLabels() = %v, want nil for unset labels", gotLabels)
	}
}

func TestAPIResourceSetDefaults(t *testing.T) {
	tests := []struct {
		name         string
		api          APIResource
		expectedName string
	}{
		{
			name: "name from ref when name is empty",
			api: APIResource{
				Ref: "my-api",
			},
			expectedName: "my-api",
		},
		{
			name: "existing name is preserved",
			api: APIResource{
				Ref:  "my-api",
				Name: "Existing API Name",
			},
			expectedName: "Existing API Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := tt.api
			api.SetDefaults()
			if api.Name != tt.expectedName {
				t.Errorf("SetDefaults() Name = %v, want %v", api.Name, tt.expectedName)
			}
		})
	}
}