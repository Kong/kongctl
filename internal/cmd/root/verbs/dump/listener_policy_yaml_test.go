package dump_test

import (
"encoding/json"
"fmt"
"strings"
"testing"

kkComps "github.com/Kong/sdk-konnect-go/models/components"
declresources "github.com/kong/kongctl/internal/declarative/resources"
"sigs.k8s.io/yaml"
)

func TestListenerPolicyYAMLSerialization(t *testing.T) {
policyType := "tls_server"
policyName := "default-listener-policy-for-dump-test-name"
desc := "Listener Policy for Dump Test"
enabled := true

policy := kkComps.EventGatewayListenerPolicy{
ID:          "some-uuid",
Type:        policyType,
Name:        &policyName,
Description: &desc,
Enabled:     &enabled,
}

// rawConfig as it would come from the API response
rawConfig := map[string]any{
"certificates": []any{
map[string]any{
"certificate": "-----BEGIN CERTIFICATE-----\nMIIBxjCCAUygAwIBAgIUX9TaLbWF76yQc8IGR+YRbeiDlHkwCgYIKoZIzj0EAwIwGjEYMBYGA1UEAwwPa29uZ19jbHVzdGVyaW5nMB4XDTI0MDMwMTE0MzkxNloXDTI3MDMwMTE0MzkxNlowGjEYMBYGA1UEAwwPa29uZ19jbHVzdGVyaW5nMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEcMndCotXzeZ9vGAMfDfZ7UxUuP5bcIrwwUOI8YlpMdvB12HvjtS7O0/ONr3fBeCWagRuitPEqd4b3EJuD8kuFUMt+2A09N6KY1YDJWgKHei7rzKgrefzVt11XgBiDsUBo1MwUTAdBgNVHQ4EFgQUIrdAC8p02h60GZW0Jlh2Vcg/WeMwHwYDVR0jBBgwFoAUIrdAC8p02h60GZW0Jlh2Vcg/WeMwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNoADBlAjBYb+yQf33sItlmsONLc41Agtx73FMEN7LfWA85OtlkMie1N1x0mj08pzS/Xc1VONwCMQDN9sBn3Kody0gse+EXYSuPPj1oo9jmFB9/xrpz35YpDATvuyhH8xwSJ4xMuxQiduc=\n-----END CERTIFICATE-----",
"key": "-----BEGIN PRIVATE KEY-----\nMIG2AgEAMBAGByqGSM49AgEGBSuBBAAiBIGeMIGbAgEBBDDLuRX+uzSbstvLWsQrWwuGK4AdjLU/tN9A/fn03gxNvppKw++SBtnLyB+9YZ29YA+hZANiAARwyd0Ki1fN5n28YAx8N9ntTFS4/ltwivDBQ4jxiWkx28HXYe+O1Ls7T842vd8F4JZqBG6K08Sp3hvcQm4PyS4VQy37YDT03opjVgMlaAod6LuvMqCt5/NW3XVeAGIOxQE=\n-----END PRIVATE KEY-----",
},
},
"versions": map[string]any{
"min": "TLSv1.2",
},
"allow_plaintext": true,
}

policyMap := map[string]any{
"type":   policy.Type,
"id":     policy.ID,
"config": rawConfig,
}
if policy.Name != nil {
policyMap["name"] = *policy.Name
}
if policy.Description != nil {
policyMap["description"] = *policy.Description
}
if policy.Enabled != nil {
policyMap["enabled"] = *policy.Enabled
}

data, err := json.Marshal(policyMap)
if err != nil {
t.Fatalf("Error marshaling policyMap: %v", err)
}
fmt.Printf("policyMap JSON: %s\n\n", string(data))

var createPolicy kkComps.EventGatewayListenerPolicyCreate
if err := json.Unmarshal(data, &createPolicy); err != nil {
t.Fatalf("Error unmarshaling into EventGatewayListenerPolicyCreate: %v", err)
}

resource := declresources.EventGatewayListenerPolicyResource{
EventGatewayListenerPolicyCreate: createPolicy,
Ref:                              policy.ID,
}

jsonBytes, err := json.Marshal(resource)
if err != nil {
t.Fatalf("Error marshaling resource to JSON: %v", err)
}
fmt.Printf("Resource JSON: %s\n\n", string(jsonBytes))

yamlBytes, err := yaml.Marshal(resource)
if err != nil {
t.Fatalf("Error marshaling resource to YAML: %v", err)
}
fmt.Printf("Resource YAML:\n%s\n", string(yamlBytes))

yamlStr := string(yamlBytes)
if !strings.Contains(yamlStr, "name: "+policyName) {
t.Errorf("Expected YAML to contain 'name: %s', got:\n%s", policyName, yamlStr)
}
if !strings.Contains(yamlStr, "type: tls_server") {
t.Errorf("Expected YAML to contain 'type: tls_server', got:\n%s", yamlStr)
}
if !strings.Contains(yamlStr, "enabled: true") {
t.Errorf("Expected YAML to contain 'enabled: true', got:\n%s", yamlStr)
}
}
