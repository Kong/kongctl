package mesh

import (
	"net/http"
	"strings"
	"testing"
)

func TestDecodeDiscoveryResponse(t *testing.T) {
	// Shapes taken from a live control plane: enterprise types report a
	// shortName but no display names, while several policies report display
	// names but no shortName.
	body := []byte(`{"resources":[
		{"name":"AccessAudit","path":"accessaudits","scope":"Global","shortName":"aa",
		 "readOnly":false,"singularDisplayName":"","pluralDisplayName":""},
		{"name":"CircuitBreaker","path":"circuit-breakers","scope":"Mesh","shortName":"",
		 "readOnly":false,"singularDisplayName":"Circuit Breaker","pluralDisplayName":"Circuit Breakers",
		 "policy":{"isTargetRef":false}},
		{"name":"DataplaneInsight","path":"dataplane-insights","scope":"Mesh","readOnly":true,
		 "singularDisplayName":"Dataplane Insight","pluralDisplayName":"Dataplane Insights"}
	]}`)

	descriptors, err := decodeDiscoveryResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descriptors) != 3 {
		t.Fatalf("expected 3 descriptors, got %d", len(descriptors))
	}

	audit := descriptors[0]
	if audit.IsMeshScoped() {
		t.Error("AccessAudit is global scoped and should not be mesh scoped")
	}
	if audit.Alias() != "aa" {
		t.Errorf("expected alias aa, got %q", audit.Alias())
	}
	// Display names are empty on the wire, so both must fall back.
	if audit.Singular() != "AccessAudit" {
		t.Errorf("expected singular to fall back to the type name, got %q", audit.Singular())
	}
	if audit.Plural() != "accessaudits" {
		t.Errorf("expected plural to fall back to the path, got %q", audit.Plural())
	}
	if audit.IsPolicy() {
		t.Error("AccessAudit carries no policy block and is not a policy")
	}

	breaker := descriptors[1]
	if !breaker.IsMeshScoped() {
		t.Error("CircuitBreaker is mesh scoped")
	}
	if breaker.Alias() != "" {
		t.Errorf("expected no alias, got %q", breaker.Alias())
	}
	if breaker.Singular() != "Circuit Breaker" {
		t.Errorf("expected reported singular display name, got %q", breaker.Singular())
	}
	if !breaker.IsPolicy() {
		t.Error("CircuitBreaker carries a policy block and is a policy")
	}

	if !descriptors[2].ReadOnly {
		t.Error("DataplaneInsight is read only")
	}
}

func TestDecodeDiscoveryResponseSkipsPathlessDescriptors(t *testing.T) {
	// Nothing can be addressed without a path, so such entries are dropped
	// rather than surfaced as unusable commands.
	body := []byte(`{"resources":[
		{"name":"Usable","path":"usables","scope":"Global"},
		{"name":"Unaddressable","path":"","scope":"Global"},
		{"name":"Blank","path":"   ","scope":"Mesh"}
	]}`)

	descriptors, err := decodeDiscoveryResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 usable descriptor, got %d", len(descriptors))
	}
	if descriptors[0].Name != "Usable" {
		t.Errorf("expected the descriptor with a path to survive, got %q", descriptors[0].Name)
	}
}

func TestDecodeDiscoveryResponseInvalidJSON(t *testing.T) {
	if _, err := decodeDiscoveryResponse([]byte(`not json`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestDescriptorPaths(t *testing.T) {
	meshScoped := ResourceDescriptor{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh}
	globalScoped := ResourceDescriptor{Name: "Zone", Path: "zones", Scope: ScopeGlobal}

	if got := meshScoped.CollectionPath("prod"); got != "/meshes/prod/dataplanes" {
		t.Errorf("unexpected mesh scoped collection path: %s", got)
	}
	if got := meshScoped.ItemPath("prod", "dp-1"); got != "/meshes/prod/dataplanes/dp-1" {
		t.Errorf("unexpected mesh scoped item path: %s", got)
	}
	// An empty mesh falls back to the default, matching kumactl.
	if got := meshScoped.CollectionPath(""); got != "/meshes/default/dataplanes" {
		t.Errorf("expected the default mesh to be applied, got %s", got)
	}

	if got := globalScoped.CollectionPath("prod"); got != "/zones" {
		t.Errorf("global scoped paths ignore the mesh, got %s", got)
	}
	if got := globalScoped.ItemPath("prod", "zone-1"); got != "/zones/zone-1" {
		t.Errorf("unexpected global scoped item path: %s", got)
	}
}

func TestBuildAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantSubstr string
	}{
		{
			name:       "unauthorized names the credential",
			statusCode: http.StatusUnauthorized,
			wantSubstr: "credential",
		},
		{
			name:       "forbidden names the credential",
			statusCode: http.StatusForbidden,
			wantSubstr: "credential",
		},
		{
			name:       "not found names the version requirement",
			statusCode: http.StatusNotFound,
			wantSubstr: "3.0",
		},
		{
			// The control plane's own wording is preferred over anything
			// kongctl would invent for the status code.
			name:       "AIP-193 envelope is quoted rather than the status",
			statusCode: http.StatusMethodNotAllowed,
			body: `{"type":"/std-errors","status":405,"title":"Method not allowed",` +
				`"detail":"Not allowed on global CP","instance":"abc","details":"Not allowed on global CP"}`,
			wantSubstr: "Not allowed on global CP",
		},
		{
			name:       "envelope validation feedback names the field",
			statusCode: http.StatusBadRequest,
			body: `{"status":400,"title":"Invalid parameters","detail":"validation failed",` +
				`"invalid_parameters":[{"field":"spec.targetRef","reason":"must be set","source":"body"}]}`,
			wantSubstr: "spec.targetRef",
		},
		{
			name:       "other statuses surface the body",
			statusCode: http.StatusInternalServerError,
			body:       "boom",
			wantSubstr: "boom",
		},
		{
			name:       "empty body still reports the status",
			statusCode: http.StatusBadGateway,
			wantSubstr: "502",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := buildAPIError(tc.statusCode, []byte(tc.body))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("expected error %q to contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}
