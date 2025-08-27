package portal

import (
	"fmt"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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

var (
	getPortalsShort = i18n.T("root.products.konnect.portal.getPortalsShort",
		"List or get Konnect portals")
	getPortalsLong = i18n.T("root.products.konnect.portal.getPortalsLong",
		`Use the get verb with the portal command to query Konnect portals.`)
	getPortalsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.getPortalExamples",
			fmt.Sprintf(`
	# List all the portals for the organization
	%[1]s get portals
	# Get details for a portal with a specific ID 
	%[1]s get portal 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for a portal with a specific name
	%[1]s get portal my-portal 
	# Get all the portals using command aliases
	%[1]s get ps
	`, meta.CLIName)))
)

// Represents a text display record for a Portal
type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	CustomDomain     string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func portalToDisplayRecord(p *kkComps.Portal) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if p.ID != "" {
		id = p.ID
	} else {
		id = missing
	}

	if p.Name != "" {
		name = p.Name
	} else {
		name = missing
	}

	description := missing
	if p.Description != nil && *p.Description != "" {
		description = *p.Description
	}

	// CustomDomain field doesn't exist in current SDK
	customDomain := missing

	createdAt := p.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := p.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		CustomDomain:     customDomain,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func portalResponseToDisplayRecord(p *kkComps.PortalResponse) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if p.ID != "" {
		id = p.ID
	} else {
		id = missing
	}

	if p.Name != "" {
		name = p.Name
	} else {
		name = missing
	}

	description := missing
	if p.Description != nil && *p.Description != "" {
		description = *p.Description
	}

	// Use CanonicalDomain from PortalResponse
	customDomain := missing
	if p.CanonicalDomain != "" {
		customDomain = p.CanonicalDomain
	}

	createdAt := p.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := p.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		CustomDomain:     customDomain,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

type getPortalCmd struct {
	*cobra.Command
}

func runListByName(name string, kkClient helpers.PortalAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.Portal, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.Portal

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListPortals(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Portals", err, helper.GetCmd(), attrs...)
		}

		// Filter by name since SDK doesn't support name filtering for portals
		for _, portal := range res.GetListPortalsResponse().Data {
			if portal.Name == name {
				return &portal, nil
			}
		}

		allData = append(allData, res.GetListPortalsResponse().Data...)
		totalItems := res.GetListPortalsResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, fmt.Errorf("portal with name %s not found", name)
}

func runList(kkClient helpers.PortalAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.Portal, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.Portal

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListPortals(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Portals", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetListPortalsResponse().Data...)
		totalItems := res.GetListPortalsResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runGet(id string, kkClient helpers.PortalAPI, helper cmd.Helper,
) (*kkComps.PortalResponse, error) {
	res, err := kkClient.GetPortal(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get Portal", err, helper.GetCmd(), attrs...)
	}

	return res.GetPortalResponse(), nil
}

func (c *getPortalCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portals requires 0 or 1 arguments (name or ID)"),
		}
	}

	config, err := helper.GetConfig()
	if err != nil {
		return err
	}

	pageSize := config.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", common.RequestPageSizeFlagName),
		}
	}
	return nil
}

func (c *getPortalCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}

	defer printer.Flush()

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	// 'get portals' can be run in various ways:
	//	> get portals <id>    # Get by UUID
	//  > get portals <name>	# Get by name
	//  > get portals         # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := strings.TrimSpace(helper.GetArgs()[0])

		isUUID := util.IsValidUUID(id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the portal by name
			portal, e := runListByName(id, sdk.GetPortalAPI(), helper, cfg)
			if e == nil {
				if outType == cmdCommon.TEXT {
					printer.Print(portalToDisplayRecord(portal))
				} else {
					printer.Print(portal)
				}
			} else {
				return e
			}
		} else {
			portalResponse, e := runGet(id, sdk.GetPortalAPI(), helper)
			if e == nil {
				if outType == cmdCommon.TEXT {
					printer.Print(portalResponseToDisplayRecord(portalResponse))
				} else {
					printer.Print(portalResponse)
				}
			} else {
				return e
			}
		}
	} else { // list all portals
		var portals []kkComps.Portal
		portals, e = runList(sdk.GetPortalAPI(), helper, cfg)
		if e == nil {
			if outType == cmdCommon.TEXT {
				var displayRecords []textDisplayRecord
				for _, portal := range portals {
					displayRecords = append(displayRecords, portalToDisplayRecord(&portal))
				}
				printer.Print(displayRecords)
			} else {
				printer.Print(portals)
			}
		} else {
			return e
		}
	}

	return nil
}

func newGetPortalCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getPortalCmd {
	rv := getPortalCmd{
		Command: baseCmd,
	}

	rv.Short = getPortalsShort
	rv.Long = getPortalsLong
	rv.Example = getPortalsExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
