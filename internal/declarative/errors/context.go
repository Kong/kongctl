package errors

import (
	"fmt"
	"strings"
)

// ResourceContext provides context information for errors
type ResourceContext struct {
	ResourceType string
	ResourceName string
	Namespace    string
	Operation    string
}

// WithResourceContext adds resource context to an error
func WithResourceContext(err error, ctx ResourceContext) error {
	if err == nil {
		return nil
	}

	var contextParts []string
	
	if ctx.Operation != "" {
		contextParts = append(contextParts, fmt.Sprintf("operation: %s", ctx.Operation))
	}
	
	if ctx.ResourceType != "" && ctx.ResourceName != "" {
		if ctx.Namespace != "" && ctx.Namespace != "*" {
			contextParts = append(contextParts, 
				fmt.Sprintf("%s: %s (namespace: %s)", ctx.ResourceType, ctx.ResourceName, ctx.Namespace))
		} else {
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", ctx.ResourceType, ctx.ResourceName))
		}
	}

	if len(contextParts) == 0 {
		return err
	}

	context := strings.Join(contextParts, ", ")
	return fmt.Errorf("%s [%s]: %w", strings.ToLower(ctx.Operation), context, err)
}

// FormatResourceError creates a formatted error message with resource context
func FormatResourceError(operation, resourceType, resourceName, namespace string, err error) error {
	ctx := ResourceContext{
		ResourceType: resourceType,
		ResourceName: resourceName,
		Namespace:    namespace,
		Operation:    operation,
	}
	return WithResourceContext(err, ctx)
}