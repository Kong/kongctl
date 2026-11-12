package sdkkonnectgo_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	o "github.com/Kong/sdk-konnect-go/models/operations"
)

type oidcConfigCapture struct{ body []byte }

func (r *oidcConfigCapture) Do(req *http.Request) (*http.Response, error) {
	var err error
	r.body, err = io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: 400,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"probe"}`)),
		Request:    req,
	}, nil
}

func TestKongctlOIDCConfigWirePresence(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		present      bool
		header       any
		count        int
	}{
		{"omitted", `{"issuer":"https://example.com","cache_tokens_salt":"probe-salt","auth_methods":["bearer"],"hide_credentials":true}`, false, nil, 4},
		{"null", `{"issuer":"https://example.com","cache_tokens_salt":"probe-salt","upstream_access_token_header":null}`, true, nil, 3},
		{"explicit-default", `{"issuer":"https://example.com","cache_tokens_salt":"probe-salt","upstream_access_token_header":"authorization:bearer"}`, true, "authorization:bearer", 3},
		{"custom", `{"issuer":"https://example.com","cache_tokens_salt":"probe-salt","upstream_access_token_header":"x-custom-token","hide_credentials":false,"scopes":[]}`, true, "x-custom-token", 5},
	} {
		for _, verb := range []string{"create", "update", "response"} {
			t.Run(tc.name+"/"+verb, func(t *testing.T) {
				var fields map[string]any
				raw := `{"name":"probe","display_name":"Probe","type":"openid-connect","config":` + tc.config + `}`
				if err := json.Unmarshal([]byte(raw), &fields); err != nil {
					t.Fatal(err)
				}
				// Preserve field presence through JSON decoding and SDK request serialization.
				encoded, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				capture := &oidcConfigCapture{}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))

				if verb == "create" {
					var request c.CreateAIGatewayAuthStrategyRequest
					if err = json.Unmarshal(encoded, &request); err != nil {
						t.Fatal(err)
					}
					_, _ = client.AIGatewayAuthStrategies.CreateAiGatewayAuthStrategy(t.Context(), "gateway", request)
				} else if verb == "update" {
					var request c.UpdateAIGatewayAuthStrategyRequest
					if err = json.Unmarshal(encoded, &request); err != nil {
						t.Fatal(err)
					}
					_, _ = client.AIGatewayAuthStrategies.UpdateAiGatewayAuthStrategy(t.Context(), o.UpdateAiGatewayAuthStrategyRequest{GatewayID: "gateway", AuthStrategyIDOrName: "probe", UpdateAIGatewayAuthStrategyRequest: request})
				}
				if verb == "response" {
					raw = strings.TrimSuffix(raw, "}") + `,"id":"00000000-0000-0000-0000-000000000001","created_at":"2026-11-11T00:00:00Z","updated_at":"2026-11-11T00:00:00Z"}`
					var response c.AIGatewayAuthStrategy
					if err := json.Unmarshal([]byte(raw), &response); err != nil {
						t.Fatal(err)
					}
					capture.body, err = json.Marshal(response)
					if err != nil {
						t.Fatal(err)
					}
				}

				if capture.body == nil {
					t.Fatal("SDK sent no request")
				}
				var body map[string]any
				if err = json.Unmarshal(capture.body, &body); err != nil {
					t.Fatal(err)
				}
				config := body["config"].(map[string]any)
				if len(config) != tc.count {
					t.Fatalf("wire config has %d fields, want %d", len(config), tc.count)
				}
				val, present := config["upstream_access_token_header"]
				if present != tc.present || val != tc.header {
					t.Fatalf("wire header=(%#v, %v), want (%#v, %v)", val, present, tc.header, tc.present)
				}
				if tc.name == "custom" {
					if config["hide_credentials"] != false {
						t.Fatal("lost explicit false")
					}
					scopes, ok := config["scopes"].([]any)
					if !ok || len(scopes) != 0 {
						t.Fatal("lost explicit empty array")
					}
				}
			})
		}
	}
}
