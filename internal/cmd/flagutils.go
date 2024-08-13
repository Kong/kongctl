package cmd

import (
	"fmt"
)

type FlagEnum struct {
	Allowed []string
	Value   string
}

func NewEnum(allowed []string, d string) *FlagEnum {
	return &FlagEnum{
		Allowed: allowed,
		Value:   d,
	}
}

func (a FlagEnum) String() string {
	return a.Value
}

func (a *FlagEnum) Set(p string) error {
	isIncluded := func(opts []string, val string) bool {
		for _, opt := range opts {
			if opt == val {
				return true
			}
		}
		return false
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
