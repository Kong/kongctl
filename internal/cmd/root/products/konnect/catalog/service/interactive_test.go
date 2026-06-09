package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogServiceDetailView_UsesAPIFieldLabels(t *testing.T) {
	detail := catalogServiceDetailView(catalogServiceView{
		DisplayRecord: catalogServiceDisplayRecord{
			ID:          "service-id",
			Name:        "billing",
			DisplayName: "Billing",
			Description: "Billing service",
		},
		Labels: map[string]string{
			"env": "test",
		},
		RawCustom: map[string]any{
			"tier": "gold",
		},
	})

	for _, expected := range []string{
		"name: billing",
		"display_name: Billing",
		"id: service-id",
		"description: Billing service",
		"labels: {",
		`"env": "test"`,
		"custom_fields: {",
		`"tier": "gold"`,
	} {
		require.Contains(t, detail, expected)
	}

	for _, oldLabel := range []string{"Name", "Display Name", "ID", "Description", "Labels", "Custom Fields"} {
		require.NotContains(t, detail, oldLabel)
	}
}
