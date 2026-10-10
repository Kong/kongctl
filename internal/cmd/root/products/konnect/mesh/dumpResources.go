package mesh

import (
	"cmp"
	"fmt"
	"slices"
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

// runDumpResources serves `dump mesh`, writing selected resources as a YAML
// stream. Federation profiles prepare resources for migration; other profiles
// may include read-only resources and are not directly applicable.
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
				fmt.Sprintf("failed to export %s", descriptor.Plural()), err, helper.GetCmd(),
			)
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
	for _, item := range ordered {
		removeFederationLabels(profile, item)
	}
	return writeYAMLStream(helper, ordered)
}

// originLabel records which control plane a resource was created on. It is
// immutable once set.
const originLabel = "kuma.io/origin"

// zoneLabel records the zone a resource was synced up from. A global control
// plane rejects it outright: "kuma.io/zone is not allowed on a global control
// plane".
const zoneLabel = "kuma.io/zone"

// Match Kuma's control-plane-owned label registry, retaining kuma.io/mesh so
// a resource remains associated with its mesh on import.
var federationExcludedLabels = []string{
	originLabel, zoneLabel, "kuma.io/policy-role", "kuma.io/display-name",
	"kuma.io/env", "k8s.kuma.io/namespace", "k8s.kuma.io/service-account",
	"kuma.io/listener-zoneingress", "kuma.io/listener-zoneegress",
	"kuma.io/managed-by", "kuma.io/deletion-grace-period-started-at",
	"k8s.kuma.io/service-name", "k8s.kuma.io/is-headless-service",
}

// removeFederationLabels drops the origin and zone labels from a federation
// export.
//
// A federation export seeds a new global control plane. A resource that still
// carries `kuma.io/origin: zone` would be imported there as a zone resource,
// and one that still carries `kuma.io/zone` is refused by the destination's
// validator before it is stored at all. Dropping both lets the destination
// record the resource as its own, which is what kumactl does for the same two
// profiles and for the same reason.
//
// This is not a way to reapply an export to the control plane it came from.
// The origin label is immutable, so a resource that reached global by syncing
// up from a zone is refused on reapply with "cannot be changed from zone to
// global" whatever the stream says. `all` and `no-dataplanes` keep every
// label, matching kumactl, since they describe a control plane rather than
// seed one.
func removeFederationLabels(profile string, item map[string]any) {
	if profile != ProfileFederation && profile != ProfileFederationWithPolicies {
		return
	}

	labels, ok := item["labels"].(map[string]any)
	if !ok {
		return
	}

	for _, label := range federationExcludedLabels {
		delete(labels, label)
	}
	if len(labels) == 0 {
		// An empty map is written out as `labels: {}`, which is noise in a
		// stream meant to be read back.
		delete(item, "labels")
	}
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
			if descriptor.IsPolicy() {
				continue
			}
		case ProfileFederationWithPolicies:
			if !descriptor.IncludeInFederation {
				continue
			}
		case ProfileNoDataplanes:
			if descriptor.Name == dataplaneTypeName || descriptor.Name == dataplaneInsightTypeName {
				continue
			}
		}
		selected = append(selected, descriptor)
	}

	slices.SortStableFunc(selected, func(a, b ResourceDescriptor) int {
		return cmp.Compare(dumpPriority(a), dumpPriority(b))
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

// writeYAMLStream writes the resources as a multi document YAML stream, the
// form kumactl export produced and `apply mesh -f` reads back.
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
