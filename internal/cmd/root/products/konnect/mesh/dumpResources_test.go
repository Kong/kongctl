package mesh

import (
	"slices"
	"testing"
)

func dumpDescriptors() []ResourceDescriptor {
	return []ResourceDescriptor{
		{Name: "Mesh", Path: "meshes", Scope: ScopeGlobal, IncludeInFederation: true},
		{Name: "Secret", Path: "secrets", Scope: ScopeMesh, IncludeInFederation: true},
		{Name: "GlobalSecret", Path: "globalsecrets", Scope: ScopeGlobal, IncludeInFederation: true},
		{Name: "HostnameGenerator", Path: "hostnamegenerators", Scope: ScopeGlobal, IncludeInFederation: true},
		// A targetRef policy: federated zones keep these, so the plain
		// federation profile leaves them behind.
		{
			Name: "MeshTimeout", Path: "meshtimeouts", Scope: ScopeMesh, IncludeInFederation: true,
			Policy: &PolicyDescriptor{IsTargetRef: true},
		},
		// Insights are computed by the target control plane, so they are never
		// part of a federation export.
		{Name: "MeshInsight", Path: "mesh-insights", Scope: ScopeGlobal, ReadOnly: true},
		{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh},
		{Name: "DataplaneInsight", Path: "dataplane-insights", Scope: ScopeMesh, ReadOnly: true},
	}
}

func selectedPaths(profile string) []string {
	var paths []string
	for _, d := range selectTypesToDump(dumpDescriptors(), profile) {
		paths = append(paths, d.Path)
	}
	return paths
}

func TestSelectTypesToDumpByProfile(t *testing.T) {
	tests := []struct {
		profile     string
		wantPresent []string
		wantAbsent  []string
	}{
		{
			profile:     ProfileFederation,
			wantPresent: []string{"meshes", "secrets", "globalsecrets", "hostnamegenerators"},
			// Excluded for two different reasons: a targetRef policy, and
			// types that are not in federation at all.
			wantAbsent: []string{"meshtimeouts", "dataplanes", "mesh-insights"},
		},
		{
			profile:     ProfileFederationWithPolicies,
			wantPresent: []string{"meshes", "meshtimeouts"},
			wantAbsent:  []string{"dataplanes", "mesh-insights"},
		},
		{
			profile:     ProfileAll,
			wantPresent: []string{"meshes", "meshtimeouts", "dataplanes", "dataplane-insights", "mesh-insights"},
		},
		{
			profile:     ProfileNoDataplanes,
			wantPresent: []string{"meshes", "meshtimeouts", "mesh-insights"},
			wantAbsent:  []string{"dataplanes", "dataplane-insights"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.profile, func(t *testing.T) {
			got := selectedPaths(tc.profile)
			for _, want := range tc.wantPresent {
				if !slices.Contains(got, want) {
					t.Errorf("%s should include %s; got %v", tc.profile, want, got)
				}
			}
			for _, absent := range tc.wantAbsent {
				if slices.Contains(got, absent) {
					t.Errorf("%s should not include %s; got %v", tc.profile, absent, got)
				}
			}
		})
	}
}

func TestFederationExcludesPoliciesWithoutTargetRef(t *testing.T) {
	descriptors := []ResourceDescriptor{{
		Name: "Policy", Path: "policies", IncludeInFederation: true, Policy: &PolicyDescriptor{},
	}}
	if got := selectTypesToDump(descriptors, ProfileFederation); len(got) != 0 {
		t.Fatalf("plain federation included a policy: %v", got)
	}
	if got := selectTypesToDump(descriptors, ProfileFederationWithPolicies); len(got) != 1 {
		t.Fatalf("federation-with-policies excluded a policy: %v", got)
	}
}

func TestFederationRemovesControlPlaneOwnedLabels(t *testing.T) {
	labels := map[string]any{
		"kuma.io/mesh": "default", "kuma.io/workload": "backend", "team": "platform",
	}
	for _, key := range federationExcludedLabels {
		labels[key] = "source-control-plane"
	}
	item := map[string]any{"labels": labels}
	removeFederationLabels(ProfileFederationWithPolicies, item)
	if len(labels) != 3 || labels["kuma.io/mesh"] != "default" || labels["kuma.io/workload"] != "backend" {
		t.Fatalf("migration retained computed labels or lost user labels: %v", labels)
	}
}

// The export has to be applicable in order: a mesh before anything inside it,
// and the global secrets after everything that a changed signing key would
// otherwise lock out.
func TestSelectTypesToDumpOrdering(t *testing.T) {
	got := selectedPaths(ProfileAll)

	if got[0] != meshesPath {
		t.Errorf("meshes must be exported first, got %v", got)
	}
	if slices.Index(got, secretsPath) > slices.Index(got, globalSecretPath) {
		t.Errorf("secrets must precede global secrets, got %v", got)
	}
	if slices.Index(got, globalSecretPath) != len(got)-1 {
		t.Errorf("global secrets must be exported last, got %v", got)
	}
}

func TestClassifyForDump(t *testing.T) {
	meshes := ResourceDescriptor{Name: "Mesh", Path: meshesPath, Scope: ScopeGlobal}
	globalSecrets := ResourceDescriptor{Name: "GlobalSecret", Path: globalSecretPath, Scope: ScopeGlobal}
	policies := ResourceDescriptor{Name: "MeshTimeout", Path: "meshtimeouts", Scope: ScopeMesh}

	tests := []struct {
		name       string
		descriptor ResourceDescriptor
		item       map[string]any
		want       dumpBucket
	}{
		{"a mesh is applied first", meshes, map[string]any{"name": "default"}, bucketFirst},
		{"ordinary resources sit in the middle", policies, map[string]any{"name": "slow"}, bucketMiddle},
		{
			"an ordinary global secret sits in the middle",
			globalSecrets,
			map[string]any{"name": "zone-token-signing-key-1"},
			bucketMiddle,
		},
		// Carrying these to another control plane breaks its TLS handshakes.
		{"envoy-admin-ca is skipped", globalSecrets, map[string]any{"name": "envoy-admin-ca"}, bucketSkip},
		{"inter-cp-ca is skipped", globalSecrets, map[string]any{"name": "inter-cp-ca"}, bucketSkip},
		// Applying this invalidates the credential in use.
		{
			"a user token signing key goes last",
			globalSecrets,
			map[string]any{"name": "user-token-signing-key-1"},
			bucketLast,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyForDump(tc.descriptor, tc.item); got != tc.want {
				t.Errorf("bucket = %d, want %d", got, tc.want)
			}
		})
	}
}

