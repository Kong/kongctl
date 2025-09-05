//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

// Test_Declarative_ExternalPortal verifies that external portal resources work correctly.
// This test demonstrates:
// 1. Platform team creates a portal in one configuration
// 2. Team A references that portal as external and publishes APIs to it
// 3. External reference resolution works without creating duplicate portals
func Test_Declarative_ExternalPortal(t *testing.T) {
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

	// Step 001: apply platform team portal
	step1, err := harness.NewStep(t, cli, "001-apply_platform_portal")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	srcPlatform := filepath.Join("testdata", "declarative", "external", "platform")
	copyDir(t, srcPlatform, step1.InputsDir)

	// Apply platform portal
	applyOut, err := step1.Apply(filepath.Join(step1.InputsDir, "portal.yaml"))
	if err != nil {
		t.Fatalf("apply platform portal failed: %v", err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply platform portal failed changes=%d", applyOut.Summary.Failed)
	}

	// Validate platform portal exists
	var portals []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	platformPortalName := "Platform Shared Portal"
	ok := retry(6, 1500*time.Millisecond, func() bool {
		portals = nil
		if err := step1.GetAndObserve("portals", &portals, map[string]any{"name": platformPortalName}); err != nil {
			return false
		}
		for _, p := range portals {
			if p.Name == platformPortalName {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("expected platform portal %q after platform apply", platformPortalName)
	}
	step1.AppendCheck("PASS: platform portal created successfully")

	// Get platform portal ID for later verification
	var portalList struct {
		Data []struct{ ID, Name string } `json:"data"`
	}
	if err := step1.GetKonnectJSON("get_platform_portal", "/v3/portals", &portalList, map[string]any{"name": platformPortalName}); err != nil {
		t.Fatalf("http get platform portals failed: %v", err)
	}
	platformPortalID := ""
	for _, p := range portalList.Data {
		if p.Name == platformPortalName {
			platformPortalID = p.ID
			break
		}
	}
	if platformPortalID == "" {
		t.Fatalf("platform portal id not found for name %q", platformPortalName)
	}

	// Step 002: apply team A configuration with external portal reference
	step2, err := harness.NewStep(t, cli, "002-apply_team_a_api")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	srcTeamA := filepath.Join("testdata", "declarative", "external", "team-a")
	copyDir(t, srcTeamA, step2.InputsDir)

	// Apply team A API with external portal reference
	applyOut, err = step2.Apply(filepath.Join(step2.InputsDir, "api.yaml"))
	if err != nil {
		t.Fatalf("apply team A API failed: %v", err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply team A API failed changes=%d", applyOut.Summary.Failed)
	}

	// Validate API was created
	var apis []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	testAPIName := "Test API"
	ok = retry(6, 1500*time.Millisecond, func() bool {
		apis = nil
		if err := step2.GetAndObserve("apis", &apis, map[string]any{"name": testAPIName}); err != nil {
			return false
		}
		for _, a := range apis {
			if a.Name == testAPIName {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("expected API %q after team A apply", testAPIName)
	}
	step2.AppendCheck("PASS: team A API created successfully")

	// Get API ID for publication verification
	var apiList struct {
		Data []struct{ ID, Name string } `json:"data"`
	}
	if err := step2.GetKonnectJSON("get_team_a_api", "/v3/apis", &apiList, map[string]any{"name": testAPIName}); err != nil {
		t.Fatalf("http get team A APIs failed: %v", err)
	}
	testAPIID := ""
	for _, a := range apiList.Data {
		if a.Name == testAPIName {
			testAPIID = a.ID
			break
		}
	}
	if testAPIID == "" {
		t.Fatalf("test API id not found for name %q", testAPIName)
	}

	// Step 003: validate external reference worked - publication should exist
	step3, err := harness.NewStep(t, cli, "003-validate_external_reference")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}

	// Verify publication exists and links to correct portal
	var publication struct {
		Visibility string `json:"visibility"`
	}
	pubPath := "/v3/apis/" + testAPIID + "/publications/" + platformPortalID
	if err := step3.GetKonnectJSON("get_api_publication", pubPath, &publication, map[string]any{"api_id": testAPIID, "portal_id": platformPortalID}); err != nil {
		t.Fatalf("http get API publication failed: %v", err)
	}
	if publication.Visibility != "public" {
		t.Fatalf("expected publication visibility=public, got %q", publication.Visibility)
	}
	step3.AppendCheck("PASS: API publication exists with correct portal reference")

	// Verify no duplicate portals were created (should still only have platform portal)
	var finalPortalList struct {
		Data []struct{ ID, Name string } `json:"data"`
	}
	if err := step3.GetKonnectJSON("get_final_portals", "/v3/portals", &finalPortalList, map[string]any{}); err != nil {
		t.Fatalf("http get final portals failed: %v", err)
	}
	portalCount := 0
	for _, p := range finalPortalList.Data {
		if p.Name == platformPortalName {
			portalCount++
		}
	}
	if portalCount != 1 {
		t.Fatalf("expected exactly 1 platform portal, found %d", portalCount)
	}
	step3.AppendCheck("PASS: no duplicate portals created - external reference worked correctly")

	// Optional small wait for consistency
	time.Sleep(500 * time.Millisecond)
}
