package components

import (
	"encoding/json"
	"testing"
)

func TestKongctlPatchedBooleansPreserveExplicitValues(t *testing.T) {
	for _, value := range []*bool{nil, new(false), new(true)} {
		for name, test := range map[string]struct {
			request any
			field   string
		}{
			"backend TLS":              {BackendClusterTLS{InsecureSkipVerify: value}, "insecure_skip_verify"},
			"email":                    {PatchCustomPortalEmailTemplatePayload{Enabled: value}, "enabled"},
			"webhook":                  {UpdatePortalAuditLogWebhook{Enabled: value}, "enabled"},
			"identity provider":        {UpdateIdentityProvider{Enabled: value}, "enabled"},
			"portal identity provider": {PortalUpdateIdentityProvider{Enabled: value}, "enabled"},
			"DCR":                      {UpdateDcrConfigHTTPInRequest{AllowMultipleCredentials: value}, "allow_multiple_credentials"},
			"portal authentication":    {UpdatePortal{AuthenticationEnabled: value}, "authentication_enabled"},
			"portal RBAC":              {UpdatePortal{RbacEnabled: value}, "rbac_enabled"},
			"portal SIPR":              {UpdatePortal{SiprEnabled: value}, "sipr_enabled"},
			"portal developers":        {UpdatePortal{AutoApproveDevelopers: value}, "auto_approve_developers"},
			"portal applications":      {UpdatePortal{AutoApproveApplications: value}, "auto_approve_applications"},
			"portal PII":               {UpdatePortal{NotificationsDeveloperPiiVisibilityEnabled: value}, "notifications_developer_pii_visibility_enabled"},
		} {
			t.Run(name, func(t *testing.T) {
				data, err := json.Marshal(test.request)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(data, &fields); err != nil {
					t.Fatal(err)
				}
				actual, present := fields[test.field]
				if value == nil {
					if present {
						t.Fatalf("unset %s serialized: %s", test.field, data)
					}
				} else if !present || actual != *value {
					t.Fatalf("%s = %v, want %v: %s", test.field, actual, *value, data)
				}
			})
		}
	}
}
