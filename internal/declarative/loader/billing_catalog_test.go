package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

const billingShapeManifest = `_defaults:
  kongctl: {namespace: billing}
tax_codes:
  - ref: tax
    name: Tax
    key: tax
    app_mappings: []
features:
  - ref: feature
    name: Usage
    key: usage
billing_plans:
  - ref: plan
    name: Plan
    key: plan
    currency: USD
    billing_cadence: P1M
    phases:
      - name: Standard
        key: standard
        rate_cards:
          - name: Usage
            key: usage
            feature: {id: feature}
            price: {type: unit, amount: '0.1'}
            entitlement: {type: metered, usage_period: P1M}
            tax_config: {code: {id: tax}}
`

func TestBillingNativeShapesAndTypedReferences(t *testing.T) {
	for _, test := range []struct{ name, old, new, want string }{
		{"native", "", "", ""},
		{"tagged feature", "feature: {id: feature}", "feature: {id: !ref feature#id}", ""},
		{"wrong feature type", "feature: {id: feature}", "feature: {id: tax}", "declared feature"},
		{"wrong tagged feature type", "feature: {id: feature}", "feature: {id: !ref tax#id}", "declared feature"},
		{"wrong selector", "feature: {id: feature}", "feature: {id: !ref feature#name}", "#id"},
		{"unknown nested price", "amount: '0.1'", "amount: '0.1', typo: true", "typo"},
		{"unknown price discriminator", "type: unit", "type: wrong", "discriminator"},
		{"missing phase key", "        key: standard\n", "", "key"},
		{"missing rate-card name", "          - name: Usage\n            key: usage", "          - key: usage", "name"},
		{"missing rate-card price", "            price: {type: unit, amount: '0.1'}\n", "", "price"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := billingShapeManifest
			if test.old != "" {
				manifest = strings.Replace(manifest, test.old, test.new, 1)
			}
			path := filepath.Join(t.TempDir(), "billing.yaml")
			require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
			_, err := New().LoadFile(path)
			if test.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.want)
			}
		})
	}
}

func TestBillingScaffoldsLoad(t *testing.T) {
	for _, kind := range []string{"tax_codes", "currencies", "billing_apps", "billing_profiles", "billing_plans"} {
		t.Run(kind, func(t *testing.T) {
			subject, err := resources.ResolveExplainSubject(kind)
			require.NoError(t, err)
			scaffold, err := resources.RenderScaffoldYAML(subject)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "scaffold.yaml")
			require.NoError(t, os.WriteFile(path, []byte(scaffold), 0o600))
			_, err = New().LoadFile(path)
			require.NoError(t, err, "%s", scaffold)
		})
	}
}
