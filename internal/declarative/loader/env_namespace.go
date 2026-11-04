package loader

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/validator"
)

func resolveNamespaceEnvPlaceholders(inputs ...*temporaryParseResult) error {
	nsValidator := validator.NewNamespaceValidator()
	resolved := make(map[string]string)
	resolve := func(namespace *string) error {
		if namespace == nil || !tags.IsEnvPlaceholder(*namespace) {
			return nil
		}
		value, ok := resolved[*namespace]
		if !ok {
			var err error
			value, err = tags.ResolveEnvPlaceholder(*namespace)
			if err != nil {
				return err
			}
			// Validate before ordinary field resolution can interpret a namespace as
			// another placeholder, including in defaults-only files.
			if err := nsValidator.ValidateNamespace(value); err != nil {
				return err
			}
			resolved[*namespace] = value
		}
		*namespace = value
		return nil
	}
	for _, input := range inputs {
		if input.Defaults != nil && input.Defaults.Kongctl != nil {
			if err := resolve(input.Defaults.Kongctl.Namespace); err != nil {
				return fmt.Errorf("_defaults.kongctl.namespace: %w", err)
			}
		}
		if err := input.ForEachNamespaceParticipant(func(participant resources.NamespaceParticipant) error {
			meta := *participant.Meta
			if meta == nil {
				return nil
			}
			if err := resolve(meta.Namespace); err != nil {
				return fmt.Errorf("%s %q kongctl.namespace: %w", participant.Label, participant.Ref, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
