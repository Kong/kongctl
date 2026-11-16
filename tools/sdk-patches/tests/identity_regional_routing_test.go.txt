package sdkkonnectgo_test

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/models/operations"
)

var errIdentityRoutingCaptured = errors.New("routing probe captured request")

type identityRoutingCapture struct {
	requests []*http.Request
}

func (c *identityRoutingCapture) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	return nil, errIdentityRoutingCaptured
}

func TestKongctlIdentityRegionalRouting(t *testing.T) {
	// Enumerate the real SDK methods so newly added operations are also exercised.
	// Counts describe the selected SDK and force review when its surface changes.
	for _, group := range []struct {
		field string
		count int
	}{
		{"KongIdentityDirectories", 6},
		{"KongPrincipal", 8},
		{"KongPrincipalCredential", 18},
		{"KongPrincipalIdentity", 6},
		{"PrincipalGroup", 15},
		{"TrustedIDPs", 7},
		{"TokenVaultKeys", 5},
		{"TokenVaultProviders", 13},
		{"TokenVaultCredentials", 3},
	} {
		t.Run(group.field, func(t *testing.T) {
			api := reflect.ValueOf(sdk.New()).Elem().FieldByName(group.field)
			if !api.IsValid() || api.IsNil() {
				t.Fatal("expected Kong Identity API is absent")
			}
			if api.NumMethod() != group.count {
				t.Fatalf("operation count = %d, want %d; review SDK changes", api.NumMethod(), group.count)
			}
			for i := range api.NumMethod() {
				method := api.Type().Method(i).Name
				for _, endpoint := range []struct{ name, url string }{
					{"us", "https://us.api.konghq.com"},
					{"eu", "https://eu.api.konghq.com"},
					{"au", "https://au.api.konghq.com"},
					{"custom", "https://custom.invalid/konnect"},
				} {
					for _, override := range []bool{false, true} {
						name := method + "/" + endpoint.name
						if override {
							name += "/override"
						}
						t.Run(name, func(t *testing.T) {
							capture := &identityRoutingCapture{}
							client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL(endpoint.url))
							call := reflect.ValueOf(client).Elem().FieldByName(group.field).MethodByName(method)
							args := []reflect.Value{reflect.ValueOf(t.Context())}
							for j := 1; j < call.Type().NumIn()-1; j++ {
								args = append(args, identityRoutingArgument(call.Type().In(j)))
							}
							want := endpoint.url
							opts := []operations.Option{}
							if override {
								want = "https://operation.invalid/override"
								opts = append(opts, operations.WithServerURL(want))
							}
							args = append(args, reflect.ValueOf(opts))
							result := call.CallSlice(args)
							if len(capture.requests) != 1 {
								t.Fatalf(
									"captured %d requests, want 1; error: %v",
									len(capture.requests),
									result[len(result)-1].Interface(),
								)
							}
							if err, ok := result[len(result)-1].Interface().(error); !ok ||
								!errors.Is(err, errIdentityRoutingCaptured) {
								t.Fatalf("expected capture error, got %v", result)
							}
							req := capture.requests[0]
							if !strings.HasPrefix(req.URL.String(), want+"/") {
								t.Fatalf("request URL = %s; want base %s", req.URL, want)
							}
							if method == "ImportOrgChart" {
								identityRoutingCheckMultipart(t, req)
							}
						})
					}
				}
			}
		})
	}
}

func identityRoutingArgument(typ reflect.Type) reflect.Value {
	value := reflect.New(typ).Elem()
	if value.Kind() == reflect.String {
		value.SetString("731559d3-39bd-48c7-b33c-eb192847eb20")
	}
	if value.Type() == reflect.TypeFor[components.ImportPrincipalsRequest]() {
		value.Set(reflect.ValueOf(components.ImportPrincipalsRequest{
			File: components.File{
				FileName: "principals.csv",
				Content:  []byte("external_id,name\nprobe,Probe\n"),
			},
		}))
	}
	if value.Kind() == reflect.Struct {
		for k := range value.NumField() {
			field := value.Field(k)
			switch field.Type() {
			case reflect.TypeFor[components.CreatePrincipalKeyBody]():
				key := components.CreateCreatePrincipalKeyBodyV1(components.CreatePrincipalKeyBodyv1{})
				field.Set(reflect.ValueOf(key))
			case reflect.TypeFor[components.KongPrincipalIdentityBody]():
				identity := components.CreateKongPrincipalIdentityBodyCustom(components.KongPrincipalIdentityCustom{})
				field.Set(reflect.ValueOf(identity))
			}
			if value.Type().Field(k).Tag.Get("pathParam") != "" &&
				field.Kind() == reflect.String {
				field.SetString("731559d3-39bd-48c7-b33c-eb192847eb20")
			}
		}
	}
	return value
}

func identityRoutingCheckMultipart(t *testing.T, req *http.Request) {
	t.Helper()
	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(req.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("missing multipart CSV fixture: %v", err)
		}
		if part.FormName() == "file" {
			body, err := io.ReadAll(part)
			if err != nil || part.FileName() != "principals.csv" || len(body) == 0 {
				t.Fatalf("invalid multipart fixture: filename=%q body=%q error=%v", part.FileName(), body, err)
			}
			return
		}
	}
}

func TestKongctlIdentityRoutingPreservesGlobalAPIs(t *testing.T) {
	for _, endpoint := range []string{
		"https://us.api.konghq.com", "https://eu.api.konghq.com",
		"https://au.api.konghq.com", "https://custom.invalid/konnect",
	} {
		for _, call := range []string{"GetOrganizationsMe", "GetUsersMe"} {
			t.Run(endpoint+"/"+call, func(t *testing.T) {
				capture := &identityRoutingCapture{}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL(endpoint))
				reflect.ValueOf(client.Me).MethodByName(call).Call([]reflect.Value{reflect.ValueOf(t.Context())})
				if len(capture.requests) != 1 {
					t.Fatalf("captured %d requests, want 1", len(capture.requests))
				}
				if got := capture.requests[0].URL.Host; got != "global.api.konghq.com" {
					t.Fatalf("global API host = %s", got)
				}
			})
		}
	}
}
