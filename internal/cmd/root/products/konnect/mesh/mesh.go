package mesh

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const CommandName = meshcommon.CommandName

var (
	meshUse = CommandName

	meshShort = i18n.T("root.products.konnect.mesh.meshShort",
		"Manage Kong Mesh control plane resources")

	meshLong = normalizers.LongDesc(i18n.T("root.products.konnect.mesh.meshLong",
		`The mesh command works with resources on a Kong Mesh control plane.

The resource types available are reported by the control plane itself, so
policies and resource types added by newer Kong Mesh releases are usable
without upgrading kongctl.

Kong Mesh 3.0 or later is required.`))

	meshExample = normalizers.Examples(i18n.T("root.products.konnect.mesh.meshExample",
		fmt.Sprintf(`
	# List the resource types a control plane serves
	%[1]s get mesh resource-types --control-plane-id <id>

	# List dataplanes in the default mesh
	%[1]s get mesh dataplanes --control-plane-id <id>

	# Read one resource, by type and name
	%[1]s get mesh meshes default --control-plane-id <id>

	# Address a type by its short name, in a named mesh
	%[1]s get mesh dp -m prod --control-plane-id <id>

	# List a mesh scoped type across every mesh
	%[1]s get mesh meshtrafficpermissions --all-meshes --control-plane-id <id>
	`, meta.CLIName)))
)

// NewMeshCmd builds the mesh container command for a verb.
//
// It follows the same constructor shape as the other product containers so
// that the verb packages can register it both directly, giving
// "kongctl get mesh ...", and under the konnect subtree.
func NewMeshCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := &cobra.Command{
		Use:     meshUse,
		Short:   meshShort,
		Long:    meshLong,
		Example: meshExample,
	}

	if parentPreRun != nil {
		baseCmd.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}
	meshcommon.AddControlPlaneFlags(baseCmd.PersistentFlags())

	// Resource types come from the control plane at runtime, so they cannot be
	// registered as subcommands without a network call at startup. Arbitrary
	// args are accepted instead and dispatched to the generic read, which
	// resolves the type against /_resources.
	baseCmd.Args = cobra.ArbitraryArgs
	// Cobra applies this default only on its own suggestion path, and the
	// dispatch below calls SuggestionsFor directly to tell a mistyped
	// subcommand apart from a resource type.
	baseCmd.SuggestionsMinimumDistance = 2
	baseCmd.RunE = func(cmdObj *cobra.Command, args []string) error {
		helper := cmd.BuildHelper(cmdObj, args)
		if _, err := helper.GetOutputFormat(); err != nil {
			return err
		}
		if verb == verbs.Get && len(args) > 0 {
			// A near miss of a real subcommand is a mistyped subcommand, not a
			// resource type. Saying so here keeps that error immediate, rather
			// than sending a doomed request to the control plane first.
			if len(cmdObj.SuggestionsFor(args[0])) > 0 {
				return cmd.UnknownSubcommandError(cmdObj, args[0])
			}
			if len(args) > 2 {
				return &cmd.ConfigurationError{
					Err: fmt.Errorf(
						"expected a resource type and an optional name, got %d arguments", len(args)),
				}
			}
			return runGetResources(helper, args)
		}
		return cmd.RequireSubcommand(cmdObj, args)
	}
	cmd.MarkRequiresSubcommand(baseCmd)

	if verb == verbs.Get {
		baseCmd.AddCommand(newGetResourceTypesCmd(verb, addParentFlags, parentPreRun))
	}

	return baseCmd, nil
}
