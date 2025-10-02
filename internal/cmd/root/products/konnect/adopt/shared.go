package adopt

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/labels"
)

type adoptResult struct {
	ResourceType string `json:"resource_type" yaml:"resource_type"`
	ID           string `json:"id" yaml:"id"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace    string `json:"namespace" yaml:"namespace"`
}

func pointerLabelMap(existing map[string]string, namespace string) map[string]*string {
	cloned := make(map[string]string, len(existing)+1)
	for k, v := range existing {
		cloned[k] = v
	}
	cloned[labels.NamespaceKey] = namespace

	result := make(map[string]*string, len(cloned))
	for k, v := range cloned {
		val := v
		result[k] = &val
	}

	return result
}

func stringLabelMap(existing map[string]string, namespace string) map[string]string {
	cloned := make(map[string]string, len(existing)+1)
	for k, v := range existing {
		cloned[k] = v
	}
	cloned[labels.NamespaceKey] = namespace
	return cloned
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
