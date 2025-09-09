//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

// copyDir recursively copies a directory tree from src to dst.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		outPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(outPath, 0o755)
		}
		in, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(outPath, in, 0o644)
	})
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}
}

// readPortalName parses portal.yaml for the portal name.
func readPortalName(t *testing.T, portalYAMLPath string) string {
	t.Helper()
	b, err := os.ReadFile(portalYAMLPath)
	if err != nil {
		t.Fatalf("read portal.yaml failed: %v", err)
	}
	var m struct {
		Portals []struct {
			Name string `yaml:"name"`
		} `yaml:"portals"`
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse portal.yaml failed: %v", err)
	}
	if len(m.Portals) == 0 {
		t.Fatalf("no portal in portal.yaml")
	}
	return m.Portals[0].Name
}

// readSMSAPIName reads the SMS API title from the OpenAPI file under inputs.
func readSMSAPIName(t *testing.T, inputsRoot string) string {
	t.Helper()
	p := filepath.Join(inputsRoot, "apis", "sms", "openapi.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read sms openapi.yaml failed: %v", err)
	}
	var m struct {
		Info struct {
			Title string `yaml:"title"`
		} `yaml:"info"`
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse sms openapi.yaml failed: %v", err)
	}
	if m.Info.Title == "" {
		t.Fatalf("sms openapi title empty")
	}
	return m.Info.Title
}

// mutatePublicationVisibility sets the visibility of the SMS publication in apis.yaml.
func mutatePublicationVisibility(t *testing.T, apisYAMLPath string, newVisibility string, pubRef string) {
	t.Helper()
	b, err := os.ReadFile(apisYAMLPath)
	if err != nil {
		t.Fatalf("read apis.yaml failed: %v", err)
	}
	var doc yamlv3.Node
	if err := yamlv3.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse apis.yaml (node) failed: %v", err)
	}
	// The document root typically has one child which is the mapping
	root := doc.Content
	if len(root) == 0 {
		t.Fatalf("unexpected YAML document structure")
	}
	m := root[0]
	if m.Kind != yamlv3.MappingNode {
		t.Fatalf("expected mapping at document root")
	}
	// Find 'apis' sequence
	apisNode := findMapKey(m, "apis")
	if apisNode == nil || apisNode.Kind != yamlv3.SequenceNode {
		t.Fatalf("apis key missing or not a sequence")
	}
	found := false
	for _, apiItem := range apisNode.Content { // each is a mapping
		if apiItem.Kind != yamlv3.MappingNode {
			continue
		}
		pubsNode := findMapKey(apiItem, "publications")
		if pubsNode == nil || pubsNode.Kind != yamlv3.SequenceNode {
			continue
		}
		for _, pubItem := range pubsNode.Content { // mapping
			if pubItem.Kind != yamlv3.MappingNode {
				continue
			}
			refNode := findMapKey(pubItem, "ref")
			if refNode != nil && refNode.Value == pubRef {
				// Set or create visibility key
				visNode := findMapKey(pubItem, "visibility")
				if visNode == nil {
					// append key and value
					pubItem.Content = append(
						pubItem.Content,
						&yamlv3.Node{Kind: yamlv3.ScalarNode, Value: "visibility", Tag: "!!str"},
						&yamlv3.Node{Kind: yamlv3.ScalarNode, Value: newVisibility, Tag: "!!str"},
					)
				} else {
					visNode.Value = newVisibility
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("publication ref %s not found in apis.yaml", pubRef)
	}
	out, err := yamlv3.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshal mutated apis.yaml failed: %v", err)
	}
	if err := os.WriteFile(apisYAMLPath, out, 0o644); err != nil {
		t.Fatalf("write mutated apis.yaml failed: %v", err)
	}
}

// findMapKey returns the value node for a given key within a mapping node.
func findMapKey(m *yamlv3.Node, key string) *yamlv3.Node {
	if m == nil || m.Kind != yamlv3.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		v := m.Content[i+1]
		if k.Value == key {
			return v
		}
	}
	return nil
}

