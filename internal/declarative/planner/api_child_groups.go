package planner

// apiChildGroupActions retains typed collection preparation and builders in
// the caller. A parent CREATE may be followed by existing-parent reconciliation.
type apiChildGroupActions struct {
	create    func(namespace, changeID string)
	reconcile func(namespace, apiID string) error
}

// planExtractedAPIChildren groups by the literal parent reference, preserving
// input order within groups and unspecified map traversal order between them.
// Prepare runs before scanning the plan, including document lookup construction.
func planExtractedAPIChildren[D any](
	p *Planner,
	desired []D,
	parentRef func(D) string,
	prepare func(string, []D) apiChildGroupActions,
	plan *Plan,
) error {
	byAPI := make(map[string][]D)
	for _, child := range desired {
		ref := parentRef(child)
		byAPI[ref] = append(byAPI[ref], child)
	}
	for apiRef, children := range byAPI {
		actions := prepare(apiRef, children)
		apiID := ""
		for _, change := range plan.Changes {
			if change.ResourceType != ResourceTypeAPI || change.ResourceRef != apiRef {
				continue
			}
			if change.Action == ActionCreate {
				namespace := change.Namespace
				if namespace == "" {
					namespace = DefaultNamespace
				}
				actions.create(namespace, change.ID)
				// A parent CREATE does not stop the scan: a later non-create change
				// supplies the parent ID. An empty ID still uses the resolved-ID fallback.
				continue
			}
			apiID = change.ResourceID
			break
		}
		if apiID == "" {
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					apiID = api.GetKonnectID()
					break
				}
			}
		}
		if apiID != "" {
			namespace := DefaultNamespace
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					if api.Kongctl != nil && api.Kongctl.Namespace != nil {
						namespace = *api.Kongctl.Namespace
					}
					break
				}
			}
			if err := actions.reconcile(namespace, apiID); err != nil {
				return err
			}
		}
	}
	return nil
}
