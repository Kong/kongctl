package tags

import (
	"strings"
	"testing"
)

func TestResolverRegistry_UnknownTag(t *testing.T) {
	registry := NewResolverRegistry()
	// Register supported custom resolvers
	registry.Register(NewFileTagResolver("/tmp", "/tmp"))
	registry.Register(NewEnvTagResolver(EnvTagModePlaceholder))

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "unknown tag !File",
			yaml:    `value: !File test.txt`,
			wantErr: "unsupported YAML tag: !File",
		},
		{
			name:    "known tag !file",
			yaml:    `value: !file ./test.txt`,
			wantErr: "file not found", // Will fail but proves tag is recognized
		},
		{
			name:    "known tag !env",
			yaml:    `value: !env HOME`,
			wantErr: "",
		},
		{
			name:    "built-in tag !!str",
			yaml:    `value: !!str "test"`,
			wantErr: "", // Should not error on built-in tags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.Process([]byte(tt.yaml))
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