// Test_Declarative_General is a multi-step scenario test; first applies the full example, then edits a publication.
func Test_Declarative_General(t *testing.T) {
	harness.RequireBinary(t)
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}

	// Step 000: reset org (captured as a step)
	stepReset, err := harness.NewStep(t, cli, "000-reset_org")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	if err := stepReset.ResetOrg("before_test"); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Step 001: apply base full example
	step0, err := harness.NewStep(t, cli, "001-apply_base")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	srcRoot := filepath.Join("testdata", "declarative", "portal", "full")
	copyDir(t, srcRoot, step0.InputsDir)

	// Expectations
	portalName := readPortalName(t, filepath.Join(step0.InputsDir, "portal.yaml"))
	_ = step0.SaveJSON("expected.json", map[string]any{"portal_name": portalName})

	// Apply base
	applyOut, err := step0.Apply(
		filepath.Join(step0.InputsDir, "portal.yaml"),
		filepath.Join(step0.InputsDir, "apis.yaml"),
	)
	if err != nil {
		t.Fatalf("apply base failed: %v", err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply base failed changes=%d", applyOut.Summary.Failed)
	}

	// Validate basics via CLI
	// portals
	var portals []struct {
		Name string `json:"name"`
	}
	ok := retry(6, 1500*time.Millisecond, func() bool {
		portals = nil
		if err := step0.GetAndObserve("portals", &portals, map[string]any{"name": portalName}); err != nil {
			return false
		}
		for _, p := range portals {
			if p.Name == portalName {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("expected portal %q after base apply", portalName)
	}
	// apis
	var apis []struct {
		Name string `json:"name"`
	}
	ok = retry(6, 1500*time.Millisecond, func() bool {
		apis = nil
		if err := step0.GetAndObserve("apis", &apis, map[string]any{"min_count": 1}); err != nil {
			return false
		}
		return len(apis) >= 1
	})
	if !ok {
		t.Fatalf("expected at least one API after base apply")
	}
	step0.AppendCheck("PASS: base applied, portal present and APIs exist")

	// Step 002: edit publication visibility (sms publication -> private)
	step1, err := harness.NewStep(t, cli, "002-edit_publication_visibility")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	// Copy base fixtures again for independent inputs
	copyDir(t, srcRoot, step1.InputsDir)
	// Mutate apis.yaml to set publication visibility
	mutatePublicationVisibility(t, filepath.Join(step1.InputsDir, "apis.yaml"), "private", "sms-api-to-getting-started")
	smsAPIName := readSMSAPIName(t, step1.InputsDir)
	_ = step1.SaveJSON(
		"expected.json",
		map[string]any{
			"portal_name":     portalName,
			"api_name":        smsAPIName,
			"publication_ref": "sms-api-to-getting-started",
			"visibility":      "private",
		},
	)

	// Apply the full configuration again (portal + apis)
	applyOut, err = step1.Apply(
		filepath.Join(step1.InputsDir, "portal.yaml"),
		filepath.Join(step1.InputsDir, "apis.yaml"),
	)
	if err != nil {
		t.Fatalf("apply publication edit failed: %v", err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("publication edit apply had failures: %d", applyOut.Summary.Failed)
	}

	// Discover apiId and portalId via CLI lists
	// get apis -> find by name
	apis = nil
	if err := step1.GetAndObserve("apis", &apis, map[string]any{"name": smsAPIName}); err != nil {
		t.Fatalf("get apis failed: %v", err)
	}
	// 'get apis' response structure is a list with 'name' fields; if it doesn't include 'id', we proceed to HTTP fallback
	// fetch api list via HTTP and find ID by name
	var apiList struct {
		Data []struct{ ID, Name string } `json:"data"`
	}
	if err := step1.GetKonnectJSON("get_apis_http", "/v3/apis", &apiList, map[string]any{"name": smsAPIName}); err != nil {
		t.Fatalf("http get apis failed: %v", err)
	}
	apiID := ""
	for _, a := range apiList.Data {
		if a.Name == smsAPIName {
			apiID = a.ID
			break
		}
	}
	if apiID == "" {
		t.Fatalf("api id not found for name %q", smsAPIName)
	}

	// get portals -> find portal id by name
	portals = nil
	if err := step1.GetKonnectJSON("get_portals_http", "/v3/portals", &struct {
		Data []struct{ ID, Name string } `json:"data"`
	}{}, map[string]any{"name": portalName}); err != nil {
		t.Fatalf("http get portals failed: %v", err)
	}
	var portalList struct {
		Data []struct{ ID, Name string } `json:"data"`
	}
	// Retrieve from last observation (stdout.json already recorded), but easier: re-request and decode
	if err := step1.GetKonnectJSON("get_portals_http2", "/v3/portals", &portalList, map[string]any{"name": portalName}); err != nil {
		t.Fatalf("http get portals 2 failed: %v", err)
	}
	portalID := ""
	for _, p := range portalList.Data {
		if p.Name == portalName {
			portalID = p.ID
			break
		}
	}
	if portalID == "" {
		t.Fatalf("portal id not found for name %q", portalName)
	}

	// GET publication by composite key to verify visibility
	var publication struct {
		Visibility string `json:"visibility"`
	}
	pubPath := "/v3/apis/" + apiID + "/publications/" + portalID
	if err := step1.GetKonnectJSON("get_publication", pubPath, &publication, map[string]any{"api_id": apiID, "portal_id": portalID}); err != nil {
		t.Fatalf("http get publication failed: %v", err)
	}
	if publication.Visibility != "private" {
		t.Fatalf("expected publication visibility=private, got %q", publication.Visibility)
	}
	step1.AppendCheck("PASS: publication visibility updated to private")
	_ = smsAPIName // silence potential unused in future edits

	// Optional small wait for consistency
	time.Sleep(500 * time.Millisecond)
}
