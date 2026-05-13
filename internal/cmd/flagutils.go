package cmd

import (
	"fmt"
	"slices"
)

type FlagEnum struct {
	Allowed []string
	Value   string
	// deferred=true means Set accepts any string without checking Allowed.
	// Callers must validate the value later (e.g. in a PersistentPreRunE)
	// once the resolved subcommand is known.
	deferred bool
}

func NewEnum(allowed []string, d string) *FlagEnum {
	return &FlagEnum{
		Allowed: allowed,
		Value:   d,
	}
}

// NewDeferredEnum returns a FlagEnum whose Set accepts any string. Use this
// when the allowed set depends on which subcommand cobra resolves to; a
// PersistentPreRunE on the parent should validate the stored Value.
// Allowed is still populated so help-text rendering keeps working.
func NewDeferredEnum(allowed []string, d string) *FlagEnum {
	return &FlagEnum{
		Allowed:  allowed,
		Value:    d,
		deferred: true,
	}
}

func (a FlagEnum) String() string {
	return a.Value
}

func (a *FlagEnum) Set(p string) error {
	if a.deferred {
		a.Value = p
		return nil
	}
	isIncluded := func(opts []string, val string) bool {
		return slices.Contains(opts, val)
	}
	if !isIncluded(a.Allowed, p) {
		return fmt.Errorf("invalid value %q, must be one of %v", p, a.Allowed)
	}
	a.Value = p
	return nil
}

func (a *FlagEnum) Type() string {
	return "string"
}
