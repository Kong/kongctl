package sdkkonnectgo_test

import (
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
)

func TestKongctlClientPatchWirePresence(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		fields        map[string]any
	}{
		{"omitted", `{}`, map[string]any{}},
		{"empty", `{"grant_types":[],"response_types":[],"redirect_uris":[],"allow_scopes":[]}`,
			map[string]any{"grant_types": []any{}, "response_types": []any{},
				"redirect_uris": []any{}, "allow_scopes": []any{}}},
		{"populated", `{"grant_types":["client_credentials"],"response_types":["code"],` +
			`"redirect_uris":["https://client.example.test/callback"],` +
			`"allow_scopes":["731559d3-39bd-48c7-b33c-eb192847eb20"],` +
			`"token_endpoint_auth_method":"client_secret_basic"}`,
			map[string]any{"grant_types": []any{"client_credentials"}, "response_types": []any{"code"},
				"redirect_uris":              []any{"https://client.example.test/callback"},
				"allow_scopes":               []any{"731559d3-39bd-48c7-b33c-eb192847eb20"},
				"token_endpoint_auth_method": "client_secret_basic"}},
		{"public-client", `{"token_endpoint_auth_method":"none"}`,
			map[string]any{"token_endpoint_auth_method": "none"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &authServerArraysCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			api := reflect.ValueOf(client).Elem().FieldByName("AuthServerClients")
			if !api.IsValid() || api.IsNil() {
				t.Skip("auth-server clients are absent from this SDK")
			}
			method := api.MethodByName("UpdateAuthServerClient")
			if !method.IsValid() || method.Type().NumIn() != 3 {
				t.Fatal("unexpected auth-server client update signature")
			}
			request := reflect.New(method.Type().In(1))
			fields := request.Elem()
			fields.FieldByName("AuthServerID").SetString("server-id")
			fields.FieldByName("ClientID").SetString("opaque-client-id")
			if err := json.Unmarshal([]byte(tc.payload), fields.FieldByName("UpdateClient").Addr().Interface()); err != nil {
				t.Fatal(err)
			}
			method.Call([]reflect.Value{reflect.ValueOf(t.Context()), request.Elem()})
			if capture.body == nil {
				t.Fatal("SDK sent no client update")
			}
			var body map[string]any
			if err := json.Unmarshal(capture.body, &body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body, tc.fields) {
				t.Fatalf("wire fields = %#v, want %#v", body, tc.fields)
			}
		})
	}
}
