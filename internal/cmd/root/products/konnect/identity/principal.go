package identity

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	directoryIDFlagName   = "directory-id"
	directoryNameFlagName = "directory-name"
	principalIDFlagName   = "principal-id"
)

var (
	principalsShort = i18n.T("root.products.konnect.identity.principalsShort",
		"List or get Kong Identity principals")
	principalsLong = normalizers.LongDesc(i18n.T("root.products.konnect.identity.principalsLong",
		`Use the principals command to list or retrieve principals for a specific Kong Identity directory.`))
	principalsExample = normalizers.Examples(
		i18n.T("root.products.konnect.identity.principalsExamples",
			fmt.Sprintf(`
# List principals for a directory by ID
%[1]s get identity directory principals --directory-id <directory-id>
# List principals for a directory by name
%[1]s get identity directory principals --directory-name workforce
# Get a specific principal by ID
%[1]s get identity directory principals --directory-id <directory-id> <principal-id>
`, meta.CLIName)),
	)
)

type principalTextRecord struct {
	ID               string
	DisplayName      string
	Description      string
	ProvisionedBy    string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type principalResource struct {
	ID            string            `json:"id"                       yaml:"id"`
	DirectoryID   string            `json:"directory_id"             yaml:"directory_id"`
	DisplayName   *string           `json:"display_name,omitempty"   yaml:"display_name,omitempty"`
	Description   string            `json:"description,omitempty"    yaml:"description,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"       yaml:"metadata,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"         yaml:"labels,omitempty"`
	ManagedBy     map[string]string `json:"managed_by,omitempty"     yaml:"managed_by,omitempty"`
	ProvisionedBy string            `json:"provisioned_by,omitempty" yaml:"provisioned_by,omitempty"`
	CreatedAt     *time.Time        `json:"created_at,omitempty"     yaml:"created_at,omitempty"`
	UpdatedAt     *time.Time        `json:"updated_at,omitempty"     yaml:"updated_at,omitempty"`
}

type getPrincipalCmd struct {
	*cobra.Command
}

func normalizePrincipal(directoryID string, principal kkComps.KongPrincipal) principalResource {
	result := principalResource{
		ID:          principal.ID,
		DirectoryID: directoryID,
		DisplayName: principal.DisplayName,
		Description: principal.Description,
		Metadata:    normalizeIdentityMetadata(principal.Metadata),
		Labels:      principal.Labels,
		ManagedBy:   principal.ManagedBy,
	}
	if principal.ProvisionedBy != nil {
		result.ProvisionedBy = string(*principal.ProvisionedBy)
	}
	if !principal.CreatedAt.IsZero() {
		created := principal.CreatedAt
		result.CreatedAt = &created
	}
	if !principal.UpdatedAt.IsZero() {
		updated := principal.UpdatedAt
		result.UpdatedAt = &updated
	}
	return result
}

func normalizeIdentityMetadata(metadata map[string]kkComps.KongIdentityMetadata) map[string]any {
	if metadata == nil {
		return nil
	}
	result := make(map[string]any, len(metadata))
	for key, value := range metadata {
		result[key] = normalizeIdentityMetadataValue(value)
	}
	return result
}

func normalizeIdentityMetadataValue(value kkComps.KongIdentityMetadata) any {
	switch {
	case value.Boolean != nil:
		return *value.Boolean
	case value.Str != nil:
		return *value.Str
	case value.Integer != nil:
		return *value.Integer
	case value.ArrayOfStr != nil:
		return slices.Clone(value.ArrayOfStr)
	case value.ArrayOfInteger != nil:
		return slices.Clone(value.ArrayOfInteger)
	default:
		return nil
	}
}

func principalToDisplayRecord(principal principalResource) principalTextRecord {
	const missing = "n/a"

	record := principalTextRecord{
		ID:               missing,
		DisplayName:      missing,
		Description:      missing,
		ProvisionedBy:    missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}
	if strings.TrimSpace(principal.ID) != "" {
		record.ID = util.AbbreviateUUID(principal.ID)
	}
	if principal.DisplayName != nil && strings.TrimSpace(*principal.DisplayName) != "" {
		record.DisplayName = *principal.DisplayName
	}
	if strings.TrimSpace(principal.Description) != "" {
		record.Description = principal.Description
	}
	if strings.TrimSpace(principal.ProvisionedBy) != "" {
		record.ProvisionedBy = principal.ProvisionedBy
	}
	if principal.CreatedAt != nil && !principal.CreatedAt.IsZero() {
		record.LocalCreatedTime = principal.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if principal.UpdatedAt != nil && !principal.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = principal.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func principalDetailView(principal principalResource) string {
	const missing = "n/a"

	displayName := missing
	if principal.DisplayName != nil && strings.TrimSpace(*principal.DisplayName) != "" {
		displayName = *principal.DisplayName
	}
	created := missing
	if principal.CreatedAt != nil && !principal.CreatedAt.IsZero() {
		created = principal.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	updated := missing
	if principal.UpdatedAt != nil && !principal.UpdatedAt.IsZero() {
		updated = principal.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", valueOrMissing(principal.ID))
	fmt.Fprintf(&b, "directory_id: %s\n", valueOrMissing(principal.DirectoryID))
	fmt.Fprintf(&b, "display_name: %s\n", displayName)
	fmt.Fprintf(&b, "description: %s\n", valueOrMissing(principal.Description))
	fmt.Fprintf(&b, "metadata: %s\n", summarizeAnyMap(principal.Metadata, missing))
	fmt.Fprintf(&b, "labels: %s\n", summarizeMap(principal.Labels))
	fmt.Fprintf(&b, "managed_by: %s\n", summarizeMap(principal.ManagedBy))
	fmt.Fprintf(&b, "provisioned_by: %s\n", valueOrMissing(principal.ProvisionedBy))
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return strings.TrimRight(b.String(), "\n")
}

func summarizeAnyMap(values map[string]any, missing string) string {
	switch {
	case values == nil:
		return missing
	case len(values) == 0:
		return "{}"
	default:
		keys := slices.Sorted(maps.Keys(values))
		pairs := make([]string, 0, len(values))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, summarizeAnyValue(values[key])))
		}
		return strings.Join(pairs, ", ")
	}
}

func summarizeAnyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(b)
	}
}

