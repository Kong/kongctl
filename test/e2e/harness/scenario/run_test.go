//go:build e2e

package scenario

import "testing"

func TestMaybeRecordVarTemplatesResponsePath(t *testing.T) {
	sc := &Scenario{
		Vars: map[string]any{
			"portalName": "Platform Shared Portal",
		},
	}
	tmplCtx := map[string]any{
		"vars": sc.Vars,
	}
	parsed := []any{
		map[string]any{
			"name": "Platform Shared Portal",
			"id":   "portal-123",
		},
	}

	err := maybeRecordVar(
		sc,
		&RecordVar{
			Name:         "platformPortalID",
			ResponsePath: "[?name=='{{ .vars.portalName }}'] | [0].id",
		},
		parsed,
		tmplCtx,
		nil,
	)
	if err != nil {
		t.Fatalf("maybeRecordVar() error = %v", err)
	}

	if got := sc.Vars["platformPortalID"]; got != "portal-123" {
		t.Fatalf("recorded var = %v, want portal-123", got)
	}
}
