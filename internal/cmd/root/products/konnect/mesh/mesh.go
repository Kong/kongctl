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

// FilenameFlagName names the -f flag that supplies resources to apply.
const FilenameFlagName = "filename"

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

// appliesResources reports whether a verb sends resource documents from -f.
//
// Only apply does. The control plane addresses a resource by type and name and
// a write creates or replaces it, so every resource write is an upsert; apply
// is both what that means and what kumactl calls it.
//
// `create mesh -f` reached the same write and so replaced an existing resource
// while reporting "updated", without the operator asking for a replacement.
// Kuma has no create-only write to implement a conflict check against, and
// detecting the conflict client-side would be a racy read-then-write, so the
// alias is gone rather than given surprising semantics. Token issuance stays
// under create, where a create is what it is.
func appliesResources(verb verbs.VerbValue) bool {
	return verb == verbs.Apply
}

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

	baseCmd.PreRunE = chainMeshPreRun(parentPreRun)
	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}
	meshcommon.AddControlPlaneFlags(baseCmd.PersistentFlags())
	if appliesResources(verb) {
		baseCmd.Flags().StringSliceP(FilenameFlagName, "f", nil,
			"Files, directories, URLs, or - for stdin, holding the mesh resources to apply. Repeatable.")
	}

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
		if appliesResources(verb) {
			// Resources come from -f, so a positional argument here is either
			// a mistyped subcommand or a misunderstanding of the command.
			if len(args) > 0 {
				return cmd.UnknownSubcommandError(cmdObj, args[0])
			}
			// Read the flag rather than binding a variable: one process can
			// hold a mesh command per verb, and a shared variable would leak
			// between them.
			filenames, err := cmdObj.Flags().GetStringSlice(FilenameFlagName)
			if err != nil {
				return err
			}
			return runApplyResources(helper, filenames)
		}
		if verb == verbs.Delete {
			if len(args) != 2 {
				return &cmd.ConfigurationError{
					Err: fmt.Errorf("expected a resource type and a name, for example 'delete mesh %s <name>'",
						"meshtrafficpermission"),
				}
			}
			return runDeleteResources(helper, args)
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
	if !appliesResources(verb) && verb != verbs.Delete {
		cmd.MarkRequiresSubcommand(baseCmd)
	}

	if verb == verbs.Get {
		baseCmd.AddCommand(newGetResourceTypesCmd(verb, addParentFlags, parentPreRun))
		baseCmd.AddCommand(newGetControlPlanesCmd(verb, addParentFlags, parentPreRun))
	}
	if verb == verbs.Create {
		baseCmd.AddCommand(newDataplaneTokenCmd(parentPreRun))
		baseCmd.AddCommand(newZoneTokenCmd(parentPreRun))
	}

	return baseCmd, nil
}

// BindFlags binds the mesh command's own flags to configuration, in the
// signature the other product binders use so it can sit in a pre-run chain.
func BindFlags(c *cobra.Command, args []string) error {
	if c == nil {
		return nil
	}

	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	return meshcommon.BindFlags(cfg, c.Flags())
}

// chainMeshPreRun runs the caller's pre-run, then binds the mesh command's own
// flags.
//
// The mesh command used to assign the caller's pre-run outright and bind
// nothing itself, which left binding to whoever registered it. The four direct
// verb paths each called meshcommon.BindFlags in their own closure; the
// explicit konnect path passed the general konnect pre-run, which knows
// nothing about mesh, so every mesh flag there was registered but unbound and
// `get konnect mesh --control-plane-url ...` reported no control plane
// selected. Binding here means the two trees cannot diverge again, and
// matches how every other product owns both halves.
func chainMeshPreRun(
	parentPreRun func(*cobra.Command, []string) error,
) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		if parentPreRun != nil {
			if err := parentPreRun(c, args); err != nil {
				return err
			}
		}
		return BindFlags(c, args)
	}
}
