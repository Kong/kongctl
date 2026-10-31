package sdkkonnectgo_test

import (
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
)

func TestKongctlClaimScopesWirePresence(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		present       bool
		length        int
	}{
		{"omitted", `{}`, false, 0},
		{"empty", `{"include_in_scopes":[]}`, true, 0},
		{"populated", `{"include_in_scopes":["731559d3-39bd-48c7-b33c-eb192847eb20"]}`, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &authServerArraysCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			api := reflect.ValueOf(client).Elem().FieldByName("AuthServerClaims")
			if !api.IsValid() || api.IsNil() {
				t.Skip("auth-server claims are absent from this SDK")
			}
			method := api.MethodByName("UpdateAuthServerClaim")
			if !method.IsValid() || method.Type().NumIn() != 3 {
				t.Fatal("unexpected auth-server claim update signature")
			}
			request := reflect.New(method.Type().In(1))
			fields := request.Elem()
			fields.FieldByName("AuthServerID").SetString("server-id")
			fields.FieldByName("ClaimID").SetString("claim-id")
			if err := json.Unmarshal([]byte(tc.payload), fields.FieldByName("UpdateClaim").Addr().Interface()); err != nil {
				t.Fatal(err)
			}
			method.Call([]reflect.Value{reflect.ValueOf(t.Context()), request.Elem()})
			if capture.body == nil {
				t.Fatal("SDK sent no claim update")
			}
			var body map[string]any
			if err := json.Unmarshal(capture.body, &body); err != nil {
				t.Fatal(err)
			}
			value, present := body["include_in_scopes"]
			if present != tc.present {
				t.Fatalf("wire presence = %t, want %t", present, tc.present)
			}
			if present {
				items, ok := value.([]any)
				if !ok || len(items) != tc.length {
					t.Fatalf("unexpected scope references: %#v", value)
				}
			}
		})
	}
}
