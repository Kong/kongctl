package portal

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	deletePortalShort = i18n.T("root.products.konnect.portal.deletePortalShort",
		"Delete a Konnect portal")
	deletePortalLong = i18n.T("root.products.konnect.portal.deletePortalLong",
		`Delete a portal by ID or name.

If the portal has child resources (pages, snippets, custom domains), the deletion will fail
unless the --force flag is used. Using --force will cascade delete the portal and all its
child resources.

A confirmation prompt will be shown before deletion unless --auto-approve is used.`)
	deletePortalExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.deletePortalExamples",
			fmt.Sprintf(`
	# Delete a portal by ID
	%[1]s delete portal 12345678-1234-1234-1234-123456789012

	# Delete a portal by name
	%[1]s delete portal my-portal

	# Force delete a portal with child resources
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
	var portal *kkComps.Portal

	// Check if argument is UUID
	uuidRegex := regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	isUUID := uuidRegex.MatchString(portalID)

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
			return fmt.Errorf("portal not found: %s", portalID)
		}
		// Convert PortalResponse to Portal for consistency
		pr := portalResponse.GetPortalResponse()
		portal = &kkComps.Portal{
			ID:          pr.ID,
			Name:        pr.Name,
			Description: pr.Description,
		}
	}

	// Show confirmation prompt unless --auto-approve
	if !c.autoApprove {
		if !c.confirmDeletion(portal, helper) {
			return fmt.Errorf("delete cancelled")
		}
	}

	// Delete the portal
	logger.Info(fmt.Sprintf("Deleting portal '%s' (ID: %s)", portal.Name, portalID))

	_, err := sdk.GetPortalAPI().DeletePortal(helper.GetContext(), portalID, c.force)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		// Check if error is due to child resources
		if !c.force && strings.Contains(err.Error(), "child") {
			attrs = append(attrs, "suggestion", "Use --force to cascade delete portal with child resources")
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
	response := map[string]interface{}{
		"id":      portalID,
		"name":    portal.Name,
		"status":  "deleted",
		"message": fmt.Sprintf("Portal '%s' deleted successfully", portal.Name),
	}

	if outType == cmdCommon.TEXT {
		// For text output, just print the success message
		fmt.Fprintf(helper.GetStreams().Out, "Portal '%s' deleted successfully\n", portal.Name)
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
) (*kkComps.Portal, error) {
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
		var matches []*kkComps.Portal
		for _, portal := range listResponse.Data {
			if portal.Name == name {
				// Create a copy to avoid pointer issues
				p := portal
				matches = append(matches, &p)
			}
		}

		if len(matches) > 1 {
			return nil, fmt.Errorf("multiple portals found with name '%s'. Please use ID instead", name)
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

	return nil, fmt.Errorf("portal not found: %s", name)
}

func (c *deletePortalCmd) confirmDeletion(portal *kkComps.Portal, helper cmd.Helper) bool {
	streams := helper.GetStreams()

	// Print warning
	fmt.Fprintln(streams.Out, "\nWARNING: This will permanently delete the following portal:")
	fmt.Fprintf(streams.Out, "\n  Name: %s\n", portal.Name)
	fmt.Fprintf(streams.Out, "  ID:   %s\n", portal.ID)

	// Add warning about child resources if not using force
	if !c.force {
		fmt.Fprintln(streams.Out, "\nNote: If this portal has child resources (pages, snippets, custom domains),")
		fmt.Fprintln(streams.Out, "      the deletion will fail.")
		fmt.Fprintln(streams.Out, "      Use --force to cascade delete the portal and all its child resources.")
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
		"Force deletion even if the portal has child resources (cascades delete)")
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