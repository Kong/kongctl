package sdkkonnectgo_test

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
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

// Baseline results were recorded with the unmodified v0.84.0 OIDC models.
// Only upstream_access_token_header is excluded from these comparisons.
//go:embed kongctl_oidc_config_baseline.json
var oidcBaseline []byte

type oidcCompatibilityCase struct {
    Name string `json:"name"`
    Config map[string]any `json:"config"`
    Expected map[string]map[string]any `json:"expected"`
}

func oidcWireConfig(t *testing.T, config map[string]any, verb string) map[string]any {
    t.Helper()
    raw, err := json.Marshal(map[string]any{
        "name":"probe", "display_name":"Probe", "type":"openid-connect", "config":config,
    })
    if err != nil { t.Fatal(err) }
    capture := &oidcConfigCapture{}
    client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
    switch verb {
    case "create":
        var request c.CreateAIGatewayAuthStrategyRequest
        if err = json.Unmarshal(raw, &request); err != nil { t.Fatal(err) }
        _, _ = client.AIGatewayAuthStrategies.CreateAiGatewayAuthStrategy(t.Context(), "gateway", request)
    case "update":
        var request c.UpdateAIGatewayAuthStrategyRequest
        if err = json.Unmarshal(raw, &request); err != nil { t.Fatal(err) }
        _, _ = client.AIGatewayAuthStrategies.UpdateAiGatewayAuthStrategy(t.Context(), o.UpdateAiGatewayAuthStrategyRequest{
            GatewayID:"gateway", AuthStrategyIDOrName:"probe", UpdateAIGatewayAuthStrategyRequest:request,
        })
    case "response":
        raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"id":"00000000-0000-0000-0000-000000000001","created_at":"2026-11-11T00:00:00Z","updated_at":"2026-11-11T00:00:00Z"}`)
        var response c.AIGatewayAuthStrategy
        if err = json.Unmarshal(raw, &response); err != nil { t.Fatal(err) }
        capture.body, err = json.Marshal(response)
        if err != nil { t.Fatal(err) }
    }
    if capture.body == nil { t.Fatal("SDK sent no request") }
    var body map[string]any
    if err = json.Unmarshal(capture.body, &body); err != nil { t.Fatal(err) }
    return body["config"].(map[string]any)
}

func TestKongctlOIDCConfigWirePresence(t *testing.T) {
    var cases []oidcCompatibilityCase
    if err := json.Unmarshal(oidcBaseline, &cases); err != nil { t.Fatal(err) }
    for _, tc := range cases {
        for _, verb := range []string{"create","update","response"} {
            for _, state := range []string{"omitted","null","default","custom"} {
                t.Run(tc.Name+"/"+verb+"/"+state, func(t *testing.T) {
                    config := make(map[string]any)
                    for k,v := range tc.Config { config[k]=v }
                    var expected any
                    switch state {
                    case "null": config["upstream_access_token_header"] = nil
                    case "default": expected="authorization:bearer"; config["upstream_access_token_header"]=expected
                    case "custom": expected="x-custom-token"; config["upstream_access_token_header"]=expected
                    }
                    actual := oidcWireConfig(t, config, verb)
                    header, present := actual["upstream_access_token_header"]
                    if present != (state != "omitted") || header != expected {
                        t.Fatalf("header=(%#v,%v), want (%#v,%v)", header,present,expected,state!="omitted")
                    }
                    delete(actual,"upstream_access_token_header")
                    if !reflect.DeepEqual(tc.Expected[verb], actual) {
                        t.Fatalf("non-header behavior changed: got %#v, want baseline %#v", actual, tc.Expected[verb])
                    }
                })
            }
        }
    }
}
