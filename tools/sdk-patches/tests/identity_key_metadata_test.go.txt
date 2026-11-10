package sdkkonnectgo_test

import (
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
)

func TestKongctlIdentityKeyMetadataWire(t *testing.T) {
	for _, tc := range []struct{ api, method, body, input string }{
		{"KongPrincipalCredential", "CreatePrincipalKey", "CreatePrincipalKeyBody", `{"type":"imported","secret":"synthetic","managed_by":{"tool":"kongctl"}}`},
		{"PrincipalGroup", "CreateGroupKey", "CreatePrincipalKeyBody", `{"type":"imported","secret":"synthetic","managed_by":{"tool":"kongctl"}}`},
		{"KongPrincipalCredential", "UpdatePrincipalKey", "UpdatePrincipalKeyBody", `{"name":null,"managed_by":{"tool":"kongctl","old":null}}`},
		{"PrincipalGroup", "UpdateGroupKey", "UpdatePrincipalKeyBody", `{"name":null,"managed_by":{"tool":"kongctl","old":null}}`},
		{"PrincipalGroup", "UpdateGroupKey", "UpdatePrincipalKeyBody", `{"name":"renamed"}`},
		{"PrincipalGroup", "UpdateGroupKey", "UpdatePrincipalKeyBody", `{"expires_at":null}`},
	} {
		t.Run(tc.api+"/"+tc.method+"/"+tc.input, func(t *testing.T) {
			capture := &credentialExpirationCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			api := reflect.ValueOf(client).Elem().FieldByName(tc.api)
			if !api.IsValid() || api.IsNil() {
				t.Skip("Identity API absent")
			}
			method := api.MethodByName(tc.method)
			argument := reflect.New(method.Type().In(1))
			for _, parent := range []string{"DirectoryID", "PrincipalID", "GroupID", "APIKeyID"} {
				if field := argument.Elem().FieldByName(parent); field.IsValid() {
					field.SetString("731559d3-39bd-48c7-b33c-eb192847eb20")
				}
			}
			body := argument.Elem().FieldByName(tc.body)
			if err := json.Unmarshal([]byte(tc.input), body.Addr().Interface()); err != nil {
				t.Fatal(err)
			}
			method.Call([]reflect.Value{reflect.ValueOf(t.Context()), argument.Elem()})
			if capture.calls != 1 {
				t.Fatal("request did not reach the wire")
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(tc.input), &want); err != nil {
				t.Fatal(err)
			}
			for field, value := range want {
				actual, present := capture.body[field]
				if !present || !reflect.DeepEqual(value, actual) {
					t.Fatalf("%s was omitted or changed on the wire", field)
				}
			}
			if _, set := want["name"]; !set {
				if _, sent := capture.body["name"]; sent {
					t.Fatal("omitted name became a write")
				}
			}
		})
	}
}
