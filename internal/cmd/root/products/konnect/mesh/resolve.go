package mesh

import (
	"fmt"
	"slices"
	"strings"
)

// ResolveType finds the resource type an operator named on the command line.
//
// A type is addressable by its URL path ("dataplanes"), its Kuma type name
// ("Dataplane"), or its control-plane-reported short name ("dp"). All three
// come from discovery, so a type added by a newer Kong Mesh release is
// addressable without a kongctl change.
//
// Matching is case insensitive because the path is lower case while the type
// name is CamelCase, and operators should not have to remember which is which.
func ResolveType(descriptors []ResourceDescriptor, arg string) (ResourceDescriptor, error) {
	wanted := strings.ToLower(strings.TrimSpace(arg))
	if wanted == "" {
		return ResourceDescriptor{}, fmt.Errorf("no resource type given")
	}

	// Path first: it is what the tables and help text display, so it is the
	// form an operator is most likely to have copied.
	for _, d := range descriptors {
		if strings.ToLower(d.Path) == wanted {
			return d, nil
		}
	}
	for _, d := range descriptors {
		if strings.ToLower(d.Name) == wanted {
			return d, nil
		}
	}
	for _, d := range descriptors {
		if alias := d.Alias(); alias != "" && strings.ToLower(alias) == wanted {
			return d, nil
		}
	}

	return ResourceDescriptor{}, unknownTypeError(descriptors, arg)
}

// unknownTypeError reports an unmatched type, offering near misses so an
// operator can correct a typo without listing every type on the control plane.
func unknownTypeError(descriptors []ResourceDescriptor, arg string) error {
	wanted := strings.ToLower(strings.TrimSpace(arg))

	// A shared prefix catches the realistic mistakes — a missing or extra
	// plural, a typo in the tail — where substring matching alone does not.
	var near []string
	for _, d := range descriptors {
		path := strings.ToLower(d.Path)
		if strings.Contains(path, wanted) || strings.Contains(wanted, path) ||
			commonPrefixLen(path, wanted) >= minNearMissPrefix {
			near = append(near, d.Path)
		}
	}
	slices.Sort(near)
	near = slices.Compact(near)

	if len(near) > 0 {
		return fmt.Errorf(
			"unknown mesh resource type %q; did you mean %s? "+
				"run 'get mesh resource-types' to list every type this control plane serves",
			arg, strings.Join(near, ", "))
	}
	return fmt.Errorf(
		"unknown mesh resource type %q on this control plane; "+
			"run 'get mesh resource-types' to list every type it serves", arg)
}

// minNearMissPrefix is how many leading characters two type names must share
// before one is offered as a correction for the other.
const minNearMissPrefix = 4

func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
