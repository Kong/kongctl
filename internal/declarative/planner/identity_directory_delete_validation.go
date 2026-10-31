package planner

import (
	"context"
	"fmt"
)

func (p *Planner) validateIdentityDirectoryDeletes(ctx context.Context, plan *Plan) error {
	for _, change := range plan.Changes {
		if change.ResourceType != ResourceTypeIdentityDirectory || change.Action != ActionDelete {
			continue
		}
		current, err := p.client.GetIdentityDirectoryByID(ctx, change.ResourceID)
		if err != nil {
			return err
		}
		if current == nil {
			continue
		}
		if current.ID != change.ResourceID || current.Name != change.Fields[FieldName] {
			return fmt.Errorf("directory identity changed; regenerate the plan")
		}
		if err := p.client.CheckIdentityDirectoryVaultDependencies(ctx, *current); err != nil {
			return fmt.Errorf("cannot delete directory %q: %w", current.Name, err)
		}
	}
	return nil
}
