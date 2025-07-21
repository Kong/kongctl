package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRef(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		errMsg  string
	}{
		// Valid refs
		{
			name:    "valid simple ref",
			ref:     "my-resource",
			wantErr: false,
		},
		{
			name:    "valid ref with underscore",
			ref:     "my_resource",
			wantErr: false,
		},
		{
			name:    "valid ref with numbers",
			ref:     "resource123",
			wantErr: false,
		},
		{
			name:    "valid ref starting with number",
			ref:     "123resource",
			wantErr: false,
		},
		{
			name:    "valid single character",
			ref:     "a",
			wantErr: false,
		},
		{
			name:    "valid max length",
			ref:     "a12345678901234567890123456789012345678901234567890123456789012", // 63 chars
			wantErr: false,
		},

		// Invalid refs
		{
			name:    "empty ref",
			ref:     "",
			wantErr: true,
			errMsg:  "ref cannot be empty",
		},
		{
			name:    "ref with colon",
			ref:     "my:resource",
			wantErr: true,
			errMsg:  "ref cannot contain colons (:)",
		},
		{
			name:    "ref with multiple colons",
			ref:     "my:resource:name",
			wantErr: true,
			errMsg:  "ref cannot contain colons (:)",
		},
		{
			name:    "ref with space",
			ref:     "my resource",
			wantErr: true,
			errMsg:  "ref cannot contain spaces",
		},
		{
			name:    "ref starting with hyphen",
			ref:     "-myresource",
			wantErr: true,
			errMsg:  "ref must start with a letter or number",
		},
		{
			name:    "ref starting with underscore",
			ref:     "_myresource",
			wantErr: true,
			errMsg:  "ref must start with a letter or number",
		},
		{
			name:    "ref with special characters",
			ref:     "my@resource",
			wantErr: true,
			errMsg:  "ref must start with a letter or number",
		},
		{
			name:    "ref with slash",
			ref:     "my/resource",
			wantErr: true,
			errMsg:  "ref must start with a letter or number",
		},
		{
			name:    "ref with dot",
			ref:     "my.resource",
			wantErr: true,
			errMsg:  "ref must start with a letter or number",
		},
		{
			name:    "ref too long",
			ref:     "a1234567890123456789012345678901234567890123456789012345678901234", // 64 chars
			wantErr: true,
			errMsg:  "ref must be between 1 and 63 characters long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRef(tt.ref)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}