package api

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
	getAPIsShort = i18n.T("root.products.konnect.api.getAPIsShort",
		"List or get Konnect APIs")
	getAPIsLong = i18n.T("root.products.konnect.api.getAPIsLong",
		`Use the get verb with the api command to query Konnect APIs.`)
	getAPIsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.getAPIExamples",
			fmt.Sprintf(`
	# List all the APIs for the organization
	%[1]s get apis
	# Get details for an API with a specific ID 
	%[1]s get api 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for an API with a specific name
	%[1]s get api my-api 
	# List all APIs with version information
	%[1]s get apis --include-versions
	# Get an API with publication information
	%[1]s get api my-api --include-publications
	# Get all the APIs using command aliases
	%[1]s get as
	`, meta.CLIName)))
)

const (
	includeVersionsFlagName     = "include-versions"
	includePublicationsFlagName = "include-publications"
)

// Represents a text display record for an API
type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	VersionCount     string
	PublicationCount string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func apiToDisplayRecord(a *kkComps.APIResponseSchema, _, includePublications bool) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if a.ID != "" {
		id = a.ID
	} else {
		id = missing
	}

	if a.Name != "" {
		name = a.Name
	} else {
		name = missing
	}

	description := missing
	if a.Description != nil && *a.Description != "" {
		description = *a.Description
	}

	versionCount := missing
	// Version count is not directly available in APIResponseSchema
	// It would require a separate API call to list versions

	publicationCount := missing
	if includePublications && a.Portals != nil {
		publicationCount = fmt.Sprintf("%d", len(a.Portals))
	}

	createdAt := a.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := a.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		VersionCount:     versionCount,
		PublicationCount: publicationCount,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

type getAPICmd struct {
	*cobra.Command
	includeVersions     bool
	includePublications bool
}

func runListByName(name string, _, _ bool, kkClient helpers.APIAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.APIResponseSchema, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.APIResponseSchema

	for {
		req := kkOps.ListApisRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		// Note: The SDK's ListApisRequest doesn't support include parameter
		// Version and publication information would require separate API calls

		res, err := kkClient.ListApis(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list APIs", err, helper.GetCmd(), attrs...)
		}

		// Filter by name since SDK doesn't support name filtering for APIs
		for _, api := range res.ListAPIResponse.Data {
			if api.Name == name {
				return &api, nil
			}
		}

		allData = append(allData, res.ListAPIResponse.Data...)
		totalItems := res.ListAPIResponse.Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, fmt.Errorf("API with name %s not found", name)
}

func runList(_, _ bool, kkClient helpers.APIAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.APIResponseSchema, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.APIResponseSchema

	for {
		req := kkOps.ListApisRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		// Note: The SDK's ListApisRequest doesn't support include parameter
		// Version and publication information would require separate API calls

		res, err := kkClient.ListApis(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list APIs", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.ListAPIResponse.Data...)
		totalItems := res.ListAPIResponse.Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runGet(id string, _, _ bool, kkClient helpers.APIAPI, helper cmd.Helper,
) (*kkComps.APIResponseSchema, error) {
	// Note: FetchAPI doesn't support include parameters
	// Version and publication information would require separate API calls
	res, err := kkClient.FetchAPI(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get API", err, helper.GetCmd(), attrs...)
	}

	return res.GetAPIResponseSchema(), nil
}

func (c *getAPICmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing APIs requires 0 or 1 arguments (name or ID)"),
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

func (c *getAPICmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	// 'get apis' can be run in various ways:
	//	> get apis <id>    # Get by UUID
	//  > get apis <name>	# Get by name
	//  > get apis         # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := strings.TrimSpace(helper.GetArgs()[0])

		isUUID := util.IsValidUUID(id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the API by name
			api, e := runListByName(id, c.includeVersions, c.includePublications, sdk.GetAPIAPI(), helper, cfg)
			if e == nil {
				if outType == cmdCommon.TEXT {
					printer.Print(apiToDisplayRecord(api, c.includeVersions, c.includePublications))
				} else {
					printer.Print(api)
				}
			} else {
				return e
			}
		} else {
			api, e := runGet(id, c.includeVersions, c.includePublications, sdk.GetAPIAPI(), helper)
			if e == nil {
				if outType == cmdCommon.TEXT {
					printer.Print(apiToDisplayRecord(api, c.includeVersions, c.includePublications))
				} else {
					printer.Print(api)
				}
			} else {
				return e
			}
		}
	} else { // list all APIs
		var apis []kkComps.APIResponseSchema
		apis, e = runList(c.includeVersions, c.includePublications, sdk.GetAPIAPI(), helper, cfg)
		if e == nil {
			if outType == cmdCommon.TEXT {
				var displayRecords []textDisplayRecord
				for _, api := range apis {
					displayRecords = append(displayRecords, apiToDisplayRecord(&api, c.includeVersions, c.includePublications))
				}
				printer.Print(displayRecords)
			} else {
				printer.Print(apis)
			}
		} else {
			return e
		}
	}

	return nil
}

func newGetAPICmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getAPICmd {
	rv := getAPICmd{
		Command: baseCmd,
	}

	rv.Short = getAPIsShort
	rv.Long = getAPIsLong
	rv.Example = getAPIsExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	// Add include flags
	rv.Flags().BoolVar(&rv.includeVersions, includeVersionsFlagName, false,
		i18n.T("root.products.konnect.api.includeVersionsDesc",
			"Include API versions in the response"))
	rv.Flags().BoolVar(&rv.includePublications, includePublicationsFlagName, false,
		i18n.T("root.products.konnect.api.includePublicationsDesc",
			"Include API publications in the response"))

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
