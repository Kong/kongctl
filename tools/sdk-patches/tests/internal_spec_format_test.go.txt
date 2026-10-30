package operations

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/Kong/sdk-konnect-go/internal/utils"
)

func TestInternalSpecFormatPreservesOmissionAndExplicitValues(t *testing.T) {
	for name, request := range map[string]any{
		"create": &CreateAPIVersionRequest{},
		"update": &UpdateAPIVersionRequest{},
		"fetch":  &FetchAPIVersionRequest{},
	} {
		t.Run(name, func(t *testing.T) {
			field := reflect.ValueOf(request).Elem().FieldByName("SpecFormat")
			if !field.IsValid() {
				t.Skip("spec_format is not exposed in this SDK")
			}
			if field.Kind() != reflect.Pointer || field.Type().Elem().Kind() != reflect.String {
				t.Fatalf("unexpected SpecFormat type: %v", field.Type())
			}
			for _, format := range []string{"", "json", "yaml"} {
				field.SetZero()
				if format != "" {
					value := reflect.New(field.Type().Elem())
					value.Elem().SetString(format)
					field.Set(value)
				}
				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://konnect.test", nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := utils.PopulateQueryParams(context.Background(), req, request, nil, nil); err != nil {
					t.Fatal(err)
				}
				query := req.URL.Query()
				if format == "" && query.Has("spec_format") {
					t.Fatalf("omitted spec_format serialized: %s", req.URL.RawQuery)
				}
				if format != "" && query.Get("spec_format") != format {
					t.Fatalf("spec_format = %q, want %q", query.Get("spec_format"), format)
				}
			}
		})
	}
}
