//go:build e2e

package scenario

import "testing"

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

	t.Run("enabled env missing", func(t *testing.T) {
		scenario := Scenario{
			Test: ScenarioTest{
				EnabledEnv: "KONGCTL_TEST_OPT_IN",
			},
		}
		want := "skipping: KONGCTL_TEST_OPT_IN not enabled"
		if got := skipScenarioReason(scenario); got != want {
			t.Fatalf("skipScenarioReason(enabledEnv missing) = %q, want %q", got, want)
		}
	})

	t.Run("enabled env present", func(t *testing.T) {
		t.Setenv("KONGCTL_TEST_OPT_IN", "true")
		scenario := Scenario{
			Test: ScenarioTest{
				EnabledEnv: "KONGCTL_TEST_OPT_IN",
			},
		}
		if got := skipScenarioReason(scenario); got != "" {
			t.Fatalf("skipScenarioReason(enabledEnv present) = %q, want empty", got)
		}
	})

	t.Run("required env missing", func(t *testing.T) {
		t.Setenv("KONGCTL_TEST_REQ_A", "1")
		scenario := Scenario{
			Test: ScenarioTest{
				RequiresEnv: []string{"KONGCTL_TEST_REQ_A", "KONGCTL_TEST_REQ_B"},
			},
		}
		want := "skipping: missing required env KONGCTL_TEST_REQ_B"
		if got := skipScenarioReason(scenario); got != want {
			t.Fatalf("skipScenarioReason(required env missing) = %q, want %q", got, want)
		}
	})
}
