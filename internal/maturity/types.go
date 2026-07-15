package maturity

import "fmt"

// Level identifies the release maturity of a capability.
type Level string

const (
	LevelGA          Level = "ga"
	LevelBeta        Level = "beta"
	LevelTechPreview Level = "tech-preview"
)

// Metadata describes the maturity of a capability.
type Metadata struct {
	Level        Level  `json:"level"                   yaml:"level"`
	Message      string `json:"message,omitempty"       yaml:"message,omitempty"`
	ReferenceURL string `json:"reference_url,omitempty" yaml:"reference_url,omitempty"`
}

// Kind identifies the kind of capability that supplied effective maturity.
type Kind string

const (
	KindDefault       Kind = "default"
	KindCommand       Kind = "command"
	KindFlag          Kind = "flag"
	KindArgument      Kind = "argument"
	KindFlagValue     Kind = "flag-value"
	KindArgumentValue Kind = "argument-value"
	KindResource      Kind = "resource"
	KindOperation     Kind = "resource-operation"
)

// Source identifies the declaration that supplied effective maturity.
type Source struct {
	Kind  Kind   `json:"kind"`
	Path  string `json:"path,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// Resolution contains both the local declaration and resolved maturity.
type Resolution struct {
	Declared  *Metadata `json:"declared,omitempty"`
	Effective Metadata  `json:"effective"`
	Source    Source    `json:"source"`
}

// Validate verifies that metadata uses a supported maturity level.
func Validate(metadata Metadata) error {
	switch metadata.Level {
	case LevelGA, LevelBeta, LevelTechPreview:
		return nil
	default:
		return fmt.Errorf("unsupported maturity level %q", metadata.Level)
	}
}

// DisplayName returns the user-facing name for a maturity level.
func (level Level) DisplayName() string {
	switch level {
	case LevelGA:
		return "GA"
	case LevelBeta:
		return "Beta"
	case LevelTechPreview:
		return "Tech Preview"
	default:
		return string(level)
	}
}

// LessThan reports whether level is less mature than other.
func (level Level) LessThan(other Level) bool {
	return level.rank() < other.rank()
}

func (level Level) rank() int {
	switch level {
	case LevelGA:
		return 3
	case LevelBeta:
		return 2
	case LevelTechPreview:
		return 1
	default:
		return 0
	}
}

func defaultResolution() Resolution {
	return Resolution{
		Effective: Metadata{Level: LevelGA},
		Source:    Source{Kind: KindDefault, Path: "default"},
	}
}

func resolveDeclaration(parent Resolution, declared *Metadata, source Source) Resolution {
	result := Resolution{
		Declared:  declared,
		Effective: parent.Effective,
		Source:    parent.Source,
	}
	if declared == nil {
		return result
	}
	if declared.Level.rank() <= parent.Effective.Level.rank() {
		result.Effective = *declared
		result.Source = source
	}
	return result
}
