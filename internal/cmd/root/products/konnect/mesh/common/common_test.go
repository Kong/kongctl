package common

import (
	"strings"
	"testing"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	configtest "github.com/kong/kongctl/test/config"
)

// stubConfig returns a config hook answering only the given paths, so a test
// states exactly the configuration it depends on.
func stubConfig(values map[string]string) *configtest.MockConfigHook {
	return &configtest.MockConfigHook{
		GetStringMock: func(key string) string { return values[key] },
	}
}

func TestResolveControlPlaneAPIURLFromControlPlaneID(t *testing.T) {
	cfg := stubConfig(map[string]string{
		ControlPlaneIDConfigPath:        "5bf706d9-1e96-4a3a-bee4-cbf806d1dc1a",
		konnectcommon.BaseURLConfigPath: "https://us.api.konghq.com",
	})

	got, err := ResolveControlPlaneAPIURL(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://us.api.konghq.com/v3/mesh/control-planes/5bf706d9-1e96-4a3a-bee4-cbf806d1dc1a"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveControlPlaneAPIURLTrimsTrailingSlashOnBaseURL(t *testing.T) {
	cfg := stubConfig(map[string]string{
		ControlPlaneIDConfigPath:        "cp-1",
		konnectcommon.BaseURLConfigPath: "https://eu.api.konghq.com/",
	})

	got, err := ResolveControlPlaneAPIURL(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "//v1") {
		t.Errorf("expected no doubled slash in %q", got)
	}
	if got != "https://eu.api.konghq.com/v3/mesh/control-planes/cp-1" {
		t.Errorf("unexpected URL: %s", got)
	}
}

// An explicit URL is what lets a self managed control plane be reached through
// the same commands as a hosted one, so it must win over the hosted inputs.
func TestResolveControlPlaneAPIURLExplicitURLWins(t *testing.T) {
	cfg := stubConfig(map[string]string{
		ControlPlaneURLConfigPath:       "https://mesh.internal.example.com:5681/",
		ControlPlaneIDConfigPath:        "cp-1",
		konnectcommon.BaseURLConfigPath: "https://us.api.konghq.com",
	})

	got, err := ResolveControlPlaneAPIURL(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://mesh.internal.example.com:5681" {
		t.Errorf("expected the explicit URL to be used verbatim, got %q", got)
	}
}

func TestResolveControlPlaneAPIURLWithoutSelection(t *testing.T) {
	cfg := stubConfig(map[string]string{})

	_, err := ResolveControlPlaneAPIURL(cfg)
	if err == nil {
		t.Fatal("expected an error when no control plane is selected")
	}
	// The message has to name the inputs an operator can supply.
	for _, want := range []string{ControlPlaneIDFlagName, ControlPlaneURLFlagName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error %q to mention --%s", err.Error(), want)
		}
	}
}

func TestResolveControlPlaneAPIURLWithUnresolvedName(t *testing.T) {
	cfg := stubConfig(map[string]string{
		ControlPlaneNameConfigPath: "my-mesh-cp",
	})

	_, err := ResolveControlPlaneAPIURL(cfg)
	if err == nil {
		t.Fatal("expected an error when only a name is configured")
	}
	if !strings.Contains(err.Error(), "my-mesh-cp") {
		t.Errorf("expected error %q to name the control plane", err.Error())
	}
}

func TestResolveMesh(t *testing.T) {
	if got := ResolveMesh(stubConfig(map[string]string{})); got != DefaultMesh {
		t.Errorf("expected the default mesh %q, got %q", DefaultMesh, got)
	}

	cfg := stubConfig(map[string]string{MeshConfigPath: " prod "})
	if got := ResolveMesh(cfg); got != "prod" {
		t.Errorf("expected surrounding whitespace to be trimmed, got %q", got)
	}
}

func TestControlPlaneAPIPath(t *testing.T) {
	if got := ControlPlaneAPIPath("cp-1"); got != "/v3/mesh/control-planes/cp-1" {
		t.Errorf("unexpected path: %s", got)
	}
}