// An exported mesh must not recreate its default policies on apply, or it
// would conflict with the policies exported alongside it.
func TestClassifyForDumpSuppressesInitialPolicies(t *testing.T) {
	item := map[string]any{"name": "default"}
	classifyForDump(ResourceDescriptor{Name: "Mesh", Path: meshesPath, Scope: ScopeGlobal}, item)

	skip, ok := item["skipCreatingInitialPolicies"].([]string)
	if !ok || len(skip) != 1 || skip[0] != "*" {
		t.Errorf("expected skipCreatingInitialPolicies to be set to [*], got %v", item["skipCreatingInitialPolicies"])
	}
}

func TestRemoveFederationLabels(t *testing.T) {
	// A global control plane rejects kuma.io/zone outright and treats
	// kuma.io/origin as immutable, so a federation export must carry neither.
	tests := []struct {
		name      string
		profile   string
		labels    map[string]any
		wantGone  []string
		wantKept  []string
		wantNoKey bool
	}{
		{
			name:    "federation drops origin and zone",
			profile: ProfileFederation,
			labels: map[string]any{
				"kuma.io/origin":       "zone",
				"kuma.io/zone":         "zone1",
				"kuma.io/display-name": "backend",
			},
			wantGone:  []string{"kuma.io/origin", "kuma.io/zone", "kuma.io/display-name"},
			wantNoKey: true,
		},
		{
			name:    "federation-with-policies drops them too",
			profile: ProfileFederationWithPolicies,
			labels: map[string]any{
				"kuma.io/origin": "zone",
				"kuma.io/zone":   "zone2",
				"kuma.io/mesh":   "default",
			},
			wantGone: []string{"kuma.io/origin", "kuma.io/zone"},
			wantKept: []string{"kuma.io/mesh"},
		},
		{
			name:    "all keeps every label",
			profile: ProfileAll,
			labels: map[string]any{
				"kuma.io/origin": "zone",
				"kuma.io/zone":   "zone1",
			},
			wantKept: []string{"kuma.io/origin", "kuma.io/zone"},
		},
		{
			name:      "labels map is dropped when it empties out",
			profile:   ProfileFederation,
			labels:    map[string]any{"kuma.io/origin": "zone", "kuma.io/zone": "zone1"},
			wantNoKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := map[string]any{"name": "res", "labels": tt.labels}
			removeFederationLabels(tt.profile, item)

			if tt.wantNoKey {
				if _, ok := item["labels"]; ok {
					t.Errorf("labels key should be removed once empty, got %v", item["labels"])
				}
				return
			}

			labels, ok := item["labels"].(map[string]any)
			if !ok {
				t.Fatalf("labels missing or wrong type: %v", item["labels"])
			}
			for _, k := range tt.wantGone {
				if _, present := labels[k]; present {
					t.Errorf("%s should have been stripped for profile %q", k, tt.profile)
				}
			}
			for _, k := range tt.wantKept {
				if _, present := labels[k]; !present {
					t.Errorf("%s should have been kept for profile %q", k, tt.profile)
				}
			}
		})
	}
}
