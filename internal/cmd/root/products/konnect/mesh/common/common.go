package common

import (
	"fmt"
	"strings"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/pflag"
)

const (
	// CommandName is the name of the mesh container command.
	CommandName = "mesh"

	ControlPlaneIDFlagName   = "control-plane-id"
	ControlPlaneNameFlagName = "control-plane-name"
	ControlPlaneURLFlagName  = "control-plane-url"

	MeshFlagName      = "mesh"
	MeshFlagShorthand = "m"

	// AllMeshesFlagName lists a mesh scoped type across every mesh. Kuma
	// registers mesh scoped list endpoints at both /meshes/{mesh}/{path} and
	// /{path}, and the second lists across all meshes.
	AllMeshesFlagName = "all-meshes"

	// DefaultMesh matches the default kumactl applies to mesh scoped
	// resources, so that commands carrying no --mesh behave the same way.
	DefaultMesh = "default"
)

var (
	ControlPlaneIDConfigPath   = "konnect.mesh.control-plane.id"
	ControlPlaneNameConfigPath = "konnect.mesh.control-plane.name"
	ControlPlaneURLConfigPath  = "konnect.mesh.control-plane.url"
	MeshConfigPath             = "konnect.mesh.mesh"
	AllMeshesConfigPath        = "konnect.mesh.all-meshes"
)

// controlPlaneAPIPathFormat fronts a Konnect hosted Kong Mesh control plane's
// own API. The control plane identifier travels in the path, so callers do not
// send a separate tenant header.
//
// The leading segment selects the Kong Mesh API line, and one Konnect control
// plane serves more than one: /v1/mesh/control-planes/{id}/api reaches a 2.14
// control plane, while /v3/mesh/control-planes/{id} reaches a Kong Mesh 3 one.
// These are distinct control planes behind a single Konnect identifier, and the
// /api segment exists only on the v1 line. kongctl supports Kong Mesh 3 only,
// so it composes the v3 form.
const controlPlaneAPIPathFormat = "/v3/mesh/control-planes/%s"

// ControlPlaneAPIPath returns the Konnect path prefix for a hosted Kong Mesh
// control plane API.
func ControlPlaneAPIPath(controlPlaneID string) string {
	return fmt.Sprintf(controlPlaneAPIPathFormat, controlPlaneID)
}

// ResolveControlPlaneAPIURL returns the base URL of the Kong Mesh control plane
// API to send requests to.
//
// Two sources are supported so that Konnect hosted and self managed control
// planes are reached through the same commands:
//
//  1. An explicit control plane URL, used verbatim. This serves self managed
//     control planes, and overrides for hosted ones.
//  2. A Konnect control plane identifier, composed onto the Konnect base URL
//     already resolved for the active profile.
//
// An explicit URL wins when both are configured.
func ResolveControlPlaneAPIURL(cfg config.Hook) (string, error) {
	if explicit := strings.TrimSpace(cfg.GetString(ControlPlaneURLConfigPath)); explicit != "" {
		return strings.TrimRight(explicit, "/"), nil
	}

	controlPlaneID := strings.TrimSpace(cfg.GetString(ControlPlaneIDConfigPath))
	if controlPlaneID == "" {
		return "", missingControlPlaneError(cfg)
	}

	konnectBaseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(konnectBaseURL, "/") + ControlPlaneAPIPath(controlPlaneID), nil
}

// missingControlPlaneError explains which inputs identify a control plane,
// naming the control plane by name when one was given but not yet resolved.
func missingControlPlaneError(cfg config.Hook) error {
	if name := strings.TrimSpace(cfg.GetString(ControlPlaneNameConfigPath)); name != "" {
		return fmt.Errorf(
			"control plane %q has not been resolved to an identifier; provide --%s instead",
			name, ControlPlaneIDFlagName,
		)
	}
	return fmt.Errorf(
		"no Kong Mesh control plane selected; provide --%s for a Konnect hosted control plane, "+
			"or --%s for a self managed one",
		ControlPlaneIDFlagName,
		ControlPlaneURLFlagName,
	)
}

// ResolveMesh returns the mesh that mesh scoped requests apply to.
func ResolveMesh(cfg config.Hook) string {
	if mesh := strings.TrimSpace(cfg.GetString(MeshConfigPath)); mesh != "" {
		return mesh
	}
	return DefaultMesh
}

// AddControlPlaneFlags registers the flags that select a control plane and a
// mesh. Commands share these so that every mesh command accepts the same
// selection inputs.
func AddControlPlaneFlags(flags *pflag.FlagSet) {
	flags.String(ControlPlaneIDFlagName, "",
		fmt.Sprintf(`ID of the Konnect Kong Mesh control plane to use.
- Config path: [ %s ]`, ControlPlaneIDConfigPath))

	flags.String(ControlPlaneNameFlagName, "",
		fmt.Sprintf(`Name of the Konnect Kong Mesh control plane to use.
- Config path: [ %s ]`, ControlPlaneNameConfigPath))

	flags.String(ControlPlaneURLFlagName, "",
		fmt.Sprintf(`API URL of a self managed Kong Mesh control plane. Takes precedence over --%s.
- Config path: [ %s ]`, ControlPlaneIDFlagName, ControlPlaneURLConfigPath))

	flags.StringP(MeshFlagName, MeshFlagShorthand, DefaultMesh,
		fmt.Sprintf(`Mesh that mesh scoped resources belong to.
- Config path: [ %s ]`, MeshConfigPath))

	flags.Bool(AllMeshesFlagName, false,
		fmt.Sprintf(`List mesh scoped resources across every mesh instead of one. Ignored for global types.
- Config path: [ %s ]`, AllMeshesConfigPath))
}

// BindFlags associates the control plane selection flags with their
// configuration paths.
func BindFlags(cfg config.Hook, flags *pflag.FlagSet) error {
	if cfg == nil || flags == nil {
		return nil
	}

	bindings := []struct{ flag, config string }{
		{ControlPlaneIDFlagName, ControlPlaneIDConfigPath},
		{ControlPlaneNameFlagName, ControlPlaneNameConfigPath},
		{ControlPlaneURLFlagName, ControlPlaneURLConfigPath},
		{MeshFlagName, MeshConfigPath},
		{AllMeshesFlagName, AllMeshesConfigPath},
	}

	for _, b := range bindings {
		if f := flags.Lookup(b.flag); f != nil {
			if err := cfg.BindFlag(b.config, f); err != nil {
				return err
			}
		}
	}
	return nil
}
