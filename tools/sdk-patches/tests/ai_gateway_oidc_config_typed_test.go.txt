//go:build kongctl_sdk_patched

package sdkkonnectgo_test

import (
	"encoding/json"
	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/optionalnullable"
	"testing"
)

func TestKongctlOIDCConfigTypedConstruction(t *testing.T) {
	for _, state := range []string{"omitted", "null", "value"} {
		t.Run(state, func(t *testing.T) {
			config := c.AIGWOpenIDConnectGeneratedConfig{
				Issuer: "https://example.com", CacheTokensSalt: "probe-salt",
				HideCredentials: optionalnullable.From(new(false)),
				AuthMethods:     optionalnullable.From(new([]c.AuthMethods{c.AuthMethodsBearer})),
				Scopes:          optionalnullable.From(new([]string{})),
			}
			if state == "null" {
				config.UpstreamAccessTokenHeader = optionalnullable.From[string](nil)
			}
			if state == "value" {
				config.UpstreamAccessTokenHeader = optionalnullable.From(new("authorization:bearer"))
			}
			// Existing typed getters remain usable without map lookup/type assertions.
			var methods []c.AuthMethods = config.GetAuthMethods()
			if len(methods) != 1 || methods[0] != c.AuthMethodsBearer {
				t.Fatal("typed enum getter changed")
			}
			if config.GetHideCredentials() == nil || *config.GetHideCredentials() {
				t.Fatal("lost typed false")
			}
			capture := &oidcConfigCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			request := c.CreateCreateAIGatewayAuthStrategyRequestOpenidConnect(c.AIGatewayAuthStrategyOpenIDConnect{
				Name: "probe", DisplayName: "Probe", Config: &config,
			})
			_, _ = client.AIGatewayAuthStrategies.CreateAiGatewayAuthStrategy(t.Context(), "gateway", request)
			var body struct {
				Config map[string]any `json:"config"`
			}
			if err := json.Unmarshal(capture.body, &body); err != nil {
				t.Fatal(err)
			}
			value, present := body.Config["upstream_access_token_header"]
			if present != (state != "omitted") {
				t.Fatalf("wrong %s presence", state)
			}
			if state == "null" && value != nil {
				t.Fatal("lost explicit null")
			}
			if state == "value" && value != "authorization:bearer" {
				t.Fatal("lost explicit default")
			}
			if body.Config["hide_credentials"] != false {
				t.Fatal("lost false")
			}
			scopes, ok := body.Config["scopes"].([]any)
			if !ok || len(scopes) != 0 {
				t.Fatal("lost empty array")
			}
			expected := 5
			if present {
				expected++
			}
			if len(body.Config) != expected {
				t.Fatalf("got %d config fields, want %d", len(body.Config), expected)
			}
		})
	}
}
