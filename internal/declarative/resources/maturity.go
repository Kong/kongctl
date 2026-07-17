package resources

import (
	"fmt"

	capmaturity "github.com/kong/kongctl/internal/maturity"
)

// Operation identifies a declarative resource operation with independent maturity.
type Operation string

const (
	OperationRead   Operation = "read"
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
	OperationDelete Operation = "delete"
	OperationAdopt  Operation = "adopt"
)

// Operations returns all supported declarative maturity operations in display order.
func Operations() []Operation {
	return []Operation{
		OperationRead,
		OperationCreate,
		OperationUpdate,
		OperationDelete,
		OperationAdopt,
	}
}

// ResourceRegistrationOption configures metadata stored beside resource registration.
type ResourceRegistrationOption func(*resourceOps) error

// WithMaturity sets the default maturity of a declarative resource type.
func WithMaturity(metadata capmaturity.Metadata) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if err := capmaturity.Validate(metadata); err != nil {
			return err
		}
		ops.maturity = new(metadata)
		return nil
	}
}

// WithOperationMaturity sets a maturity override for a declarative resource operation.
func WithOperationMaturity(
	operation Operation,
	metadata capmaturity.Metadata,
) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if err := validateOperation(operation); err != nil {
			return err
		}
		if err := capmaturity.Validate(metadata); err != nil {
			return err
		}
		if ops.operationMaturity == nil {
			ops.operationMaturity = make(map[Operation]capmaturity.Metadata)
		}
		ops.operationMaturity[operation] = metadata
		return nil
	}
}

// MaturityFor resolves a resource's default maturity or one operation's maturity.
// With no operation it resolves the resource. Exactly one operation may be supplied.
func MaturityFor(resourceType ResourceType, operations ...Operation) (capmaturity.Resolution, error) {
	ops, ok := registry[resourceType]
	if !ok {
		return capmaturity.Resolution{}, fmt.Errorf("resource type %q is not registered", resourceType)
	}
	if len(operations) > 1 {
		return capmaturity.Resolution{}, fmt.Errorf("at most one resource operation may be resolved")
	}

	resolved := capmaturity.DefaultResolution()
	resolved = capmaturity.ResolveDeclaration(resolved, ops.maturity, capmaturity.Source{
		Kind: capmaturity.KindResource,
		Path: string(resourceType),
	})
	if len(operations) == 0 {
		resolved.Declared = ops.maturity
		return resolved, nil
	}

	operation := operations[0]
	if err := validateOperation(operation); err != nil {
		return capmaturity.Resolution{}, err
	}
	var declared *capmaturity.Metadata
	if metadata, ok := ops.operationMaturity[operation]; ok {
		declared = new(metadata)
	}
	return capmaturity.ResolveDeclaration(resolved, declared, capmaturity.Source{
		Kind: capmaturity.KindOperation,
		Path: string(resourceType),
		Name: string(operation),
	}), nil
}

// ExplainMaturity is the machine-readable resource maturity schema extension.
type ExplainMaturity struct {
	Level        capmaturity.Level               `json:"level"                   yaml:"level"`
	Message      string                          `json:"message,omitempty"       yaml:"message,omitempty"`
	ReferenceURL string                          `json:"reference_url,omitempty" yaml:"reference_url,omitempty"`
	Operations   map[string]capmaturity.Metadata `json:"operations"              yaml:"operations"`
}

func explainMaturityFor(resourceType ResourceType) (ExplainMaturity, error) {
	resource, err := MaturityFor(resourceType)
	if err != nil {
		return ExplainMaturity{}, err
	}
	result := ExplainMaturity{
		Level:        resource.Effective.Level,
		Message:      resource.Effective.Message,
		ReferenceURL: resource.Effective.ReferenceURL,
		Operations:   make(map[string]capmaturity.Metadata),
	}
	for _, operation := range Operations() {
		resolved, err := MaturityFor(resourceType, operation)
		if err != nil {
			return ExplainMaturity{}, err
		}
		if resolved.Effective.Level != resource.Effective.Level {
			result.Operations[string(operation)] = resolved.Effective
		}
	}
	return result, nil
}

func validateOperation(operation Operation) error {
	switch operation {
	case OperationRead, OperationCreate, OperationUpdate, OperationDelete, OperationAdopt:
		return nil
	default:
		return fmt.Errorf("unsupported resource operation %q", operation)
	}
}
