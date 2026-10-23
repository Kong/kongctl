package state

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
)

// Stamp at the actual write boundary so saved plans record the executing
// binary, including direct SDK requests and auxiliary secret updates.
func stampManagedByRequest[T any](ctx context.Context, kind resources.ResourceType, request *T) error {
	if !resources.SupportsManagedBy(kind) {
		return nil
	}
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode resource request for managed_by metadata")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil || payload == nil {
		return fmt.Errorf("resource request must be an object for managed_by metadata")
	}
	var metadata map[string]string
	if raw := payload["managed_by"]; raw != nil {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return fmt.Errorf("managed_by must contain string values")
		}
	}
	metadata, err = resources.PrepareManagedBy(kind, metadata, build.Version(ctx))
	if err != nil {
		return err
	}
	payload["managed_by"], err = json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to encode managed_by metadata")
	}
	data, err = json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode resource request")
	}
	var updated T
	if err := json.Unmarshal(data, &updated); err != nil {
		return fmt.Errorf("failed to map managed_by metadata into resource request")
	}
	// Fail closed if a newly enrolled SDK request silently discards metadata.
	verified, err := json.Marshal(updated)
	if err != nil {
		return fmt.Errorf("failed to verify managed_by request")
	}
	var observed map[string]json.RawMessage
	if err := json.Unmarshal(verified, &observed); err != nil {
		return fmt.Errorf("failed to verify managed_by request")
	}
	var retained map[string]string
	if err := json.Unmarshal(observed["managed_by"], &retained); err != nil || !maps.Equal(metadata, retained) {
		return fmt.Errorf("SDK request did not preserve managed_by metadata")
	}
	*request = updated
	return nil
}
