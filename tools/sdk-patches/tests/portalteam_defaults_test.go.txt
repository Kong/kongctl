package components

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPortalTeamRequestsPreserveMembershipManagement(t *testing.T) {
	disabled, enabled := false, true
	for _, value := range []*bool{nil, &disabled, &enabled} {
		for name, request := range map[string]any{
			"create": PortalCreateTeamRequest{Name: "team", KonnectManaged: value},
			"update": PortalUpdateTeamRequest{KonnectManaged: value},
		} {
			t.Run(name, func(t *testing.T) {
				data, err := json.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(data, &fields); err != nil {
					t.Fatal(err)
				}
				actual, present := fields["konnect_managed"]
				if value == nil {
					if present {
						t.Fatalf("omitted membership management serialized as %v", actual)
					}
				} else if !present || actual != *value {
					t.Fatalf("membership management = %v, want %v", actual, *value)
				}
			})
		}
	}
	var create PortalCreateTeamRequest
	if err := json.Unmarshal([]byte(`{"name":"team"}`), &create); err != nil {
		t.Fatal(err)
	}
	var update PortalUpdateTeamRequest
	if err := json.Unmarshal([]byte(`{"name":"renamed"}`), &update); err != nil {
		t.Fatal(err)
	}
	if create.KonnectManaged != nil || update.KonnectManaged != nil {
		t.Fatal("decoding an omitted field must not introduce membership management")
	}
}

// Organization team membership management is present only in the internal SDK.
func TestOrganizationTeamRequestsPreserveMembershipManagement(t *testing.T) {
	for name, request := range map[string]any{"create": &CreateTeam{Name: "team"}, "update": &UpdateTeam{}} {
		t.Run(name, func(t *testing.T) {
			field := reflect.ValueOf(request).Elem().FieldByName("KonnectManaged")
			if !field.IsValid() {
				t.Skip("organization membership management is not exposed in this SDK")
			}
			if field.Type() != reflect.TypeFor[*bool]() {
				t.Fatalf("KonnectManaged type = %v, want *bool", field.Type())
			}
			for _, value := range []*bool{nil, new(false), new(true)} {
				field.Set(reflect.ValueOf(value))
				encoded, err := json.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				actual, present := fields["konnect_managed"]
				if value == nil && present {
					t.Fatalf("omitted membership management serialized: %s", encoded)
				}
				if value != nil && (!present || actual != *value) {
					t.Fatalf("membership management = %v, want %v: %s", actual, *value, encoded)
				}
			}
			field.SetZero()
			if err := json.Unmarshal([]byte(`{"name":"team"}`), request); err != nil {
				t.Fatal(err)
			}
			if !field.IsNil() {
				t.Fatal("decoding an omitted field must not introduce membership management")
			}
		})
	}
}
