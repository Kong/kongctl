package resources

import (
	"testing"

	"sigs.k8s.io/yaml"
)

func TestPortalEmailConfigUnmarshal(t *testing.T) {
	yamlConfig := `
portals:
  - ref: portal-email
    name: Portal Email
    email_config:
      ref: email-config
      domain_name: mail.example.com
      from_name: From Name
      from_email: from@example.com
      reply_to_email: reply@example.com
`

	var parsed struct {
		Portals []PortalResource `yaml:"portals"`
	}

	if err := yaml.UnmarshalStrict([]byte(yamlConfig), &parsed); err != nil {
		t.Fatalf("unexpected error parsing portal email config: %v", err)
	}

	if len(parsed.Portals) != 1 {
		t.Fatalf("expected 1 portal, got %d", len(parsed.Portals))
	}

	if parsed.Portals[0].EmailConfig == nil {
		t.Fatalf("expected email config to be set")
	}
}
