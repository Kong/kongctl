package fieldcontracts_test

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/stretchr/testify/require"
)

func TestAPITextPresenceAcrossProjections(t *testing.T) {
	empty, version, previous := "", "v2", "previous"
	desired := kkComps.CreateAPIRequest{Description: &empty, Version: &version, Slug: &empty}
	current := kkComps.APIResponseSchema{Version: &previous, Slug: &previous}
	fields := map[string]any{}
	fieldcontracts.ExtractAPIFields(desired, fields)
	require.Equal(t, map[string]any{
		fieldcontracts.FieldDescription: "", fieldcontracts.FieldVersion: "v2", fieldcontracts.FieldSlug: "",
	}, fields)
	var create kkComps.CreateAPIRequest
	fieldcontracts.MapAPICreateFields(fields, &create)
	require.Equal(t, desired, create)

	changes := fieldcontracts.CompareAPIFields(desired, current)
	require.Equal(t, map[string]fieldcontracts.FieldChange{
		fieldcontracts.FieldVersion: {Old: previous, New: version},
		fieldcontracts.FieldSlug:    {Old: previous, New: empty},
	}, changes)
	updates := map[string]any{}
	for name, change := range changes {
		updates[name] = change.New
	}
	var update kkComps.UpdateAPIRequest
	fieldcontracts.MapAPIUpdateFields(updates, &update)
	require.Nil(t, update.Description, "equal nil/empty text must not become a PATCH field")
	require.Equal(t, &version, update.Version)
	require.Equal(t, &empty, update.Slug)
	require.Empty(t, fieldcontracts.CompareAPIFields(kkComps.CreateAPIRequest{}, current))

	var exported kkComps.CreateAPIRequest
	fieldcontracts.ExportAPIFields(kkComps.APIResponseSchema{Version: &version, Slug: &empty}, &exported)
	require.Nil(t, exported.Description)
	require.Equal(t, &version, exported.Version)
	require.Equal(t, &empty, exported.Slug, "export must retain explicitly empty text")
}

func TestAPIAttributesKeepRequestAndExportPoliciesDistinct(t *testing.T) {
	raw := map[string]any{"remove": nil, "audience": []any{"web", "mobile"}}
	normalized := map[string]any{"remove": nil, "audience": []string{"web", "mobile"}}
	desired := kkComps.CreateAPIRequest{Attributes: raw}
	fields := map[string]any{}
	fieldcontracts.ExtractAPIFields(desired, fields)
	require.Equal(t, normalized, fields[fieldcontracts.FieldAttributes])

	changes := fieldcontracts.CompareAPIFields(desired, kkComps.APIResponseSchema{
		Attributes: map[string][]string{"remove": {"old"}, "audience": {"mobile", "web"}},
	})
	require.Len(t, changes, 1)
	require.Equal(t, normalized, changes[fieldcontracts.FieldAttributes].New)
	var create kkComps.CreateAPIRequest
	var update kkComps.UpdateAPIRequest
	fieldcontracts.MapAPICreateFields(fields, &create)
	fieldcontracts.MapAPIUpdateFields(map[string]any{
		fieldcontracts.FieldAttributes: changes[fieldcontracts.FieldAttributes].New,
	}, &update)
	require.Equal(t, normalized, create.Attributes)
	require.Equal(t, normalized, update.Attributes, "null removals and request list order must survive comparison")
	require.Equal(t, []any{"web", "mobile"}, raw["audience"], "comparison must not mutate inputs")

	var exported kkComps.CreateAPIRequest
	fieldcontracts.ExportAPIFields(kkComps.APIResponseSchema{Attributes: raw}, &exported)
	require.Equal(t, raw, exported.Attributes, "export must not canonicalize remote attributes")
	fieldcontracts.ExportAPIFields(kkComps.APIResponseSchema{Attributes: map[string]any{}}, &exported)
	require.Nil(t, exported.Attributes)

	for _, value := range []any{nil, map[string]any{}} {
		fields := map[string]any{}
		fieldcontracts.ExtractAPIFields(kkComps.CreateAPIRequest{Attributes: value}, fields)
		_, present := fields[fieldcontracts.FieldAttributes]
		require.Equal(t, value != nil, present, "empty attributes are declared; nil attributes are omitted")
	}
}

func TestAPIFieldMappingPreservesIgnoredTextAndAttributeFallback(t *testing.T) {
	seed := "existing"
	create := kkComps.CreateAPIRequest{Description: &seed, Version: &seed, Slug: &seed, Attributes: "existing"}
	update := kkComps.UpdateAPIRequest{Description: &seed, Version: &seed, Slug: &seed, Attributes: "existing"}
	fields := map[string]any{fieldcontracts.FieldDescription: 42, fieldcontracts.FieldSlug: nil}
	fieldcontracts.MapAPICreateFields(fields, &create)
	fieldcontracts.MapAPIUpdateFields(fields, &update)
	require.Equal(t, kkComps.CreateAPIRequest{
		Description: &seed, Version: &seed, Slug: &seed, Attributes: "existing",
	}, create)
	require.Equal(t, kkComps.UpdateAPIRequest{
		Description: &seed, Version: &seed, Slug: &seed, Attributes: "existing",
	}, update)
	for _, value := range []any{"raw fallback", nil} {
		fields := map[string]any{fieldcontracts.FieldAttributes: value}
		fieldcontracts.MapAPICreateFields(fields, &create)
		fieldcontracts.MapAPIUpdateFields(fields, &update)
		require.Equal(t, value, create.Attributes)
		require.Equal(t, value, update.Attributes)
	}
}
