package sdkkonnectgo_test

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	sdk "github.com/Kong/sdk-konnect-go"
)

type credentialExpirationCapture struct {
	body  map[string]any
	calls int
}

func (c *credentialExpirationCapture) Do(request *http.Request) (*http.Response, error) {
	c.calls++
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &c.body); err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"id":"731559d3-39bd-48c7-b33c-eb192847eb20"}`)), Request: request,
	}, nil
}

func credentialExpirationField(value reflect.Value) reflect.Value {
	if field := value.FieldByName("ExpiresAt"); field.IsValid() {
		return field
	}
	// A key create body is a discriminated union. Only the selected variant
	// exists after decoding the public request body.
	for i := range value.NumField() {
		field := value.Field(i)
		if field.Kind() == reflect.Pointer && !field.IsNil() && field.Elem().Kind() == reflect.Struct {
			if found := credentialExpirationField(field.Elem()); found.IsValid() {
				return found
			}
		}
	}
	return reflect.Value{}
}

func TestKongctlIdentityCredentialExpirationWireCases(t *testing.T) {
	const expiration = "2031-01-01T00:00:00Z"
	for _, tc := range []struct {
		api, method, body, input string
		required                 bool
	}{
		{"KongPrincipalCredential", "CreatePrincipalKey", "CreatePrincipalKeyBody", `{"type":"v1"}`, false},
		{"KongPrincipalCredential", "CreatePrincipalKey", "CreatePrincipalKeyBody", `{"type":"imported","secret":"test-secret"}`, false},
		{"KongPrincipalCredential", "UpdatePrincipalKey", "UpdatePrincipalKeyBody", `{}`, false},
		{"PrincipalGroup", "CreateGroupKey", "CreatePrincipalKeyBody", `{"type":"v1"}`, false},
		{"PrincipalGroup", "CreateGroupKey", "CreatePrincipalKeyBody", `{"type":"imported","secret":"test-secret"}`, false},
		{"PrincipalGroup", "UpdateGroupKey", "UpdatePrincipalKeyBody", `{}`, false},
		{"KongPrincipalCredential", "CreatePassword", "CreatePasswordBody", `{}`, false},
		{"KongPrincipalCredential", "CreatePassword", "CreatePasswordBody", `{"secret":"test-secret"}`, false},
		{"KongPrincipalCredential", "UpdatePassword", "UpdatePasswordBody", `{}`, false},
		{"KongPrincipalCredential", "ReplacePassword", "ReplacePasswordBody", `{"secret":"test-secret"}`, true},
	} {
		for _, state := range []string{"omitted", "timestamp", "null"} {
			t.Run(tc.api+"/"+tc.method+"/"+tc.input+"/"+state, func(t *testing.T) {
				capture := &credentialExpirationCapture{}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
				api := reflect.ValueOf(client).Elem().FieldByName(tc.api)
				if !api.IsValid() || api.IsNil() {
					t.Skip("Identity credential operations are absent from this SDK")
				}
				method := api.MethodByName(tc.method)
				if !method.IsValid() {
					t.Fatal("expected credential operation is missing")
				}
				argument := reflect.New(method.Type().In(1))
				for _, parent := range []string{"DirectoryID", "PrincipalID", "GroupID", "BasicAuthID", "APIKeyID", "PasswordID"} {
					if field := argument.Elem().FieldByName(parent); field.IsValid() {
						field.SetString("731559d3-39bd-48c7-b33c-eb192847eb20")
					}
				}
				body := argument.Elem().FieldByName(tc.body)
				if !body.IsValid() {
					t.Fatal("expected credential body is missing")
				}
				var input map[string]any
				if err := json.Unmarshal([]byte(tc.input), &input); err != nil {
					t.Fatal(err)
				}
				if state == "timestamp" {
					input["expires_at"] = expiration
				}
				if state == "null" {
					input["expires_at"] = nil
				}
				data, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, body.Addr().Interface()); err != nil {
					t.Fatal(err)
				}
				field := credentialExpirationField(body)
				method.Call([]reflect.Value{reflect.ValueOf(t.Context()), argument.Elem()})
				if capture.calls != 1 {
					t.Fatal("credential request did not reach the wire")
				}
				value, present := capture.body["expires_at"]
				wantPresent := tc.required || state != "omitted"
				if present != wantPresent {
					t.Fatalf("expires_at presence = %v, want %v", present, wantPresent)
				}
				if state == "timestamp" {
					if value != expiration {
						t.Fatalf("expires_at = %v, want timestamp", value)
					}
				} else if wantPresent && value != nil {
					t.Fatal("expires_at must be explicit null")
				}
				if field.IsValid() && state == "timestamp" {
					if getter := field.MethodByName("Get"); getter.IsValid() {
						values := getter.Call(nil)
						if !values[1].Bool() {
							t.Fatal("expiration lost presence")
						}
						field = values[0]
					}
					for field.Kind() == reflect.Pointer {
						field = field.Elem()
					}
					if field.Type() != reflect.TypeFor[time.Time]() {
						t.Fatal("expiration lost its date-time type")
					}
				}
			})
		}
	}
}