func addIdentityDirectoryScopeFlags(c *cobra.Command) {
	c.Flags().String(directoryIDFlagName, "", "Directory ID")
	c.Flags().String(directoryNameFlagName, "", "Directory name")
}

func resolveIdentityDirectoryID(
	helper cmd.Helper,
	directoryAPI helpers.IdentityDirectoryAPI,
	cfg config.Hook,
) (string, error) {
	directoryID, _ := helper.GetCmd().Flags().GetString(directoryIDFlagName)
	directoryName, _ := helper.GetCmd().Flags().GetString(directoryNameFlagName)
	directoryID = strings.TrimSpace(directoryID)
	directoryName = strings.TrimSpace(directoryName)

	if directoryID != "" && directoryName != "" {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", directoryIDFlagName, directoryNameFlagName),
		}
	}
	if directoryID != "" {
		return directoryID, nil
	}
	if directoryName == "" {
		directory, err := runDefaultDirectoryLookup(directoryAPI, helper)
		if err != nil {
			return "", err
		}
		return directory.ID, nil
	}

	directory, err := runDirectoryByIdentifier(directoryName, directoryAPI, helper, cfg)
	if err != nil {
		return "", err
	}
	return directory.ID, nil
}

func runDefaultDirectoryLookup(
	api helpers.IdentityDirectoryAPI,
	helper cmd.Helper,
) (*directoryResource, error) {
	pageSize := int64(2)
	sortByName := "name"
	page := &kkComps.CursorPageParameters{Size: &pageSize}

	res, err := api.ListKongDirectories(helper.GetContext(), page, &sortByName)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list identity directories", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.GetListKongDirectories() == nil {
		return nil, directoryIdentifierRequiredError("no identity directories were found")
	}

	directories := res.GetListKongDirectories().GetData()
	switch len(directories) {
	case 0:
		return nil, directoryIdentifierRequiredError("no identity directories were found")
	case 1:
		directory := normalizeDirectory(directories[0])
		return &directory, nil
	default:
		return nil, directoryIdentifierRequiredError("multiple identity directories exist")
	}
}

func directoryIdentifierRequiredError(reason string) *cmd.ConfigurationError {
	return &cmd.ConfigurationError{
		Err: fmt.Errorf(
			"a directory identifier is required because %s. Provide --%s or --%s",
			reason,
			directoryIDFlagName,
			directoryNameFlagName,
		),
	}
}

