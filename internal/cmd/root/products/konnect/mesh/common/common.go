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

	// ExportProfileFlagName selects which types an export covers. It is not
	// called "profile": that is kongctl's global configuration profile, and a
	// local flag of the same name shadows it.
	ExportProfileFlagName = "export-profile"

	// TokenValidForFlagName sets how long an issued token remains valid, and
	// TokenScopeFlagName which scopes a zone token carries. Both express a
	// policy an operator applies to every token they issue, so both support a
	// persistent default.
	TokenValidForFlagName = "valid-for"
	TokenScopeFlagName    = "scope"

	// These configure how a self managed control plane is reached. They apply
	// only alongside ControlPlaneURLFlagName: a Konnect hosted control plane
	// is reached with Konnect credentials and Konnect's own certificates.
	ControlPlaneTokenFlagName = "control-plane-token"
	CACertFileFlagName        = "ca-cert-file"
	ClientCertFileFlagName    = "client-cert-file"
	ClientKeyFileFlagName     = "client-key-file"
	TLSSkipVerifyFlagName     = "tls-skip-verify"
)

var (
	ControlPlaneIDConfigPath   = "konnect.mesh.control-plane.id"
	ControlPlaneNameConfigPath = "konnect.mesh.control-plane.name"
	ControlPlaneURLConfigPath  = "konnect.mesh.control-plane.url"
	MeshConfigPath             = "konnect.mesh.mesh"
	AllMeshesConfigPath        = "konnect.mesh.all-meshes"
	ExportProfileConfigPath    = "konnect.mesh.export-profile"
	// These name where a token's lifetime and scope are configured. They hold
	// no credential themselves.
	TokenValidForConfigPath = "konnect.mesh.token.valid-for" // #nosec G101 -- configuration path, not a credential
	TokenScopeConfigPath    = "konnect.mesh.token.scope"     // #nosec G101 -- configuration path, not a credential

	// Self managed control plane connection settings.
	ControlPlaneTokenConfigPath = "konnect.mesh.control-plane.token" // #nosec G101 -- configuration path, not a credential
	CACertFileConfigPath        = "konnect.mesh.control-plane.ca-cert-file"
	ClientCertFileConfigPath    = "konnect.mesh.control-plane.client-cert-file"
	ClientKeyFileConfigPath     = "konnect.mesh.control-plane.client-key-file"
	TLSSkipVerifyConfigPath     = "konnect.mesh.control-plane.tls-skip-verify"
)

// ControlPlanesPath lists the Konnect hosted Kong Mesh control planes, and
// each control plane's own API hangs off its entry there. The control plane
// identifier travels in the path, so callers send no separate tenant header.
//
// The leading segment selects the Kong Mesh API line, and one Konnect control
// plane serves more than one: /v1/mesh/control-planes/{id}/api reaches a 2.14
// control plane, while /v3/mesh/control-planes/{id} reaches a Kong Mesh 3 one.
// These are distinct control planes behind a single Konnect identifier, and the
// /api segment exists only on the v1 line. kongctl supports Kong Mesh 3 only,
// so it composes the v3 form.
const ControlPlanesPath = "/v3/mesh/control-planes"

// ControlPlaneAPIPath returns the Konnect path prefix for a hosted Kong Mesh
// control plane API. A control plane's own API hangs directly off its entry in
// the control plane collection.
func ControlPlaneAPIPath(controlPlaneID string) string {
	return ControlPlanesPath + "/" + controlPlaneID
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
		return "", ErrNoControlPlaneSelected
	}

	return ControlPlaneAPIURLForID(cfg, controlPlaneID)
}

// ControlPlaneAPIURLForID builds the API URL of a Konnect hosted control plane
// from its ID.
//
// Callers that have established which selector the operator chose use this to
// address that control plane directly, without re-entering the precedence in
// ResolveControlPlaneAPIURL and picking up a different configured selector.
func ControlPlaneAPIURLForID(cfg config.Hook, controlPlaneID string) (string, error) {
	controlPlaneID = strings.TrimSpace(controlPlaneID)
	if controlPlaneID == "" {
		return "", ErrNoControlPlaneSelected
	}

	konnectBaseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(konnectBaseURL, "/") + ControlPlaneAPIPath(controlPlaneID), nil
}

// ErrNoControlPlaneSelected reports that nothing identified a control plane.
//
// A name is not resolvable from configuration alone, so callers that can reach
// Konnect check for this and try the name before surfacing it.
var ErrNoControlPlaneSelected = fmt.Errorf(
	"no Kong Mesh control plane selected; provide --%s or --%s for a Konnect hosted control plane, "+
		"or --%s for a self managed one",
	ControlPlaneIDFlagName,
	ControlPlaneNameFlagName,
	ControlPlaneURLFlagName,
)

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

	addSelfManagedFlags(flags)
}

// addSelfManagedFlags registers how a self managed control plane is reached.
//
// A self managed control plane authenticates its own callers, so none of these
// carry a Konnect credential. Kuma authenticates an API caller as admin over
// loopback, which is why a local control plane needs no token at all, and
// accepts a bearer token or a client certificate otherwise.
func addSelfManagedFlags(flags *pflag.FlagSet) {
	flags.String(ControlPlaneTokenFlagName, "",
		fmt.Sprintf(`Bearer token for a self managed control plane. Not used for a Konnect hosted one.
- Config path: [ %s ]`, ControlPlaneTokenConfigPath))

	flags.String(CACertFileFlagName, "",
		fmt.Sprintf(`Path to a CA certificate that verifies a self managed control plane.
- Config path: [ %s ]`, CACertFileConfigPath))

	flags.String(ClientCertFileFlagName, "",
		fmt.Sprintf(`Path to a client certificate presented to a self managed control plane.
- Config path: [ %s ]`, ClientCertFileConfigPath))

	flags.String(ClientKeyFileFlagName, "",
		fmt.Sprintf(`Path to the key for --%s.
- Config path: [ %s ]`, ClientCertFileFlagName, ClientKeyFileConfigPath))

	flags.Bool(TLSSkipVerifyFlagName, false,
		fmt.Sprintf(`Do not verify a self managed control plane's certificate. Prefer --%s.
- Config path: [ %s ]`, CACertFileFlagName, TLSSkipVerifyConfigPath))
}

// BindFlags associates the mesh flags with their configuration paths.
//
// Every option that supports a persistent default is listed here, and a flag
// absent from the command being run is skipped, so one call covers the shared
// selection flags and whichever command specific options are present.
//
// Options deliberately absent identify the subject of a single invocation --
// which dataplane, workload, zone or tags a token is for -- where a persistent
// default would silently issue a token for something other than what the
// operator named.
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
		{ExportProfileFlagName, ExportProfileConfigPath},
		{TokenValidForFlagName, TokenValidForConfigPath},
		{TokenScopeFlagName, TokenScopeConfigPath},
		{ControlPlaneTokenFlagName, ControlPlaneTokenConfigPath},
		{CACertFileFlagName, CACertFileConfigPath},
		{ClientCertFileFlagName, ClientCertFileConfigPath},
		{ClientKeyFileFlagName, ClientKeyFileConfigPath},
		{TLSSkipVerifyFlagName, TLSSkipVerifyConfigPath},
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
