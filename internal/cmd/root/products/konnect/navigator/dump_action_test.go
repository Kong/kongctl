package navigator

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/stretchr/testify/require"

	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func TestDumpResourceForSelectionUsesParentType(t *testing.T) {
	resource, err := dumpResourceForSelection(tableview.SelectionContext{
		ParentType: common.ViewParentAPI,
		Label:      "Orders API",
	})

	require.NoError(t, err)
	require.Equal(t, "apis", resource)
}

func TestDumpResourceForSelectionUsesNavigatorResourceRow(t *testing.T) {
	resource, err := dumpResourceForSelection(tableview.SelectionContext{
		Row: table.Row{common.ViewResourceControlPlanes},
	})

	require.NoError(t, err)
	require.Equal(t, "control_planes", resource)
}

func TestDumpResourceForSelectionRejectsUnsupportedResource(t *testing.T) {
	_, err := dumpResourceForSelection(tableview.SelectionContext{
		Label: common.ViewResourceCatalogServices,
		Row:   table.Row{common.ViewResourceCatalogServices},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "dump is not available")
}

func TestStringFieldExtractsStringAndStringPointerIDs(t *testing.T) {
	id := "12345678-1234-1234-1234-123456789012"

	require.Equal(t, id, stringField(struct {
		ID string
	}{ID: id}, "ID"))
	require.Equal(t, id, stringField(&struct {
		ID *string
	}{ID: &id}, "ID"))
}

func TestDefaultDumpOutputFile(t *testing.T) {
	require.Equal(t, "apis.yaml", defaultDumpOutputFile("apis", "apis", ""))
	require.Equal(t,
		"control-planes-prod.yaml",
		defaultDumpOutputFile("control_planes", "Prod", "12345678-1234-1234-1234-123456789012"))
	require.Equal(t,
		"event-gateways-12345678-123.yaml",
		defaultDumpOutputFile("event_gateways", "", "12345678-1234-1234-1234-123456789012"))
}
