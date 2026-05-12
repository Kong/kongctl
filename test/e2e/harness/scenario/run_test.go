//go:build e2e

package scenario

import (
	"strconv"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

func intPtr(v int) *int {
	return &v
}

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

func TestRenderStringReplacesBin(t *testing.T) {
	tmplCtx := map[string]any{
		"bin": "/tmp/kongctl",
	}

	got := renderString("{{ .bin }} plan", tmplCtx)
	if got != "/tmp/kongctl plan" {
		t.Fatalf("renderString() = %q, want %q", got, "/tmp/kongctl plan")
	}
}

func TestParseMaxConcurrencyValues(t *testing.T) {
	got, err := parseMaxConcurrencyValues("1, 2,5")
	if err != nil {
		t.Fatalf("parseMaxConcurrencyValues() error = %v", err)
	}
	want := []int{1, 2, 5}
	if len(got) != len(want) {
		t.Fatalf("parseMaxConcurrencyValues() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseMaxConcurrencyValues()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestParseMaxConcurrencyValuesRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"0", "201", "1,,2", "abc"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseMaxConcurrencyValues(raw); err == nil {
				t.Fatalf("parseMaxConcurrencyValues(%q) expected error", raw)
			}
		})
	}
}

func TestCommandMaxConcurrencyPrecedence(t *testing.T) {
	suite := 2
	cli := &harness.CLI{}
	sc := Scenario{Defaults: Defaults{MaxConcurrency: intPtr(3)}}
	st := Step{MaxConcurrency: intPtr(4)}
	cmd := Command{MaxConcurrency: intPtr(6)}

	got, ok, err := commandMaxConcurrency(cli, sc, st, cmd, nil, &suite)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() error = %v", err)
	}
	if !ok || got != 6 {
		t.Fatalf("commandMaxConcurrency() = (%d, %t), want (6, true)", got, ok)
	}

	cmd.MaxConcurrency = nil
	got, ok, err = commandMaxConcurrency(cli, sc, st, cmd, nil, &suite)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() error = %v", err)
	}
	if !ok || got != 4 {
		t.Fatalf("commandMaxConcurrency() = (%d, %t), want (4, true)", got, ok)
	}

	st.MaxConcurrency = nil
	got, ok, err = commandMaxConcurrency(cli, sc, st, cmd, nil, &suite)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() error = %v", err)
	}
	if !ok || got != 3 {
		t.Fatalf("commandMaxConcurrency() = (%d, %t), want (3, true)", got, ok)
	}

	sc.Defaults.MaxConcurrency = nil
	got, ok, err = commandMaxConcurrency(cli, sc, st, cmd, nil, &suite)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() error = %v", err)
	}
	if !ok || got != 2 {
		t.Fatalf("commandMaxConcurrency() = (%d, %t), want (2, true)", got, ok)
	}
}

func TestCommandMaxConcurrencyDoesNotOverrideConfiguredEnvWithSuiteValue(t *testing.T) {
	suite := 2
	cli := &harness.CLI{
		Env: []string{maxConcurrencyEnvName + "=9"},
	}

	got, ok, err := commandMaxConcurrency(cli, Scenario{}, Step{}, Command{}, nil, &suite)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() error = %v", err)
	}
	if ok {
		t.Fatalf("commandMaxConcurrency() = (%d, %t), want no override", got, ok)
	}

	got, ok, err = commandMaxConcurrency(
		&harness.CLI{},
		Scenario{},
		Step{},
		Command{},
		map[string]string{maxConcurrencyEnvName: "8"},
		&suite,
	)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() with env override error = %v", err)
	}
	if ok {
		t.Fatalf("commandMaxConcurrency() with env override = (%d, %t), want no override", got, ok)
	}
}

func TestCommandMaxConcurrencyYAMLOverridesConfiguredEnv(t *testing.T) {
	suite := 2
	cli := &harness.CLI{
		Env: []string{maxConcurrencyEnvName + "=9"},
	}
	cmd := Command{MaxConcurrency: intPtr(7)}

	got, ok, err := commandMaxConcurrency(cli, Scenario{}, Step{}, cmd, nil, &suite)
	if err != nil {
		t.Fatalf("commandMaxConcurrency() error = %v", err)
	}
	if !ok || got != 7 {
		t.Fatalf("commandMaxConcurrency() = (%d, %t), want (7, true)", got, ok)
	}
}

func TestCommandMaxConcurrencyRejectsInvalidYAML(t *testing.T) {
	for _, value := range []int{0, 201} {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			_, _, err := commandMaxConcurrency(
				&harness.CLI{},
				Scenario{Defaults: Defaults{MaxConcurrency: intPtr(value)}},
				Step{},
				Command{},
				nil,
				nil,
			)
			if err == nil {
				t.Fatalf("commandMaxConcurrency() expected error for %d", value)
			}
		})
	}
}

func TestStableIndexIsDeterministic(t *testing.T) {
	key := "test/e2e/scenarios/portal/sync/scenario.yaml"
	first := stableIndex(key, 4)
	second := stableIndex(key, 4)
	if first != second {
		t.Fatalf("stableIndex() = %d then %d", first, second)
	}
	if first < 0 || first >= 4 {
		t.Fatalf("stableIndex() = %d, want in [0,4)", first)
	}
}
