//go:build e2e

package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScenarioMaturity(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    Maturity
		wantErr bool
	}{
		{name: "omitted defaults stable", want: MaturityStable},
		{name: "stable", value: "stable", want: MaturityStable},
		{name: "beta", value: "beta", want: MaturityBeta},
		{name: "whitespace trimmed", value: " beta ", want: MaturityBeta},
		{name: "unknown", value: "experimental", wantErr: true},
		{name: "case sensitive", value: "Beta", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scenarioMaturity(Scenario{Test: ScenarioTest{Maturity: tt.value}})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("scenarioMaturity() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("scenarioMaturity() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("scenarioMaturity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBetaMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    BetaMode
		wantErr bool
	}{
		{name: "omitted defaults fail", want: BetaModeFail},
		{name: "fail", value: "fail", want: BetaModeFail},
		{name: "warn", value: "warn", want: BetaModeWarn},
		{name: "skip", value: "skip", want: BetaModeSkip},
		{name: "whitespace trimmed", value: " warn ", want: BetaModeWarn},
		{name: "unknown", value: "ignore", wantErr: true},
		{name: "case sensitive", value: "WARN", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBetaMode(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseBetaMode() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBetaMode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseBetaMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreflightScenarioBetaModes(t *testing.T) {
	beta := Scenario{Test: ScenarioTest{Maturity: string(MaturityBeta)}}

	for _, mode := range []BetaMode{BetaModeFail, BetaModeWarn} {
		got, err := preflightScenario(beta, mode)
		if err != nil {
			t.Fatalf("preflightScenario(beta, %q) error = %v", mode, err)
		}
		if got.Maturity != MaturityBeta || got.SkipReason != "" {
			t.Fatalf("preflightScenario(beta, %q) = %+v, want runnable beta", mode, got)
		}
	}

	got, err := preflightScenario(beta, BetaModeSkip)
	if err != nil {
		t.Fatalf("preflightScenario(beta, skip) error = %v", err)
	}
	if got.Maturity != MaturityBeta || got.SkipReason == "" {
		t.Fatalf("preflightScenario(beta, skip) = %+v, want skipped beta", got)
	}

	stable, err := preflightScenario(Scenario{}, BetaModeSkip)
	if err != nil {
		t.Fatalf("preflightScenario(stable, skip) error = %v", err)
	}
	if stable.Maturity != MaturityStable || stable.SkipReason != "" {
		t.Fatalf("preflightScenario(stable, skip) = %+v, want runnable stable", stable)
	}
}

func TestRunSkipsBetaBeforeHarnessInitialization(t *testing.T) {
	scenarioPath := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(scenarioPath, []byte("test:\n  maturity: beta\n"), 0o600); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	t.Setenv("KONGCTL_E2E_BIN", filepath.Join(t.TempDir(), "missing-kongctl"))

	returned := false
	t.Run("beta", func(t *testing.T) {
		_, _ = Run(t, scenarioPath, BetaModeSkip)
		returned = true
	})
	if returned {
		t.Fatal("Run() returned normally, want beta scenario skipped before harness initialization")
	}
}

func TestPreflightScenarioValidatesMaturityBeforeSkipGates(t *testing.T) {
	disabled := false
	_, err := preflightScenario(Scenario{
		Test: ScenarioTest{
			Enabled:  &disabled,
			Maturity: "experimental",
		},
	}, BetaModeWarn)
	if err == nil {
		t.Fatal("preflightScenario() error = nil, want invalid maturity error")
	}
}

func TestIsAdvisoryFailure(t *testing.T) {
	for _, tt := range []struct {
		name     string
		maturity Maturity
		mode     BetaMode
		want     bool
	}{
		{name: "stable fail", maturity: MaturityStable, mode: BetaModeFail},
		{name: "stable warn", maturity: MaturityStable, mode: BetaModeWarn},
		{name: "stable skip", maturity: MaturityStable, mode: BetaModeSkip},
		{name: "beta fail", maturity: MaturityBeta, mode: BetaModeFail},
		{name: "beta warn", maturity: MaturityBeta, mode: BetaModeWarn, want: true},
		{name: "beta skip", maturity: MaturityBeta, mode: BetaModeSkip},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAdvisoryFailure(tt.maturity, tt.mode); got != tt.want {
				t.Fatalf("IsAdvisoryFailure(%q, %q) = %v, want %v", tt.maturity, tt.mode, got, tt.want)
			}
		})
	}
}

func TestMissingEnvVars(t *testing.T) {
	t.Setenv("KONGCTL_TEST_ENV_A", "1")
	t.Setenv("KONGCTL_TEST_ENV_B", "")

	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty names ignored",
			input: []string{"", "   "},
			want:  nil,
		},
		{
			name:  "env present",
			input: []string{"KONGCTL_TEST_ENV_A"},
			want:  nil,
		},
		{
			name:  "env empty treated missing",
			input: []string{"KONGCTL_TEST_ENV_B"},
			want:  []string{"KONGCTL_TEST_ENV_B"},
		},
		{
			name:  "mixed envs",
			input: []string{"KONGCTL_TEST_ENV_A", "KONGCTL_TEST_ENV_B", "KONGCTL_TEST_ENV_C"},
			want:  []string{"KONGCTL_TEST_ENV_B", "KONGCTL_TEST_ENV_C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingEnvVars(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("missingEnvVars(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("missingEnvVars(%v) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestTruthyEnvValue(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "", want: false},
		{input: "0", want: false},
		{input: "false", want: false},
		{input: "off", want: false},
		{input: "no", want: false},
		{input: "1", want: true},
		{input: "true", want: true},
		{input: "yes", want: true},
		{input: "on", want: true},
		{input: "Y", want: true},
		{input: "  TrUe  ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := truthyEnvValue(tt.input); got != tt.want {
				t.Fatalf("truthyEnvValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatSkipReason(t *testing.T) {
	tests := []struct {
		name     string
		info     string
		fallback string
		want     string
	}{
		{
			name:     "empty info and fallback",
			info:     "",
			fallback: "",
			want:     "",
		},
		{
			name:     "fallback only",
			info:     "",
			fallback: "scenario disabled",
			want:     "skipping: scenario disabled",
		},
		{
			name:     "info only",
			info:     "Requires Gmail",
			fallback: "",
			want:     "Requires Gmail",
		},
		{
			name:     "info with prefixed fallback",
			info:     "Requires Gmail",
			fallback: "skipping: missing required env FOO",
			want:     "Requires Gmail (skipping: missing required env FOO)",
		},
		{
			name:     "info with unprefixed fallback",
			info:     "Requires Gmail",
			fallback: "missing required env FOO",
			want:     "Requires Gmail (skipping: missing required env FOO)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSkipReason(tt.info, tt.fallback); got != tt.want {
				t.Fatalf("formatSkipReason(%q, %q) = %q, want %q", tt.info, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestSkipScenarioReason(t *testing.T) {
	t.Run("disabled scenario", func(t *testing.T) {
		disabled := false
		scenario := Scenario{
			Test: ScenarioTest{
				Enabled: &disabled,
			},
		}
		want := "skipping: scenario disabled via scenario.yaml"
		if got := skipScenarioReason(scenario); got != want {
			t.Fatalf("skipScenarioReason(disabled) = %q, want %q", got, want)
		}
	})

	t.Run("disabled scenario with info", func(t *testing.T) {
		disabled := false
		scenario := Scenario{
			Test: ScenarioTest{
				Enabled: &disabled,
				Info:    "Requires Gmail credentials",
			},
		}
		want := "Requires Gmail credentials (skipping: scenario disabled via scenario.yaml)"
		if got := skipScenarioReason(scenario); got != want {
			t.Fatalf("skipScenarioReason(disabled info) = %q, want %q", got, want)
		}
	})

	t.Run("enabled by env var missing", func(t *testing.T) {
		scenario := Scenario{
			Test: ScenarioTest{
				EnabledByEnvVar: "KONGCTL_TEST_OPT_IN",
			},
		}
		want := "skipping: KONGCTL_TEST_OPT_IN not enabled"
		if got := skipScenarioReason(scenario); got != want {
			t.Fatalf("skipScenarioReason(enabledByEnvVar missing) = %q, want %q", got, want)
		}
	})

	t.Run("enabled by env var present", func(t *testing.T) {
		t.Setenv("KONGCTL_TEST_OPT_IN", "true")
		scenario := Scenario{
			Test: ScenarioTest{
				EnabledByEnvVar: "KONGCTL_TEST_OPT_IN",
			},
		}
		if got := skipScenarioReason(scenario); got != "" {
			t.Fatalf("skipScenarioReason(enabledByEnvVar present) = %q, want empty", got)
		}
	})

	t.Run("required env missing", func(t *testing.T) {
		t.Setenv("KONGCTL_TEST_REQ_A", "1")
		scenario := Scenario{
			Test: ScenarioTest{
				RequiredEnvVars: []string{"KONGCTL_TEST_REQ_A", "KONGCTL_TEST_REQ_B"},
			},
		}
		want := "skipping: missing required env KONGCTL_TEST_REQ_B"
		if got := skipScenarioReason(scenario); got != want {
			t.Fatalf("skipScenarioReason(required env missing) = %q, want %q", got, want)
		}
	})
}

func TestScenarioRequiresPAT(t *testing.T) {
	t.Run("default requires pat", func(t *testing.T) {
		scenario := Scenario{}
		if got := scenarioRequiresPAT(scenario); !got {
			t.Fatalf("scenarioRequiresPAT(default) = %v, want true", got)
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		requiresPAT := true
		scenario := Scenario{
			Test: ScenarioTest{
				RequiresPAT: &requiresPAT,
			},
		}
		if got := scenarioRequiresPAT(scenario); !got {
			t.Fatalf("scenarioRequiresPAT(true) = %v, want true", got)
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		requiresPAT := false
		scenario := Scenario{
			Test: ScenarioTest{
				RequiresPAT: &requiresPAT,
			},
		}
		if got := scenarioRequiresPAT(scenario); got {
			t.Fatalf("scenarioRequiresPAT(false) = %v, want false", got)
		}
	})
}
