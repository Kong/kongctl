package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
)

type childAdoptionMetadataKey struct{}

// WithChildAdoptionMetadata preserves unrelated metadata during an ownership-only
// AI Gateway child adoption. Declarative writes retain their canonical map contract.
func WithChildAdoptionMetadata(ctx context.Context) context.Context {
	return context.WithValue(ctx, childAdoptionMetadataKey{}, true)
}

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
	current := metadata
	metadata, err = resources.PrepareManagedBy(kind, metadata, build.Version(ctx))
	if err != nil {
		return err
	}
	if preserve, _ := ctx.Value(childAdoptionMetadataKey{}).(bool); preserve {
		if !resources.HasIndependentChildOwnership(kind) {
			return fmt.Errorf("child adoption metadata requires independent child ownership")
		}
		for key, value := range current {
			if !strings.HasPrefix(key, "kongctl-") && key != resources.ManagedByToolKey && key != resources.ManagedByVersionKey {
				metadata[key] = value
			}
		}
		if err := resources.ValidateChildManagedBy(metadata, false); err != nil {
			return err
		}
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
	decode := func(data []byte) (any, error) {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var value any
		err := decoder.Decode(&value)
		return value, err
	}
	expectedPayload, err := decode(data)
	if err != nil {
		return fmt.Errorf("failed to verify original managed_by request")
	}
	observedPayload, err := decode(verified)
	if err != nil || !reflect.DeepEqual(expectedPayload, observedPayload) {
		return fmt.Errorf("SDK request did not preserve payload while stamping managed_by metadata")
	}

	*request = updated
	return nil
}
