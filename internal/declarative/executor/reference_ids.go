package executor

import (
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
)

const unknownReferenceID = "[unknown]"

func hasResolvedReferenceID(id string) bool {
	trimmed := strings.TrimSpace(id)
	return trimmed != "" && trimmed != unknownReferenceID
}

func refIDNeedsResolution(id string) bool {
	return !hasResolvedReferenceID(id)
}

func normalizeUnresolvedReferenceIDs(change *planner.PlannedChange) {
	if change == nil {
		return
	}

	if change.Parent != nil && refIDNeedsResolution(change.Parent.ID) {
		change.Parent.ID = ""
	}

	for key, refInfo := range change.References {
		updated := false

		if refIDNeedsResolution(refInfo.ID) {
			refInfo.ID = ""
			updated = true
		}

		if len(refInfo.ResolvedIDs) > 0 {
			for i, id := range refInfo.ResolvedIDs {
				if refIDNeedsResolution(id) {
					refInfo.ResolvedIDs[i] = ""
					updated = true
				}
			}
		}

		if updated {
			change.References[key] = refInfo
		}
	}
}
