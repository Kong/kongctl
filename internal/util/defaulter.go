package util

import (
	"gopkg.in/yaml.v3"
)

func ApplyDefaults[T any](obj *T) error {
	var withDefaults T

	// 1. Marshal zero-value struct to get defaults
	b, err := yaml.Marshal(new(T))
	if err != nil {
		return err
	}

	// 2. Unmarshal defaults
	if err := yaml.Unmarshal(b, &withDefaults); err != nil {
		return err
	}

	// 3. Overlay user-provided values
	b2, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(b2, &withDefaults); err != nil {
		return err
	}

	// 4. Copy back into original pointer
	*obj = withDefaults
	return nil
}
