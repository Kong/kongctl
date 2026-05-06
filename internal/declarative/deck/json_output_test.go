package deck

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeMaskedJSONOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		changed bool
	}{
		{
			name:    "valid JSON without masked value",
			input:   `{"summary":{"total":1}}`,
			want:    `{"summary":{"total":1}}`,
			changed: false,
		},
		{
			name:    "quoted masked string unchanged",
			input:   `{"name":"[masked]"}`,
			want:    `{"name":"[masked]"}`,
			changed: false,
		},
		{
			name:    "bare masked object value quoted",
			input:   `{"config":{"dimensions":[masked]}}`,
			want:    `{"config":{"dimensions":"[masked]"}}`,
			changed: true,
		},
		{
			name:    "multiple bare masked values quoted",
			input:   `{"input_cost":[masked],"output_cost":[masked]}`,
			want:    `{"input_cost":"[masked]","output_cost":"[masked]"}`,
			changed: true,
		},
		{
			name:    "masked text inside string unchanged",
			input:   `{"description":"literal [masked] value"}`,
			want:    `{"description":"literal [masked] value"}`,
			changed: false,
		},
		{
			name:    "masked text inside escaped string unchanged",
			input:   `{"description":"quoted \"[masked]\" value","cost":[masked]}`,
			want:    `{"description":"quoted \"[masked]\" value","cost":"[masked]"}`,
			changed: true,
		},
		{
			name:    "bare masked array value quoted",
			input:   `{"values":[[masked],1]}`,
			want:    `{"values":["[masked]",1]}`,
			changed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := NormalizeMaskedJSONOutput(tt.input)
			require.Equal(t, tt.changed, changed)
			require.Equal(t, tt.want, got)
			require.NoError(t, json.Unmarshal([]byte(got), new(any)))
		})
	}
}
