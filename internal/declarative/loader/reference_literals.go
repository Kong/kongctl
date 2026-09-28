package loader

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
)

func restoreReferenceLiterals(rs *resources.ResourceSet) error {
	for _, resource := range rs.AllResources() {
		if err := values.Transform(resource, func(path, value string) (any, error) {
			literal, ok := tags.ParseReferenceLiteral(value)
			if !ok {
				return value, nil
			}
			rs.AddLiteralSource(resource.GetRef(), path, literal)
			return literal, nil
		}); err != nil {
			return err
		}
	}
	return nil
}
