package components

import (
	"encoding/json"
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
