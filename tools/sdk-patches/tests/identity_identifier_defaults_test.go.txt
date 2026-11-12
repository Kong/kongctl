package sdkkonnectgo_test

import (
	"encoding/json"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	o "github.com/Kong/sdk-konnect-go/models/operations"
)

func TestKongctlTrustedIDPIdentifierWirePresence(t *testing.T) {
	for _, identifier := range []*string{nil, new("sub"), new("oid")} {
		for _, verb := range []string{"create", "replace"} {
			capture := &realmsCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			mapping := &c.TrustedIdpJitMapping{Email: new("email"), Identifier: identifier}
			// Also exercise decoding, which must not introduce a default.
			encoded, err := json.Marshal(mapping)
			if err != nil { t.Fatal(err) }
			if err := json.Unmarshal(encoded, &mapping); err != nil { t.Fatal(err) }
			jit := &c.TrustedIdpJit{Mapping: mapping}
			if verb == "create" {
				_, _ = client.TrustedIDPs.CreateTrustedIdp(t.Context(), "directory", c.CreateTrustedIdpPayload{
					IssuerURL: "https://issuer.example.test", Jit: jit,
				})
			} else {
				_, _ = client.TrustedIDPs.UpdateTrustedIdp(t.Context(), o.UpdateTrustedIdpRequest{
					DirectoryID: "directory", TrustedIdpID: "trust",
					ReplaceTrustedIdpBody: c.ReplaceTrustedIdpBody{IssuerURL: "https://issuer.example.test", Jit: jit},
				})
			}
			var body struct { Jit struct { Mapping map[string]any `json:"mapping"` } `json:"jit"` }
			if err := json.Unmarshal(capture.body, &body); err != nil { t.Fatal(err) }
			value, present := body.Jit.Mapping["identifier"]
			if identifier == nil {
				if present { t.Fatalf("%s injected identifier: %s", verb, capture.body) }
			} else if !present || value != *identifier {
				t.Fatalf("%s identifier = %v, want %s", verb, value, *identifier)
			}
		}
	}
}
