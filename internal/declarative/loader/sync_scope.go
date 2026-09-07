package loader

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"sigs.k8s.io/yaml"
)

func captureSyncScope(content []byte, rs *resources.ResourceSet) error {
	var raw map[string]any
	// Called after strict parsing succeeds; inspect key presence before nested
	// extraction removes the distinction between omitted and empty collections.
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return fmt.Errorf("failed to inspect sync scope: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}
	return rs.EnsureSyncScope().CaptureDeclared(raw)
}
