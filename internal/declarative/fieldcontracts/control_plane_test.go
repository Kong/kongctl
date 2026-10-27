package fieldcontracts_test

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneProjectionParticipation(t *testing.T) {
	empty, disabled := "", false
	cluster := kkComps.CreateControlPlaneRequestClusterType("CLUSTER_TYPE_HYBRID")
	auth := kkComps.AuthType("")
	first := kkComps.ProxyURL{Host: "first", Protocol: "https", Port: 443}
	second := kkComps.ProxyURL{Host: "second", Protocol: "http", Port: 80}
	desired := kkComps.CreateControlPlaneRequest{
		Description: &empty, ClusterType: &cluster, AuthType: &auth, CloudGateway: &disabled,
		ProxyUrls: []kkComps.ProxyURL{second, first},
	}
	fields := map[string]any{}
	fieldcontracts.ExtractControlPlaneFields(desired, fields)
	require.Len(t, fields, 5)
	require.Equal(t, false, fields[fieldcontracts.FieldCloudGateway])
	var create kkComps.CreateControlPlaneRequest
	require.NoError(t, fieldcontracts.MapControlPlaneCreateFields(fields, &create))
	require.Equal(t, desired, create)
	current := kkComps.ControlPlane{Description: "old", Config: kkComps.Config{
		AuthType: "pinned_client_certs", CloudGateway: true, ProxyUrls: []kkComps.ProxyURL{first, second},
	}}
	changes := fieldcontracts.CompareControlPlaneFields(desired, current)
	require.Equal(t, map[string]fieldcontracts.FieldChange{
		fieldcontracts.FieldDescription: {Old: "old", New: ""},
		fieldcontracts.FieldProxyURLs:   {Old: current.Config.ProxyUrls, New: desired.ProxyUrls},
	}, changes, "empty auth and create-only fields must not become updates; URL order matters")
	require.Empty(t, fieldcontracts.CompareControlPlaneFields(kkComps.CreateControlPlaneRequest{}, current))
	require.Empty(t, fieldcontracts.CompareControlPlaneFields(
		kkComps.CreateControlPlaneRequest{ProxyUrls: []kkComps.ProxyURL{}}, kkComps.ControlPlane{},
	))
	var update kkComps.UpdateControlPlaneRequest
	err := fieldcontracts.MapControlPlaneUpdateField(fieldcontracts.FieldClusterType, string(cluster), &update)
	require.NoError(t, err)
	require.NoError(t, fieldcontracts.MapControlPlaneUpdateField(fieldcontracts.FieldCloudGateway, true, &update))
	require.Equal(t, kkComps.UpdateControlPlaneRequest{}, update)
	require.NoError(t, fieldcontracts.MapControlPlaneUpdateField(fieldcontracts.FieldAuthType, "", &update))
	require.NotNil(t, update.AuthType, "comparison skips empty auth, but request mapping retains it")
	require.Empty(t, *update.AuthType)
}

func TestControlPlaneExportOmissions(t *testing.T) {
	var target kkComps.CreateControlPlaneRequest
	fieldcontracts.ExportControlPlaneFields(kkComps.ControlPlane{
		Description: " \t", Config: kkComps.Config{ProxyUrls: []kkComps.ProxyURL{}},
	}, &target)
	require.Equal(t, kkComps.CreateControlPlaneRequest{}, target)
	current := kkComps.ControlPlane{Description: " spaced ", Config: kkComps.Config{
		ClusterType: "CLUSTER_TYPE_HYBRID", AuthType: "pinned_client_certs", CloudGateway: true,
		ProxyUrls: []kkComps.ProxyURL{{Host: "proxy", Port: 443, Protocol: "https"}},
	}}
	fieldcontracts.ExportControlPlaneFields(current, &target)
	require.Equal(t, " spaced ", *target.Description)
	require.Equal(t, string(current.Config.ClusterType), string(*target.ClusterType))
	require.Equal(t, string(current.Config.AuthType), string(*target.AuthType))
	require.True(t, *target.CloudGateway)
	require.Equal(t, current.Config.ProxyUrls, target.ProxyUrls)
}
