package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func sampleTrustedCA() string {
	return "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----"
}

func newTrustBundlePlanner() *Planner {
	return &Planner{}
}

func makeTrustBundleCurrent(name string,
	desc *string, ca string, labels map[string]string) state.EventGatewayTLSTrustBundle {
	normalized := labels
	if normalized == nil {
		normalized = map[string]string{}
	}
	return state.EventGatewayTLSTrustBundle{
		TLSTrustBundle: kkComps.TLSTrustBundle{
			ID:          "bundle-id-123",
			Name:        name,
			Description: desc,
			Config:      kkComps.TLSTrustBundleConfig{TrustedCa: ca},
			Labels:      labels,
		},
		NormalizedLabels: normalized,
	}
}

func makeTrustBundleDesired(name string,
	desc *string, ca string, labels map[string]string) resources.EventGatewayTLSTrustBundleResource {
	return resources.EventGatewayTLSTrustBundleResource{
		CreateTLSTrustBundleRequest: kkComps.CreateTLSTrustBundleRequest{
			Name:        name,
			Description: desc,
			Config:      kkComps.TLSTrustBundleConfig{TrustedCa: ca},
			Labels:      labels,
		},
		Ref: name + "-ref",
	}
}

// ---------------------------------------------------------------------------
// shouldUpdateTrustBundle – no change
// ---------------------------------------------------------------------------

func TestShouldUpdateTrustBundle_NoChanges(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	desc := "a description"
	ca := sampleTrustedCA()
	labels := map[string]string{"env": "prod"}

	current := makeTrustBundleCurrent("my-bundle", &desc, ca, labels)
	desired := makeTrustBundleDesired("my-bundle", &desc, ca, labels)

	needsUpdate, fields, changed := p.shouldUpdateTrustBundle(current, desired)

	assert.False(t, needsUpdate)
	assert.Empty(t, fields)
	assert.Empty(t, changed)
}

func TestShouldUpdateTrustBundle_NoChanges_NilDescription(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	ca := sampleTrustedCA()

	current := makeTrustBundleCurrent("my-bundle", nil, ca, nil)
	desired := makeTrustBundleDesired("my-bundle", nil, ca, nil)

	needsUpdate, _, _ := p.shouldUpdateTrustBundle(current, desired)
	assert.False(t, needsUpdate)
}

// ---------------------------------------------------------------------------
// shouldUpdateTrustBundle – name changed
// ---------------------------------------------------------------------------

func TestShouldUpdateTrustBundle_NameChanged(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	ca := sampleTrustedCA()

	current := makeTrustBundleCurrent("old-name", nil, ca, nil)
	desired := makeTrustBundleDesired("new-name", nil, ca, nil)

	needsUpdate, updateFields, changed := p.shouldUpdateTrustBundle(current, desired)

	assert.True(t, needsUpdate)
	require.Contains(t, changed, "name")
	assert.Equal(t, "old-name", changed["name"].Old)
	assert.Equal(t, "new-name", changed["name"].New)
	assert.NotEmpty(t, updateFields)
}

// ---------------------------------------------------------------------------
// shouldUpdateTrustBundle – description changed
// ---------------------------------------------------------------------------

func TestShouldUpdateTrustBundle_DescriptionChanged(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	ca := sampleTrustedCA()
	oldDesc := "old description"
	newDesc := "new description"

	current := makeTrustBundleCurrent("my-bundle", &oldDesc, ca, nil)
	desired := makeTrustBundleDesired("my-bundle", &newDesc, ca, nil)

	needsUpdate, updateFields, changed := p.shouldUpdateTrustBundle(current, desired)

	assert.True(t, needsUpdate)
	require.Contains(t, changed, "description")
	assert.Equal(t, oldDesc, changed["description"].Old)
	assert.Equal(t, newDesc, changed["description"].New)
	assert.NotEmpty(t, updateFields)
}

func TestShouldUpdateTrustBundle_DescriptionAddedToNil(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	ca := sampleTrustedCA()
	newDesc := "now has a description"

	current := makeTrustBundleCurrent("my-bundle", nil, ca, nil)
	desired := makeTrustBundleDesired("my-bundle", &newDesc, ca, nil)

	needsUpdate, _, changed := p.shouldUpdateTrustBundle(current, desired)

	assert.True(t, needsUpdate)
	assert.Contains(t, changed, "description")
}

// ---------------------------------------------------------------------------
// shouldUpdateTrustBundle – trusted_ca changed
// ---------------------------------------------------------------------------

func TestShouldUpdateTrustBundle_TrustedCaChanged(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	oldCA := "-----BEGIN CERTIFICATE-----\nOLD\n-----END CERTIFICATE-----"
	newCA := "-----BEGIN CERTIFICATE-----\nNEW\n-----END CERTIFICATE-----"

	current := makeTrustBundleCurrent("my-bundle", nil, oldCA, nil)
	desired := makeTrustBundleDesired("my-bundle", nil, newCA, nil)

	needsUpdate, updateFields, changed := p.shouldUpdateTrustBundle(current, desired)

	assert.True(t, needsUpdate)
	require.Contains(t, changed, "config.trusted_ca")
	assert.Equal(t, oldCA, changed["config.trusted_ca"].Old)
	assert.Equal(t, newCA, changed["config.trusted_ca"].New)
	assert.NotEmpty(t, updateFields)
}

// ---------------------------------------------------------------------------
// shouldUpdateTrustBundle – labels changed
// ---------------------------------------------------------------------------

func TestShouldUpdateTrustBundle_LabelsChanged(t *testing.T) {
	t.Parallel()

	p := newTrustBundlePlanner()
	ca := sampleTrustedCA()

	current := makeTrustBundleCurrent("my-bundle", nil, ca, map[string]string{"env": "staging"})
	desired := makeTrustBundleDesired("my-bundle", nil, ca, map[string]string{"env": "prod"})

	needsUpdate, _, changed := p.shouldUpdateTrustBundle(current, desired)

	assert.True(t, needsUpdate)
	assert.Contains(t, changed, "labels")
}
