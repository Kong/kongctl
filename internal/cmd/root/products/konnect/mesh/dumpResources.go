package mesh

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"sigs.k8s.io/yaml"
)

// Export profiles, matching kumactl export so that a customer's existing
// choice of profile keeps its meaning.
const (
	// ProfileFederation carries what a new global control plane needs, without
	// the targetRef policies.
	ProfileFederation = "federation"
	// ProfileFederationWithPolicies adds those policies.
	ProfileFederationWithPolicies = "federation-with-policies"
	// ProfileAll exports every type the control plane serves.
	ProfileAll = "all"
	// ProfileNoDataplanes exports everything except dataplanes and their
	// insights, which a target control plane rebuilds for itself.
	ProfileNoDataplanes = "no-dataplanes"
)

// Profiles lists the supported export profiles, for flag help and validation.
var Profiles = []string{
	ProfileAll, ProfileFederation, ProfileFederationWithPolicies, ProfileNoDataplanes,
}

// Global secrets that must not be exported: re-applying them elsewhere breaks
// TLS between control planes.
var excludedGlobalSecrets = []string{"envoy-admin-ca", "inter-cp-ca"}

// userTokenSigningKeyPrefix names the global secrets that must be applied last.
// Applying a user token signing key invalidates the credential in use, so
// anything after it in the stream would fail to apply.
const userTokenSigningKeyPrefix = "user-token-signing-key"

// Type paths whose ordering matters when the export is applied back.
const (
	meshesPath       = "meshes"
	secretsPath      = "secrets"
	globalSecretPath = "globalsecrets"
)

// runDumpResources serves `dump mesh`, writing every resource on the control
// plane as a YAML stream that `create mesh -f` can apply back.
//
// Which types are exported comes from /_resources — chiefly its
// includeInFederation flag — so a type added by a newer Kong Mesh release is
// exported without a kongctl change.
func runDumpResources(helper cmd.Helper, profile string) error {
	if !slices.Contains(Profiles, profile) {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("invalid profile %q; available profiles are %s",
				profile, strings.Join(Profiles, ", ")),
		}
	}

	descriptors, err := Discover(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve mesh resource types", err, helper.GetCmd())
	}

	selected := selectTypesToDump(descriptors, profile)

	meshNames, err := listMeshNames(helper)
	if err != nil {
		return err
	}

	// Ordered so the stream can be applied back as-is: meshes first, then the
	// resources that depend on them, then the signing keys.
	var first, middle, last []map[string]any
	for _, descriptor := range selected {
		items, err := collectResources(helper, descriptor, meshNames)
		if err != nil {
			return cmd.PrepareExecutionError(
				fmt.Sprintf("failed to export %s", descriptor.Plural()), err, helper.GetCmd())
		}

		for _, item := range items {
			switch bucket := classifyForDump(descriptor, item); bucket {
			case bucketFirst:
				first = append(first, item)
			case bucketLast:
				last = append(last, item)
			case bucketMiddle:
				middle = append(middle, item)
			case bucketSkip:
			}
		}
	}

	ordered := slices.Concat(first, middle, last)
	return writeYAMLStream(helper, ordered)
}

// selectTypesToDump narrows the discovered types by profile, then orders them
// so that a mesh is applied before anything inside it.
func selectTypesToDump(descriptors []ResourceDescriptor, profile string) []ResourceDescriptor {
	selected := make([]ResourceDescriptor, 0, len(descriptors))

	for _, descriptor := range descriptors {
		switch profile {
		case ProfileFederation:
			if !descriptor.IncludeInFederation {
				continue
			}
			// The targetRef policies are the ones a federated zone keeps for
			// itself, so the plain federation profile leaves them behind.
			if descriptor.Policy != nil && descriptor.Policy.IsTargetRef {
				continue
			}
		case ProfileFederationWithPolicies:
			if !descriptor.IncludeInFederation {
				continue
			}
		case ProfileNoDataplanes:
			if descriptor.Name == "Dataplane" || descriptor.Name == "DataplaneInsight" {
				continue
			}
		}
		selected = append(selected, descriptor)
	}

	sort.SliceStable(selected, func(i, j int) bool {
		return dumpPriority(selected[i]) < dumpPriority(selected[j])
	})
	return selected
}