func runPrincipalList(
	api helpers.IdentityPrincipalAPI,
	helper cmd.Helper,
	cfg config.Hook,
	directoryID string,
) ([]principalResource, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principals client is not available")
	}

	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}
	sortByDisplayName := "display_name"
	var pageAfter *string
	var principals []principalResource

	for {
		page := &kkComps.CursorPageParameters{Size: &pageSize}
		if pageAfter != nil {
			page.After = pageAfter
		}

		res, err := api.ListPrincipals(helper.GetContext(), kkOps.ListPrincipalsRequest{
			DirectoryID: directoryID,
			Page:        page,
			Sort:        &sortByDisplayName,
		})
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list identity principals", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.GetListKongPrincipals() == nil {
			return principals, nil
		}

		list := res.GetListKongPrincipals()
		for _, principal := range list.GetData() {
			principals = append(principals, normalizePrincipal(directoryID, principal))
		}

		nextCursor := pagination.ExtractPageAfterCursor(list.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return principals, nil
}

func runPrincipalByID(
	api helpers.IdentityPrincipalAPI,
	helper cmd.Helper,
	directoryID string,
	principalID string,
) (*principalResource, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principals client is not available")
	}

	res, err := api.GetPrincipal(helper.GetContext(), directoryID, principalID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get identity principal", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.GetKongPrincipal() == nil {
		return nil, fmt.Errorf("identity principal with ID %q not found", principalID)
	}

	principal := normalizePrincipal(directoryID, *res.GetKongPrincipal())
	return &principal, nil
}

func (c *getPrincipalCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing identity principals requires 0 or 1 arguments (principal ID)"),
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

func (c *getPrincipalCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
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

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	directoryID, err := resolveIdentityDirectoryID(helper, sdk.GetIdentityDirectoryAPI(), cfg)
	if err != nil {
		return err
	}

	if len(helper.GetArgs()) == 1 {
		principal, err := runPrincipalByID(
			sdk.GetIdentityPrincipalAPI(),
			helper,
			directoryID,
			strings.TrimSpace(helper.GetArgs()[0]),
		)
		if err != nil {
			return err
		}

		return tableview.RenderForFormat(
			helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			principalToDisplayRecord(*principal),
			principal,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	principals, err := runPrincipalList(sdk.GetIdentityPrincipalAPI(), helper, cfg, directoryID)
	if err != nil {
		return err
	}

	return renderPrincipalList(helper, helper.GetCmd().Name(), outType, printer, principals)
}

func renderPrincipalList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	principals []principalResource,
) error {
	displayRecords := make([]principalTextRecord, 0, len(principals))
	for i := range principals {
		displayRecords = append(displayRecords, principalToDisplayRecord(principals[i]))
	}

	childView := buildPrincipalChildView(principals)
	options := []tableview.Option{
		tableview.WithCustomTable(childView.Headers, childView.Rows),
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	}
	if childView.DetailRenderer != nil {
		options = append(options, tableview.WithDetailRenderer(childView.DetailRenderer))
	}
	if childView.DetailContext != nil {
		options = append(options, tableview.WithDetailContext(childView.ParentType, childView.DetailContext))
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		principals,
		"",
		options...,
	)
}

func buildPrincipalChildView(principals []principalResource) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(principals))
	for i := range principals {
		record := principalToDisplayRecord(principals[i])
		tableRows = append(tableRows, table.Row{record.ID, record.DisplayName, record.Description})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(principals) {
			return ""
		}
		return principalDetailView(principals[index])
	}

	return tableview.ChildView{
		Headers:        []string{"id", "display_name", "description"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Identity Principals",
		ParentType:     common.ViewParentIdentityPrincipal,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(principals) {
				return nil
			}
			return principals[index]
		},
	}
}

func newPrincipalCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	baseCmd := &cobra.Command{
		Use:     "principals [principal-id]",
		Short:   principalsShort,
		Long:    principalsLong,
		Example: principalsExample,
		Aliases: []string{"principal"},
	}

	rv := getPrincipalCmd{Command: baseCmd}
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	addIdentityDirectoryScopeFlags(rv.Command)

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	identityCmd := newPrincipalIdentityCmd(verb, addParentFlags, parentPreRun)
	rv.AddCommand(identityCmd)

	return rv.Command
}
