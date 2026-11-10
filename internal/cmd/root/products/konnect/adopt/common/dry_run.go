package common

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/segmentio/cli"
)

type dryRunKey struct{}

// AdoptionPreview is an ownership-only report, separate from declarative plans.
type AdoptionPreview struct {
	DryRun  bool             `json:"dry_run" yaml:"dry_run"`
	Changes []AdoptionChange `json:"changes" yaml:"changes"`
}

type AdoptionChange struct {
	AdoptResult      `yaml:",inline"`
	Parent           map[string]string `json:"parent,omitempty" yaml:"parent,omitempty"`
	MetadataLocation string            `json:"metadata_location" yaml:"metadata_location"`
	Before           map[string]string `json:"before" yaml:"before"`
	After            map[string]string `json:"after" yaml:"after"`
}

// PreviewAdoption is called after validation and immediately before mutation.
// It returns a result without issuing the write when dry-run is enabled.
func PreviewAdoption(ctx context.Context, kind, id, name, namespace string, parent map[string]string,
	location string, before, after map[string]string,
) (*AdoptResult, bool) {
	if ctx == nil {
		return nil, false
	}
	preview, ok := ctx.Value(dryRunKey{}).(*AdoptionPreview)
	if !ok {
		return nil, false
	}
	// Label adoption merges reserved entries; existing labels are not removed.
	if location == "labels" {
		preserved := make(map[string]string)
		maps.Copy(preserved, before)
		maps.Copy(preserved, after)
		after = preserved
	}
	filter := func(values map[string]string) map[string]string {
		result := make(map[string]string)
		for key, value := range values {
			if location != "labels" || strings.HasPrefix(key, "KONGCTL-") {
				result[key] = value
			}
		}
		return result
	}
	result := AdoptResult{ResourceType: kind, ID: id, Name: name, Namespace: namespace}
	preview.Changes = append(preview.Changes, AdoptionChange{
		AdoptResult: result, Parent: maps.Clone(parent), MetadataLocation: location,
		Before: filter(before),
		After:  filter(after),
	})
	return &result, true
}

func PrintDryRun(helper cmdpkg.Helper, outType cmdCommon.OutputFormat) (bool, error) {
	ctx := helper.GetContext()
	if ctx == nil {
		return false, nil
	}
	preview, ok := ctx.Value(dryRunKey{}).(*AdoptionPreview)
	if !ok {
		return false, nil
	}
	if outType != cmdCommon.TEXT {
		printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
		if err != nil {
			return true, err
		}
		defer printer.Flush()
		printer.Print(preview)
		return true, nil
	}
	out := helper.GetStreams().Out
	fmt.Fprintln(out, "Dry run: no resources will be changed.")
	for _, change := range preview.Changes {
		fmt.Fprintf(out,
			"Would adopt %s %q (%s) into namespace %q\n",
			change.ResourceType,
			change.Name,
			change.ID,
			change.Namespace)
		for _, key := range slices.Sorted(maps.Keys(change.Parent)) {
			fmt.Fprintf(out, "  %s: %s\n", key, change.Parent[key])
		}
		fmt.Fprintf(out, "  %s:\n", change.MetadataLocation)
		keys := maps.Clone(change.Before)
		maps.Copy(keys, change.After)
		for _, key := range slices.Sorted(maps.Keys(keys)) {
			before, presentBefore := change.Before[key]
			after, presentAfter := change.After[key]
			if presentBefore == presentAfter && before == after {
				continue
			}
			oldValue, newValue := "<absent>", "<removed>"
			if presentBefore {
				oldValue = fmt.Sprintf("%q", before)
			}
			if presentAfter {
				newValue = fmt.Sprintf("%q", after)
			}
			fmt.Fprintf(out, "    %s: %s -> %s\n", key, oldValue, newValue)
		}
	}
	return true, nil
}
