package sdkkonnectgo_test

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
)

type authServerArraysCapture struct{ body []byte }

func (c *authServerArraysCapture) Do(request *http.Request) (*http.Response, error) {
	var err error
	c.body, err = io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: 400,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"wire probe"}`)),
		Request:    request,
	}, nil
}

func TestKongctlAuthServerOriginsWirePresence(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		present       bool
		length        int
	}{
		{"omitted", `{}`, false, 0},
		{"empty", `{"trusted_origins":[]}`, true, 0},
		{"populated", `{"trusted_origins":["https://client.example.test"]}`, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &authServerArraysCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			// Older public SDKs have no auth-server API; avoid requiring its types
			// merely to install the shared patch bundle on those versions.
			api := reflect.ValueOf(client).Elem().FieldByName("AuthServer")
			if !api.IsValid() || api.IsNil() {
				t.Skip("auth-server management is absent from this SDK")
			}
			method := api.MethodByName("UpdateAuthServer")
			if !method.IsValid() || method.Type().NumIn() != 4 {
				t.Fatal("unexpected auth-server SDK update signature")
			}
			request := reflect.New(method.Type().In(2))
			if err := json.Unmarshal([]byte(tc.payload), request.Interface()); err != nil {
				t.Fatal(err)
			}
			method.Call([]reflect.Value{
				reflect.ValueOf(t.Context()), reflect.ValueOf("server-id"), request.Elem(),
			})
			if capture.body == nil {
				t.Fatal("SDK sent no update request")
			}
			var body map[string]any
			if err := json.Unmarshal(capture.body, &body); err != nil {
				t.Fatal(err)
			}
			value, present := body["trusted_origins"]
			if present != tc.present {
				t.Fatalf("wire presence = %t, want %t", present, tc.present)
			}
			if present {
				items, ok := value.([]any)
				if !ok || len(items) != tc.length {
					t.Fatalf("unexpected origins: %#v", value)
				}
			}
		})
	}
}
