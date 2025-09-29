package api

import (
	"fmt"
	"strings"
	"time"

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
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	versionsCommandName = "versions"
)

type apiVersionSummaryRecord struct {
	ID               string
	Version          string
	SpecType         string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type apiVersionDetailRecord struct {
	ID               string
	Version          string
	SpecType         string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	versionsUse = versionsCommandName

	versionsShort = i18n.T("root.products.konnect.api.versionsShort",
		"Manage API versions for a Konnect API")
	versionsLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.versionsLong",
		`Use the versions command to list or retrieve API versions for a specific Konnect API.`))
	versionsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.versionsExamples",
			fmt.Sprintf(`
# List versions for an API by ID
%[1]s get api versions --api-id <api-id>
# List versions for an API by name
%[1]s get api versions --api-name my-api
# Get a specific version by ID
%[1]s get api versions --api-id <api-id> <version-id>
# Get a specific version by semantic version
%[1]s get api versions --api-id <api-id> 1.0.0
`, meta.CLIName)))
)

func newGetAPIVersionsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     versionsUse,
		Short:   versionsShort,
		Long:    versionsLong,
		Example: versionsExample,
		Aliases: []string{"version", "vs", "ver"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindAPIChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := apiVersionsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addAPIChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type apiVersionsHandler struct {
	cmd *cobra.Command
}

func (h apiVersionsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing API versions requires 0 or 1 arguments (ID or version string)",
			),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	apiID, apiName := getAPIIdentifiers(cfg)
	if apiID != "" && apiName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", apiIDFlagName, apiNameFlagName),
		}
	}

	if apiID == "" && apiName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("an API identifier is required. Provide --%s or --%s", apiIDFlagName, apiNameFlagName),
		}
	}

	if apiID == "" {
		apiID, err = resolveAPIIDByName(apiName, sdk.GetAPIAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	apiVersionAPI := sdk.GetAPIVersionAPI()
	if apiVersionAPI == nil {
		return &cmd.ExecutionError{
			Msg: "API versions client is not available",
			Err: fmt.Errorf("api versions client not configured"),
		}
	}

	if len(args) == 1 {
		return h.getSingleVersion(helper, apiVersionAPI, apiID, strings.TrimSpace(args[0]), outType, printer, cfg)
	}

	return h.listVersions(helper, apiVersionAPI, apiID, outType, printer, cfg)
}

func (h apiVersionsHandler) listVersions(
	helper cmd.Helper,
	apiVersionAPI helpers.APIVersionAPI,
	apiID string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
	cfg config.Hook,
) error {
	summaries, err := fetchVersionSummaries(helper, apiVersionAPI, apiID, cfg)
	if err != nil {
		return err
	}

	if outType == cmdCommon.TEXT {
		records := make([]apiVersionSummaryRecord, 0, len(summaries))
		for _, summary := range summaries {
			records = append(records, versionSummaryToRecord(summary))
		}
		printer.Print(records)
		return nil
	}

	printer.Print(summaries)
	return nil
}

func (h apiVersionsHandler) getSingleVersion(
	helper cmd.Helper,
	apiVersionAPI helpers.APIVersionAPI,
	apiID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
	cfg config.Hook,
) error {
	var versionID string

	if util.IsValidUUID(identifier) {
		versionID = identifier
	} else {
		summaries, err := fetchVersionSummaries(helper, apiVersionAPI, apiID, cfg)
		if err != nil {
			return err
		}
		match := findVersionByString(summaries, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("version %q not found", identifier),
			}
		}
		versionID = match.ID
	}

	res, err := apiVersionAPI.FetchAPIVersion(helper.GetContext(), apiID, versionID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get API version", err, helper.GetCmd(), attrs...)
	}

	version := res.GetAPIVersionResponse()
	if version == nil {
		return &cmd.ExecutionError{
			Msg: "API version response was empty",
			Err: fmt.Errorf("no version returned for id %s", versionID),
		}
	}

	if outType == cmdCommon.TEXT {
		printer.Print(versionDetailToRecord(version))
		return nil
	}

	printer.Print(version)
	return nil
}

func fetchVersionSummaries(
	helper cmd.Helper,
	apiVersionAPI helpers.APIVersionAPI,
	apiID string,
	cfg config.Hook,
) ([]kkComps.ListAPIVersionResponseAPIVersionSummary, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.ListAPIVersionResponseAPIVersionSummary

	for {
		req := kkOps.ListAPIVersionsRequest{
			APIID:      apiID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := apiVersionAPI.ListAPIVersions(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list API versions", err, helper.GetCmd(), attrs...)
		}

		if res.GetListAPIVersionResponse() == nil {
			break
		}

		data := res.GetListAPIVersionResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListAPIVersionResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func findVersionByString(
	summaries []kkComps.ListAPIVersionResponseAPIVersionSummary,
	identifier string,
) *kkComps.ListAPIVersionResponseAPIVersionSummary {
	lowered := strings.ToLower(identifier)
	for i := range summaries {
		if strings.ToLower(summaries[i].GetVersion()) == lowered {
			return &summaries[i]
		}
	}
	return nil
}

func versionSummaryToRecord(summary kkComps.ListAPIVersionResponseAPIVersionSummary) apiVersionSummaryRecord {
	specType := valueNA
	if summary.GetSpec() != nil && summary.GetSpec().GetType() != nil {
		specType = string(*summary.GetSpec().GetType())
	}

	return apiVersionSummaryRecord{
		ID:               summary.GetID(),
		Version:          summary.GetVersion(),
		SpecType:         specType,
		LocalCreatedTime: summary.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: summary.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func versionDetailToRecord(version *kkComps.APIVersionResponse) apiVersionDetailRecord {
	specType := valueNA
	if version.GetSpec() != nil && version.GetSpec().GetType() != nil {
		specType = string(*version.GetSpec().GetType())
	}

	return apiVersionDetailRecord{
		ID:               version.GetID(),
		Version:          version.GetVersion(),
		SpecType:         specType,
		LocalCreatedTime: version.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: version.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}
