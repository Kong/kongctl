package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayV13ExplainMatchesSDK(t *testing.T) {
	cluster, err := eventGatewayClusterPolicyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	assertCustomExplainSupportsSDKShape[kkComps.EventGatewayRequestRuleValidatorPolicy](
		t, explainUnionBranchByType(t, cluster, "request_rule_validator"),
	)
	produce, err := eventGatewayProducePolicyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	assertCustomExplainSupportsSDKShape[kkComps.EventGatewayParsedRecordMaskFieldsProducePolicyCreate](
		t, explainUnionBranchByType(t, produce, "mask_fields"),
	)
	assertCustomExplainSupportsSDKShape[kkComps.EventGatewayParsedRecordTranscodeProducePolicyCreate](
		t, explainUnionBranchByType(t, produce, "transcode"),
	)
	consume, err := eventGatewayConsumePolicyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	assertCustomExplainSupportsSDKShape[kkComps.EventGatewayParsedRecordMaskFieldsConsumePolicyCreate](
		t, explainUnionBranchByType(t, consume, "mask_fields"),
	)
	assertCustomExplainSupportsSDKShape[kkComps.EventGatewayParsedRecordTranscodeConsumePolicyCreate](
		t, explainUnionBranchByType(t, consume, "transcode"),
	)
}
