package sdkkonnectgo_test

import (
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
)

type identityCursorCapture struct{ query url.Values }

func (c *identityCursorCapture) Do(request *http.Request) (*http.Response, error) {
	c.query = request.URL.Query()
	return &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
		Body:    io.NopCloser(strings.NewReader(`{"data":[],"meta":{"page":{"size":1,"next":null}}}`)),
		Request: request,
	}, nil
}

func TestKongctlIdentityCursorQueryWireNames(t *testing.T) {
	for _, tc := range []struct {
		field, method string
		positional    bool
	}{
		{"KongIdentityDirectories", "ListKongDirectories", true},
		{"KongPrincipal", "ListPrincipals", false},
		{"KongPrincipalIdentity", "ListIdentities", false},
		{"KongPrincipalCredential", "ListPrincipalKeys", false},
		{"KongPrincipalCredential", "ListPrincipalBasicAuths", false},
		{"KongPrincipalCredential", "ListPasswords", false},
		{"PrincipalGroup", "ListGroups", false},
		{"PrincipalGroup", "ListGroupMembership", false},
	} {
		t.Run(tc.method, func(t *testing.T) {
			capture := &identityCursorCapture{}
			client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
			api := reflect.ValueOf(client).Elem().FieldByName(tc.field)
			if !api.IsValid() || api.IsNil() {
				t.Skip("Kong Identity operations are absent from this SDK")
			}
			method := api.MethodByName(tc.method)
			if !method.IsValid() {
				t.Fatal("missing expected list method")
			}
			argument := reflect.New(method.Type().In(1))
			var page reflect.Value
			if tc.positional {
				page = reflect.New(method.Type().In(1).Elem())
			} else {
				pageField := argument.Elem().FieldByName("Page")
				page = reflect.New(pageField.Type().Elem())
				pageField.Set(page)
				argument.Elem().FieldByName("DirectoryID").SetString("731559d3-39bd-48c7-b33c-eb192847eb20")
				for _, parent := range []string{"PrincipalID", "GroupID", "BasicAuthID"} {
					if field := argument.Elem().FieldByName(parent); field.IsValid() {
						field.SetString("731559d3-39bd-48c7-b33c-eb192847eb21")
					}
				}
			}
			page.Elem().FieldByName("Size").Set(reflect.ValueOf(new(int64(1))))
			page.Elem().FieldByName("After").Set(reflect.ValueOf(new("cursor+/==")))
			args := []reflect.Value{reflect.ValueOf(t.Context()), argument.Elem()}
			if tc.positional {
				args[1] = page
				args = append(args, reflect.Zero(method.Type().In(2)))
			}
			method.Call(args)
			if capture.query.Get("page[size]") != "1" || capture.query.Get("page[after]") != "cursor+/==" {
				t.Fatalf("cursor query = %v; want bracketed page[size] and page[after]", capture.query)
			}
			if capture.query.Has("size") || capture.query.Has("after") {
				t.Fatal("flat pagination fields must be absent")
			}
		})
	}
}
