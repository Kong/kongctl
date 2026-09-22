package dump

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
)

type childDumpContext struct {
	logger     *slog.Logger
	client     *declstate.Client
	parentID   string
	parentName string
}

type childCollector[P any] struct {
	kind    declresources.ResourceType
	warning string
	collect func(context.Context, *childDumpContext, *P) error
	// These kinds are exported inside this collector, not by separate parent calls.
	nested []declresources.ResourceType
}

func childCollection[P, R any, RPtr interface {
	*R
	declresources.Resource
}](
	warning string,
	collect func(context.Context, *childDumpContext) ([]R, error),
	destination func(*P) *[]R,
	nested ...declresources.ResourceType,
) childCollector[P] {
	if collect == nil || destination == nil || strings.TrimSpace(warning) == "" {
		panic("child dump requires collection, storage, and a warning")
	}
	return childCollector[P]{
		kind:    RPtr(new(R)).GetType(),
		warning: warning,
		nested:  nested,
		collect: func(ctx context.Context, d *childDumpContext, parent *P) error {
			values, err := collect(ctx, d)
			if err != nil {
				return err
			}
			if len(values) > 0 {
				*destination(parent) = values
			}
			return nil
		},
	}
}

// Derive completeness from registered ownership, including grandchildren.
// SyncCollections validates owner chains before returning this metadata.
func validateChildCollectors[P any, PPtr interface {
	*P
	declresources.Resource
}](consumer string, collectors []childCollector[P]) error {
	root := PPtr(new(P)).GetType()
	parents := make(map[declresources.ResourceType]declresources.ResourceType)
	for _, collection := range declresources.SyncCollections() {
		parents[collection.ResourceType] = collection.ParentType
	}
	if parent, ok := parents[root]; !ok || parent != "" {
		return fmt.Errorf("%s requires a managed root, got %s", consumer, root)
	}
	seen := make(map[declresources.ResourceType]bool)
	record := func(kind, owner declresources.ResourceType) error {
		if parents[kind] != owner {
			return fmt.Errorf("%s: %s is not a managed child of %s", consumer, kind, owner)
		}
		if seen[kind] {
			return fmt.Errorf("%s registers %s more than once", consumer, kind)
		}
		seen[kind] = true
		return nil
	}
	for _, collector := range collectors {
		if collector.collect == nil || strings.TrimSpace(collector.warning) == "" {
			return fmt.Errorf("%s requires a collector and warning for %s", consumer, collector.kind)
		}
		if err := record(collector.kind, root); err != nil {
			return err
		}
		for _, kind := range collector.nested {
			if err := record(kind, collector.kind); err != nil {
				return err
			}
		}
	}
	var missing []declresources.ResourceType
	for kind := range parents {
		for owner := parents[kind]; owner != ""; owner = parents[owner] {
			if owner == root {
				if !seen[kind] {
					missing = append(missing, kind)
				}
				break
			}
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		return fmt.Errorf("%s is missing resource types %v", consumer, missing)
	}
	return nil
}
