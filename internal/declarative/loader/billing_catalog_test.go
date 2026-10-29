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
metering-billing:
  tax_codes:
    - ref: tax
      name: Tax
      key: tax
      app_mappings: []
  features:
    - ref: feature
      name: Usage
      key: usage
  plans:
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
		{"missing phase key", "          key: standard\n", "", "key"},
		{"missing rate-card name", "            - name: Usage\n              key: usage", "            - key: usage", "name"},
		{"missing rate-card price", "              price: {type: unit, amount: '0.1'}\n", "", "price"},
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
	for _, kind := range []string{"meters", "features", "tax_codes", "currencies", "apps", "profiles", "plans"} {
		t.Run(kind, func(t *testing.T) {
			subject, err := resources.ResolveExplainSubject("metering-billing." + kind)
			require.NoError(t, err)
			scaffold, err := resources.RenderScaffoldYAML(subject)
			require.NoError(t, err)
			require.Contains(t, scaffold, "metering-billing:\n")
			require.NotContains(t, scaffold, "create_billing_profile")
			require.NotContains(t, resources.RenderExplainSchema(subject).Properties, "create_billing_profile")
			path := filepath.Join(t.TempDir(), "scaffold.yaml")
			require.NoError(t, os.WriteFile(path, []byte(scaffold), 0o600))
			_, err = New().LoadFile(path)
			require.NoError(t, err, "%s", scaffold)
		})
	}
}

func TestBillingAppRejectsInstallationShortcut(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		t.Run(value, func(t *testing.T) {
			manifest := "metering-billing:\n  apps:\n    - ref: sandbox\n" +
				"      name: Sandbox\n      type: sandbox\n      create_billing_profile: " + value + "\n"
			_, err := New().parseYAML(strings.NewReader(manifest), "test.yaml", ".")
			require.ErrorContains(t, err, "unknown field 'create_billing_profile'")
		})
	}
}

func TestMeteringBillingGroupingScope(t *testing.T) {
	kinds := map[string]resources.ResourceType{
		"meters":     resources.ResourceTypeMeter,
		"features":   resources.ResourceTypeFeature,
		"tax_codes":  resources.ResourceTypeTaxCode,
		"currencies": resources.ResourceTypeCurrency,
		"apps":       resources.ResourceTypeBillingApp,
		"profiles":   resources.ResourceTypeBillingProfile,
		"plans":      resources.ResourceTypeBillingPlan,
	}
	for key, kind := range kinds {
		t.Run(key, func(t *testing.T) {
			rs, err := New().parseYAML(strings.NewReader("metering-billing:\n  "+key+": []\n"), "test.yaml", ".")
			require.NoError(t, err)
			require.Equal(t, map[resources.ResourceType]struct{}{kind: {}}, rs.SyncScope.RootResourceTypes)
		})
	}
	for _, manifest := range []string{"metering-billing: {}", "metering-billing: null", "{}"} {
		rs, err := New().parseYAML(strings.NewReader(manifest), "test.yaml", ".")
		require.NoError(t, err)
		require.Empty(t, rs.SyncScope.RootResourceTypes)
	}
}

func TestMeteringBillingGroupingRejectsInvalidPlacement(t *testing.T) {
	for _, key := range []string{
		"meters", "features", "tax_codes", "currencies", "billing_apps", "billing_profiles", "billing_plans",
	} {
		_, err := New().parseYAML(strings.NewReader(key+": []"), "test.yaml", ".")
		require.ErrorContains(t, err, key)
	}
	for _, manifest := range []string{
		"metering-billing: []", "metering-billing: {typo: []}", "metering-billing: {plans: {}}",
	} {
		_, err := New().parseYAML(strings.NewReader(manifest), "test.yaml", ".")
		require.Error(t, err, manifest)
	}
}

func TestMeteringBillingGroupingMergesFileNamespaces(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"first", "second"} {
		manifest := "_defaults:\n  kongctl: {namespace: " + name + "}\n" +
			"metering-billing:\n  features:\n    - ref: " + name + "\n" +
			"      name: " + name + "\n      key: " + name + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(manifest), 0o600))
	}
	rs, err := New().LoadFromSources([]Source{{Path: dir, Type: SourceTypeDirectory}}, false)
	require.NoError(t, err)
	require.Len(t, rs.Features, 2)
	for _, feature := range rs.Features {
		require.Equal(t, feature.Ref, resources.GetNamespace(feature.Kongctl))
	}
}
