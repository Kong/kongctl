package components

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPortalAISettingsPatchPreservesOmissionAndExplicitValues(t *testing.T) {
	for _, input := range []string{
		`{"enabled":false}`,
		`{"enabled":true}`,
		`{"enabled":true,"features":{"mcp_server":{"enabled":true}}}`,
		`{"enabled":true,"features":{"mcp_server":{"enabled":true,"write_operations_enabled":false}}}`,
		`{"enabled":true,"features":{"mcp_server":{"enabled":false,"write_operations_enabled":true}}}`,
	} {
		t.Run(input, func(t *testing.T) {
			var request PatchAISettingsRequest
			if err := json.Unmarshal([]byte(input), &request); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal([]byte(input), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("PATCH payload = %s, want %s", data, input)
			}
		})
	}
}
