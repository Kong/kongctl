package resources

import (
	"testing"
)

func TestExtractNameAndID(t *testing.T) {
	tests := []struct {
		name         string
		konnectRes   interface{}
		embeddedName string
		wantName     string
		wantID       string
	}{
		{
			name: "direct fields",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "test-service", ID: "uuid-123"},
			embeddedName: "",
			wantName:     "test-service",
			wantID:       "uuid-123",
		},
		{
			name: "embedded struct",
			konnectRes: struct {
				APIResponseSchema struct {
					Name string
					ID   string
				}
			}{APIResponseSchema: struct {
				Name string
				ID   string
			}{Name: "my-api", ID: "api-456"}},
			embeddedName: "APIResponseSchema",
			wantName:     "my-api",
			wantID:       "api-456",
		},
		{
			name: "pointer fields",
			konnectRes: func() interface{} {
				name := "ptr-name"
				id := "ptr-id"
				return struct {
					Name *string
					ID   *string
				}{Name: &name, ID: &id}
			}(),
			embeddedName: "",
			wantName:     "ptr-name",
			wantID:       "ptr-id",
		},
		{
			name:         "empty struct",
			konnectRes:   struct{}{},
			embeddedName: "",
			wantName:     "",
			wantID:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, id := extractNameAndID(tt.konnectRes, tt.embeddedName)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
		})
	}
}
