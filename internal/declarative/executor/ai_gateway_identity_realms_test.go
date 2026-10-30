package executor

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	o "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
)

type realmsCapture struct{ body []byte }

func (r *realmsCapture) Do(req *http.Request) (*http.Response, error) {
	var err error
	r.body, err = io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"probe"}`)),
		Request:    req,
	}, nil
}

func TestAIGatewayAuthStrategyPreservesIdentityRealmsOnWire(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		present      bool
		length       int
	}{
		{"omitted", `{"principals":{"enabled":true}}`, false, 0},
		{"empty", `{"identity_realms":[],"principals":{"enabled":true}}`, true, 0},
		{"populated", `{"identity_realms":[{"scope":"cp"}]}`, true, 1},
	} {
		for _, verb := range []string{"create", "update"} {
			t.Run(tc.name+"/"+verb, func(t *testing.T) {
				var fields map[string]any
				raw := `{"name":"probe","display_name":"Probe","type":"key-auth","config":` + tc.config + `}`
				if err := json.Unmarshal([]byte(raw), &fields); err != nil {
					t.Fatal(err)
				}
				// Exercise a saved-plan JSON round trip before the real kongctl mapper.
				encoded, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				capture := &realmsCapture{}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
				adapter := NewAIGatewayAuthStrategyAdapter(nil)
				if verb == "create" {
					var request c.CreateAIGatewayAuthStrategyRequest
					if err = adapter.MapCreateFields(t.Context(), nil, fields, &request); err != nil {
						t.Fatal(err)
					}
					_, _ = client.AIGatewayAuthStrategies.CreateAiGatewayAuthStrategy(t.Context(), "gateway", request)
				} else {
					var request c.UpdateAIGatewayAuthStrategyRequest
					if err = adapter.MapUpdateFields(t.Context(), nil, fields, &request, nil); err != nil {
						t.Fatal(err)
					}
					_, _ = client.AIGatewayAuthStrategies.UpdateAiGatewayAuthStrategy(
						t.Context(), o.UpdateAiGatewayAuthStrategyRequest{
							GatewayID:                          "gateway",
							AuthStrategyIDOrName:               "probe",
							UpdateAIGatewayAuthStrategyRequest: request,
						},
					)
				}
				if capture.body == nil {
					t.Fatal("SDK sent no request")
				}
				var body map[string]any
				if err = json.Unmarshal(capture.body, &body); err != nil {
					t.Fatal(err)
				}
				config := body[planner.FieldConfig].(map[string]any)
				val, present := config["identity_realms"]
				if present != tc.present {
					t.Fatalf("wire presence=%v want=%v", present, tc.present)
				}
				if present {
					items, ok := val.([]any)
					if !ok || len(items) != tc.length {
						t.Fatalf("unexpected realms: %#v", val)
					}
				}
				if tc.name != "populated" {
					principals := config["principals"].(map[string]any)
					if principals["enabled"] != true {
						t.Fatal("lost enabled principals")
					}
				}
			})
		}
	}
}
