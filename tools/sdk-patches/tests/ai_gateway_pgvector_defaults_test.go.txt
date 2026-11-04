package components

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestKongctlPgvectorConnectionPreservesOmission(t *testing.T) {
	for _, input := range []string{
		`{"type":"pgvector","dimensions":1536,"distance_metric":"cosine"}`,
		`{"type":"pgvector","dimensions":1536,"distance_metric":"cosine","ssl":{}}`,
		`{"type":"pgvector","dimensions":1536,"distance_metric":"cosine","host":"127.0.0.1",` +
			`"database":"kong-pgvector","port":5432,"user":"postgres","timeout":5000,` +
			`"ssl":{"enabled":false,"required":false,"verify":false,"version":"tlsv1_2"}}`,
	} {
		t.Run(input, func(t *testing.T) {
			// Exercise union decoding, as used by model requests, rather than
			// only directly constructing a generated struct.
			var request AIGatewayModelVectorDBConfig
			if err := json.Unmarshal([]byte(input), &request); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var expected, actual map[string]any
			if err := json.Unmarshal([]byte(input), &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(expected, actual) {
				t.Fatalf("pgvector connection changed during round trip: got %s, want %s", data, input)
			}
		})
	}
	data, err := json.Marshal(CreateAIGatewayModelVectorDBConfigPgvector(AIGatewayModelVectorDBConfigPgVector{
		Dimensions: 1536, DistanceMetric: DistanceMetricCosine,
	}))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 3 {
		t.Fatalf("struct construction introduced connection settings: %s", data)
	}
}