// dumpPriority orders types for re-apply, matching kumactl export.
func dumpPriority(descriptor ResourceDescriptor) int {
	switch descriptor.Path {
	case meshesPath:
		return 0
	case secretsPath:
		return 1
	case globalSecretPath:
		return 99
	default:
		return 50
	}
}

// collectResources lists every instance of a type, visiting each mesh in turn
// for mesh scoped types so the export covers the whole control plane.
func collectResources(
	helper cmd.Helper, descriptor ResourceDescriptor, meshNames []string,
) ([]map[string]any, error) {
	if !descriptor.IsMeshScoped() {
		return listAll(helper, descriptor.CollectionPath(""))
	}

	var items []map[string]any
	for _, mesh := range meshNames {
		page, err := listAll(helper, descriptor.CollectionPath(mesh))
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
	}
	return items, nil
}

// listMeshNames reads the meshes an export has to walk.
func listMeshNames(helper cmd.Helper) ([]string, error) {
	items, err := listAll(helper, "/meshes")
	if err != nil {
		return nil, cmd.PrepareExecutionError("failed to list meshes", err, helper.GetCmd())
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		if name := stringField(item, "name"); name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// dumpPageSize is how many resources are requested per call. The control plane
// caps its own page size, so this is a request rather than a guarantee.
const dumpPageSize = 100

// listAll pages through a collection and returns every item.
//
// The `next` link in the response cannot be followed: on Konnect hosted
// control planes it carries an internal cluster address. Pagination is
// therefore driven by constructing offset and size directly.
func listAll(helper cmd.Helper, path string) ([]map[string]any, error) {
	var items []map[string]any
	offset := 0

	for {
		query := url.Values{}
		query.Set("size", fmt.Sprint(dumpPageSize))
		if offset > 0 {
			query.Set("offset", fmt.Sprint(offset))
		}

		body, err := fetch(helper, path+"?"+query.Encode())
		if err != nil {
			return nil, err
		}

		var envelope listEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("failed to decode %s: %w", path, err)
		}

		items = append(items, envelope.Items...)

		// Termination is driven by the reported total, not by a short page:
		// the control plane caps its own page size, so a page smaller than
		// the one requested is normal and says nothing about being the last.
		// A page that returns nothing ends the loop regardless, so a control
		// plane that keeps offering a next link cannot spin here.
		if len(envelope.Items) == 0 || len(items) >= envelope.Total {
			return items, nil
		}
		offset += len(envelope.Items)
	}
}

// writeYAMLStream writes the resources as a multi document YAML stream, the
// form kumactl export produced and `create mesh -f` reads back.
func writeYAMLStream(helper cmd.Helper, items []map[string]any) error {
	out := helper.GetStreams().Out

	for _, item := range items {
		encoded, err := yaml.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to encode %s %s: %w",
				stringField(item, "type"), stringField(item, "name"), err)
		}
		if _, err := fmt.Fprintf(out, "---\n%s", encoded); err != nil {
			return err
		}
	}
	return nil
}

// dumpBucket is where a resource lands in the exported stream. The stream has
// to be applicable in order, so position is part of the export's correctness.
type dumpBucket int

const (
	// bucketFirst is applied before anything else.
	bucketFirst dumpBucket = iota
	// bucketMiddle is the bulk of the export.
	bucketMiddle
	// bucketLast is applied after everything else.
	bucketLast
	// bucketSkip is not exported at all.
	bucketSkip
)

// classifyForDump decides where one resource belongs in the stream, and
// mutates the mesh document where an export has to differ from what was read.
func classifyForDump(descriptor ResourceDescriptor, item map[string]any) dumpBucket {
	switch descriptor.Path {
	case meshesPath:
		// A mesh that recreated its default policies on apply would conflict
		// with the policies exported alongside it.
		item["skipCreatingInitialPolicies"] = []string{"*"}
		return bucketFirst

	case globalSecretPath:
		name := stringField(item, "name")
		// These secure control plane to control plane traffic. Carrying them
		// to another control plane breaks its TLS handshakes.
		if slices.Contains(excludedGlobalSecrets, name) {
			return bucketSkip
		}
		// Applying a user token signing key invalidates the credential in
		// use, so anything after it in the stream would fail to apply.
		if strings.HasPrefix(name, userTokenSigningKeyPrefix) {
			return bucketLast
		}
		return bucketMiddle

	default:
		return bucketMiddle
	}
}
