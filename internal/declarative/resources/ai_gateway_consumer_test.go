package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerMatchesOnlyByName(t *testing.T) {
	const id = "c3296957-12fb-4bdf-ae35-15b19a749592"
	for _, tc := range []struct{ name, ref, resolvedID string }{
		{name: "same-name", ref: "local-alias"},
		{name: "different-name", ref: id},
		{name: "different-name", ref: "local-alias", resolvedID: id},
		{name: "different-name", ref: "same-name"},
	} {
		t.Run(tc.name+"/"+tc.ref, func(t *testing.T) {
			desired := AIGatewayConsumerResource{
				BaseResource: BaseResource{Ref: tc.ref},
				CreateAIGatewayConsumerRequest: kkComps.CreateAIGatewayConsumerRequest{
					Name: tc.name, DisplayName: "Shared Display",
				},
			}
			desired.SetKonnectID(tc.resolvedID)
			matched := desired.TryMatchKonnectResource(kkComps.AIGatewayConsumer{
				ID: id, Name: "same-name", DisplayName: "Shared Display",
			})
			require.Equal(t, tc.name == "same-name", matched)
			if matched {
				require.Equal(t, id, desired.GetKonnectID())
			}
		})
	}
}
