package identity

import (
	"slices"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
)

func TestNewIdentityCmdDirectoryAliases(t *testing.T) {
	cmd, err := NewIdentityCmd(verbs.Get, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	directoryCmd, _, err := cmd.Find([]string{"directories"})
	if err != nil {
		t.Fatalf("expected directory command lookup to succeed: %v", err)
	}
	if directoryCmd == nil {
		t.Fatal("expected directory command")
	}

	for _, alias := range []string{"directories", "dir", "dirs"} {
		if !slices.Contains(directoryCmd.Aliases, alias) {
			t.Fatalf("expected alias %q in %v", alias, directoryCmd.Aliases)
		}
	}
}

func TestDirectoryDetailViewIncludesRealmConfig(t *testing.T) {
	allowAll := true
	ttl := int64(300)
	realmTTL := int64(10)
	directory := directoryResource{
		ID:                    "d67a4203-b1e8-4631-a626-5fe7c55efe88",
		Name:                  "workforce",
		Description:           "Workforce identities",
		AllowedControlPlanes:  []string{"cp-1", "cp-2"},
		AllowAllControlPlanes: &allowAll,
		TTLSecs:               &ttl,
		Labels:                map[string]string{"env": "test"},
		RealmConfig: &directoryRealmConfig{
			TTL:            &realmTTL,
			ConsumerGroups: []string{"employees"},
		},
	}

	detail := directoryDetailView(directory)
	for _, expected := range []string{
		"name: workforce",
		"allowed_control_planes: cp-1, cp-2",
		"allow_all_control_planes: true",
		"ttl_secs: 300",
		"labels: env=test",
		"realm_config.ttl: 10",
		"realm_config.consumer_groups: employees",
	} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("expected detail to contain %q, got:\n%s", expected, detail)
		}
	}
}

func TestDirectoryAdoptLabelsPreserveExistingLabels(t *testing.T) {
	result := directoryAdoptLabels(map[string]string{
		"team":              "platform",
		labels.ProtectedKey: labels.TrueValue,
	}, "identity")

	if result["team"] != "platform" {
		t.Fatalf("expected user label to be preserved, got %v", result)
	}
	if result[labels.ProtectedKey] != labels.TrueValue {
		t.Fatalf("expected protected label to be preserved, got %v", result)
	}
	if result[labels.NamespaceKey] != "identity" {
		t.Fatalf("expected namespace label to be set, got %v", result)
	}
}
