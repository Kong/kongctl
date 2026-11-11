package sdkkonnectgo_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/models/components"
)

type aiGatewayUpdateCapture struct {
	body []byte
}

func (c *aiGatewayUpdateCapture) Do(request *http.Request) (*http.Response, error) {
	var err error
	c.body, err = io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"gateway-id","name":"existing-gateway","display_name":"Existing gateway",
			"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"
		}`)),
		Request: request,
	}, nil
}

func TestKongctlAIGatewayUpdateRuntimeWirePresence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   *bool
		payload string
	}{
		{"omitted", nil, `{"name":"existing-gateway","display_name":"Existing gateway"}`},
		{"disabled", new(false), `{"name":"existing-gateway","display_name":"Existing gateway","runtime_auto_upgrade":false}`},
		{"enabled", new(true), `{"name":"existing-gateway","display_name":"Existing gateway","runtime_auto_upgrade":true}`},
	} {
		for _, decode := range []bool{false, true} {
			mode := "constructed"
			if decode {
				mode = "decoded"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				request := components.UpdateAIGatewayRequest{
					Name: "existing-gateway", DisplayName: "Existing gateway", RuntimeAutoUpgrade: tc.value,
				}
				if decode {
					request = components.UpdateAIGatewayRequest{}
					if err := json.Unmarshal([]byte(tc.payload), &request); err != nil {
						t.Fatal(err)
					}
				}
				capture := &aiGatewayUpdateCapture{}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
				if _, err := client.AIGateways.UpdateAiGateway(t.Context(), "gateway-id", request); err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(capture.body, &fields); err != nil {
					t.Fatal(err)
				}
				actual, present := fields["runtime_auto_upgrade"]
				if tc.value == nil {
					if present {
						t.Fatalf("unset runtime_auto_upgrade serialized: %s", capture.body)
					}
				} else if !present || actual != *tc.value {
					t.Fatalf("runtime_auto_upgrade = %v, want %v: %s", actual, *tc.value, capture.body)
				}
			})
		}
	}
}
