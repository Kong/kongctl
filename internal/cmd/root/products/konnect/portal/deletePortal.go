package portal

import (
	"fmt"
	"os"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type portalSummary struct {
	ID          string
	Name        *string
	Description *string
}

var (
	deletePortalShort = i18n.T("root.products.konnect.portal.deletePortalShort",
		"Delete a Konnect portal")
	deletePortalLong = i18n.T("root.products.konnect.portal.deletePortalLong",
		`Delete a portal by ID or name.

If the portal has published APIs, the deletion will fail unless the --force flag is used.
Using --force will delete the portal along with all API publications.

A confirmation prompt will be shown before deletion unless --auto-approve is used.`)
	deletePortalExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.deletePortalExamples",
			fmt.Sprintf(`
	# Delete a portal by ID
	%[1]s delete portal 12345678-1234-1234-1234-123456789012

	# Delete a portal by name
	%[1]s delete portal my-portal

	# Force delete a portal with published APIs
	%[1]s delete portal my-portal --force

	# Delete without confirmation prompt
	%[1]s delete portal my-portal --auto-approve
	`, meta.CLIName)))
)

type deletePortalCmd struct {
	*cobra.Command
	force       bool
	autoApprove bool
}

func (c *deletePortalCmd) validate(helper cmd.Helper) error {
	args := helper.GetArgs()
	if len(args) == 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("portal ID or name is required"),
		}
	}
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Deleting a portal requires exactly 1 argument (name or ID)"),
		}
	}
	return nil
}

func (c *deletePortalCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	// Get portal ID (resolve name if necessary)
	portalID := strings.TrimSpace(args[0])
	var portal *portalSummary

	// Check if argument is UUID
	isUUID := util.IsValidUUID(portalID)

	if !isUUID {
		// Resolve name to ID
		logger.Debug(fmt.Sprintf("Resolving portal name '%s' to ID", portalID))
		resolvedPortal, err := c.resolvePortalByName(portalID, sdk.GetPortalAPI(), helper, nil)
		if err != nil {
			return err
		}
		portal = resolvedPortal
		portalID = portal.ID
	} else {
		// Get portal details for confirmation
		portalResponse, err := sdk.GetPortalAPI().GetPortal(helper.GetContext(), portalID)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get portal details", err, helper.GetCmd(), attrs...)
		}
		if portalResponse.GetPortalResponse() == nil {
			return cmd.PrepareExecutionErrorMsg(helper, fmt.Sprintf("portal not found: %s", portalID))
		}
		// Convert PortalResponse to Portal for consistency
		pr := portalResponse.GetPortalResponse()
		portal = &portalSummary{
			ID:          pr.ID,
			Name:        pr.Name,
			Description: pr.Description,
		}
	}

	// Show confirmation prompt unless --auto-approve
	if !c.autoApprove {
		if !c.confirmDeletion(portal, helper) {
			return cmd.PrepareExecutionErrorMsg(helper, "delete cancelled")
		}
	}

	// Delete the portal
	portalName := util.StringValue(portal.Name)
	logger.Info(fmt.Sprintf("Deleting portal '%s' (ID: %s)", portalName, portalID))

	_, err := sdk.GetPortalAPI().DeletePortal(helper.GetContext(), portalID, c.force)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		// Check if error is due to published APIs
		if !c.force && (strings.Contains(err.Error(), "published") || strings.Contains(err.Error(), "API")) {
			attrs = append(attrs, "suggestion", "Use --force to delete portal with published APIs")
		}
		return cmd.PrepareExecutionError("Failed to delete portal", err, helper.GetCmd(), attrs...)
	}

	// Format and output response
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	// Create success response
	response := map[string]any{
		"id":      portalID,
		"name":    portalName,
		"status":  "deleted",
		"message": fmt.Sprintf("Portal '%s' deleted successfully", portalName),
	}

	if outType == cmdCommon.TEXT {
		// For text output, just print the success message
		fmt.Fprintf(helper.GetStreams().Out, "Portal '%s' deleted successfully\n", portalName)
	} else {
		// For JSON/YAML output, print the structured response
		printer.Print(response)
	}

	return nil
}

func (c *deletePortalCmd) resolvePortalByName(
	name string,
	api helpers.PortalAPI,
	helper cmd.Helper,
	_ config.Hook,
) (*portalSummary, error) {
	pageSize := int64(common.DefaultRequestPageSize)
	pageNumber := int64(1)

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := api.ListPortals(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portals", err, c.Command, attrs...)
		}

		listResponse := res.GetListPortalsResponse()
		if listResponse == nil || listResponse.Data == nil {
			break
		}

		// Look for exact name match
		var matches []*portalSummary
		for _, portal := range listResponse.Data {
			if util.StringValue(portal.Name) == name {
				// Create a copy to avoid pointer issues
				matches = append(matches, &portalSummary{
					ID:          portal.ID,
					Name:        portal.Name,
					Description: portal.Description,
				})
			}
		}

		if len(matches) > 1 {
			return nil, cmd.PrepareExecutionErrorMsg(helper,
				fmt.Sprintf("multiple portals found with name '%s'. Please use ID instead", name))
		}

		if len(matches) == 1 {
			return matches[0], nil
		}

		// Check if there are more pages
		totalItems := listResponse.Meta.Page.Total
		if len(listResponse.Data) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, cmd.PrepareExecutionErrorMsg(helper, fmt.Sprintf("portal not found: %s", name))
}

func (c *deletePortalCmd) confirmDeletion(portal *portalSummary, helper cmd.Helper) bool {
	streams := helper.GetStreams()

	// Print warning
	fmt.Fprintln(streams.Out, "\nWARNING: This will permanently delete the following portal:")
	fmt.Fprintf(streams.Out, "\n  Name: %s\n", util.StringValue(portal.Name))
	fmt.Fprintf(streams.Out, "  ID:   %s\n", portal.ID)

	// Add warning about published APIs if not using force
	if !c.force {
		fmt.Fprintln(streams.Out, "\nNote: If this portal has published APIs, the deletion will fail.")
		fmt.Fprintln(streams.Out, "      Use --force to delete the portal along with all API publications.")
	}

	fmt.Fprint(streams.Out, "\nDo you want to continue? Type 'yes' to confirm: ")

	// Handle input (check if stdin is piped)
	input := streams.In
	if f, ok := input.(*os.File); ok && f.Fd() == 0 {
		// stdin is piped, try to use /dev/tty
		tty, err := os.Open("/dev/tty")
		if err == nil {
			defer tty.Close()
			input = tty
		}
	}

	// Read user input
	var response string
	_, err := fmt.Fscanln(input, &response)
	if err != nil {
		// If there's an error reading input, treat as non-confirmation
		return false
	}

	return strings.ToLower(strings.TrimSpace(response)) == "yes"
}

func newDeletePortalCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *deletePortalCmd {
	rv := deletePortalCmd{
		Command: baseCmd,
	}

	rv.Short = deletePortalShort
	rv.Long = deletePortalLong
	rv.Example = deletePortalExample
	rv.Args = cobra.ExactArgs(1)

	// Add flags
	rv.Flags().BoolVar(&rv.force, "force", false,
		"Force deletion even if the portal has published APIs")
	rv.Flags().BoolVar(&rv.autoApprove, "auto-approve", false,
		"Skip confirmation prompt")

	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
