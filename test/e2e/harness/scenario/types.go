//go:build e2e

package scenario

// Scenario is the top-level schema parsed from scenario.yaml.
type Scenario struct {
	BaseInputsPath string            `yaml:"baseInputsPath"`
	LogLevel       string            `yaml:"log-level"`
	Env            map[string]string `yaml:"env"`
	Vars           map[string]any    `yaml:"vars"`
	Defaults       Defaults          `yaml:"defaults"`
	Test           ScenarioTest      `yaml:"test"`
	Steps          []Step            `yaml:"steps"`
}

type Defaults struct {
	Retry Retry `yaml:"retry"`
	Mask  Mask  `yaml:"mask"`
}

type ScenarioTest struct {
	Enabled *bool  `yaml:"enabled"`
	Info    string `yaml:"info"`
}

type Retry struct {
	Attempts      int      `yaml:"attempts"`
	Interval      string   `yaml:"interval"`
	MaxInterval   string   `yaml:"maxInterval"`
	BackoffFactor float64  `yaml:"backoffFactor"`
	Jitter        string   `yaml:"jitter"`
	Only          []string `yaml:"only"`
	Never         []string `yaml:"never"`
}

type Mask struct {
	DropKeys []string `yaml:"dropKeys"`
}

type Step struct {
	Name                 string            `yaml:"name"`
	SkipInputs           bool              `yaml:"skipInputs"`
	InputOverlayDirs     []string          `yaml:"inputOverlayDirs"`
	InputOverlayOpsFiles []string          `yaml:"inputOverlayOpsFiles"`
	InputOverlayOps      []InlineOp        `yaml:"inputOverlayOps"`
	Env                  map[string]string `yaml:"env"`
	Mask                 Mask              `yaml:"mask"`
	Retry                Retry             `yaml:"retry"`
	Commands             []Command         `yaml:"commands"`
}

// InlineOp allows targeted overlay operations to be declared directly in scenario.yaml.
type InlineOp struct {
	File  string         `yaml:"file"`
	Match string         `yaml:"match"`
	Set   map[string]any `yaml:"set"`
}

type Command struct {
	Name         string            `yaml:"name"`
	Run          []string          `yaml:"run"`
	Exec         []string          `yaml:"exec"`
	Stdin        string            `yaml:"stdin"`
	StdinFile    string            `yaml:"stdinFile"`
	Env          map[string]string `yaml:"env"`
	ResetOrg     bool              `yaml:"resetOrg"`
	ResetRegions []string          `yaml:"resetOrgRegions"`
	Workdir      string            `yaml:"workdir"`
	Mask         Mask              `yaml:"mask"`
	Retry        Retry             `yaml:"retry"`
	Assertions   []Assertion       `yaml:"assertions"`
	ExpectFail   *ExpectedFailure  `yaml:"expectFailure"`
	Create       *CreateSpec       `yaml:"create"`
	OutputFormat string            `yaml:"outputFormat"`
	ParseAs      string            `yaml:"parseAs"`
	StdoutFile   string            `yaml:"stdoutFile"`
	RecordVar    *RecordVar        `yaml:"recordVar"`
}

type CreateSpec struct {
	Resource       string            `yaml:"resource"`
	Payload        CreatePayload     `yaml:"payload"`
	ExpectStatus   int               `yaml:"expectStatus"`
	RecordVar      *RecordVar        `yaml:"recordVar"`
	EndpointParams map[string]string `yaml:"endpointParams"`
}

type CreatePayload struct {
	Inline map[string]any `yaml:"inline"`
	File   string         `yaml:"file"`
}

type RecordVar struct {
	Name         string `yaml:"name"`
	ResponsePath string `yaml:"responsePath"`
}

// ExpectedFailure describes the failure conditions that a command is expected to hit.
// When present, the command harness treats a non-zero exit as success if it matches
// the provided expectations.
type ExpectedFailure struct {
	ExitCode *int   `yaml:"exitCode"`
	Contains string `yaml:"contains"`
}

type Assertion struct {
	Name   string       `yaml:"name"`
	Source AssertionSrc `yaml:"source"`
	Select string       `yaml:"select"`
	Expect Expect       `yaml:"expect"`
	Mask   Mask         `yaml:"mask"`
	Retry  Retry        `yaml:"retry"`
}

type AssertionSrc struct {
	Get string `yaml:"get"`
}

type Expect struct {
	File     string         `yaml:"file"`
	Overlays []string       `yaml:"overlays"`
	Fields   map[string]any `yaml:"fields"`
}
