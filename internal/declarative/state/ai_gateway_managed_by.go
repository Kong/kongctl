package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
)

// Stamp at the actual write boundary so saved plans record the executing
// binary, including direct SDK requests and auxiliary secret updates.
func stampAIGatewayManagedBy[T any](ctx context.Context, request *T) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway request for managed_by metadata")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil || payload == nil {
		return fmt.Errorf("AI Gateway request must be an object for managed_by metadata")
	}
	var metadata map[string]string
	if raw := payload["managed_by"]; raw != nil {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return fmt.Errorf("managed_by must contain string values")
		}
	}
	metadata = resources.ManagedByWithProvenance(metadata, build.Version(ctx))
	if err := resources.ValidateChildManagedBy(metadata, false); err != nil {
		return err
	}
	payload["managed_by"], err = json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to encode managed_by metadata")
	}
	data, err = json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway request")
	}
	var updated T
	if err := json.Unmarshal(data, &updated); err != nil {
		return fmt.Errorf("failed to map managed_by metadata into AI Gateway request")
	}
	*request = updated
	return nil
}
