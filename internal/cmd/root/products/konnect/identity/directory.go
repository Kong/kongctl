package identity

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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

var (
	getDirectoriesShort = i18n.T("root.products.konnect.identity.getDirectoriesShort",
		"List or get Kong Identity directories")
	getDirectoriesLong = i18n.T("root.products.konnect.identity.getDirectoriesLong",
		`Use the get verb with the identity directory command to query Kong Identity directories.`)
	getDirectoriesExample = normalizers.Examples(
		i18n.T("root.products.konnect.identity.getDirectoriesExamples",
			fmt.Sprintf(`
	# List all Kong Identity directories
	%[1]s get identity directories
	# Get details for a Kong Identity directory with a specific ID
	%[1]s get identity directory 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for a Kong Identity directory with a specific name
	%[1]s get identity directory workforce
	`, meta.CLIName)),
	)
)

func directoryCommandLong(verb verbs.VerbValue) string {
	if verb == verbs.List {
		return i18n.T("root.products.konnect.identity.listDirectoriesLong",
			`Use the list verb with the identity directory command to query Kong Identity directories.`)
	}
	return getDirectoriesLong
}

func directoryCommandExample(verb verbs.VerbValue) string {
	if verb == verbs.List {
		return normalizers.Examples(
			i18n.T("root.products.konnect.identity.listDirectoriesExamples",
				fmt.Sprintf(`
	# List all Kong Identity directories
	%[1]s list identity directories
	`, meta.CLIName)),
		)
	}
	return getDirectoriesExample
}

type directoryTextRecord struct {
	ID                    string
	Name                  string
	Description           string
	AllowAllControlPlanes string
	AllowedControlPlanes  string
	TTLSecs               string
	NegativeTTLSecs       string
	LocalCreatedTime      string
	LocalUpdatedTime      string
}

type directoryRealmConfig struct {
	TTL            *int64   `json:"ttl,omitempty"             yaml:"ttl,omitempty"`
	NegativeTTL    *int64   `json:"negative_ttl,omitempty"    yaml:"negative_ttl,omitempty"`
	ConsumerGroups []string `json:"consumer_groups,omitempty" yaml:"consumer_groups,omitempty"`
}

type directoryResource struct {
	ID                    string                `json:"id"                                yaml:"id"`
	Name                  string                `json:"name"                              yaml:"name"`
	Description           string                `json:"description,omitempty"             yaml:"description,omitempty"`
	AllowedControlPlanes  []string              `json:"allowed_control_planes,omitempty" yaml:"allowed_control_planes,omitempty"`     //nolint:lll
	AllowAllControlPlanes *bool                 `json:"allow_all_control_planes,omitempty" yaml:"allow_all_control_planes,omitempty"` //nolint:lll
	TTLSecs               *int64                `json:"ttl_secs,omitempty"               yaml:"ttl_secs,omitempty"`
	NegativeTTLSecs       *int64                `json:"negative_ttl_secs,omitempty"      yaml:"negative_ttl_secs,omitempty"` //nolint:lll
	Labels                map[string]string     `json:"labels,omitempty"                 yaml:"labels,omitempty"`
	ManagedBy             map[string]string     `json:"managed_by,omitempty"             yaml:"managed_by,omitempty"`
	RealmConfig           *directoryRealmConfig `json:"realm_config,omitempty"           yaml:"realm_config,omitempty"`
	CreatedAt             *time.Time            `json:"created_at,omitempty"             yaml:"created_at,omitempty"`
	UpdatedAt             *time.Time            `json:"updated_at,omitempty"             yaml:"updated_at,omitempty"`
}

type getDirectoryCmd struct {
	*cobra.Command
}

func normalizeDirectory(directory kkComps.KongDirectory) directoryResource {
	result := directoryResource{
		ID:                    directory.ID,
		Name:                  directory.Name,
		Description:           directory.Description,
		AllowedControlPlanes:  slices.Clone(directory.AllowedControlPlanes),
		AllowAllControlPlanes: directory.AllowAllControlPlanes,
		TTLSecs:               directory.TTLSecs,
		NegativeTTLSecs:       directory.NegativeTTLSecs,
		Labels:                directory.Labels,
		ManagedBy:             directory.ManagedBy,
	}
	if !directory.CreatedAt.IsZero() {
		created := directory.CreatedAt
		result.CreatedAt = &created
	}
	if !directory.UpdatedAt.IsZero() {
		updated := directory.UpdatedAt
		result.UpdatedAt = &updated
	}
	return result
}

func normalizeRealmConfig(realm *kkComps.KongDirectoryRealmValues) *directoryRealmConfig {
	if realm == nil {
		return nil
	}
	return &directoryRealmConfig{
		TTL:            realm.TTL,
		NegativeTTL:    realm.NegativeTTL,
		ConsumerGroups: slices.Clone(realm.ConsumerGroups),
	}
}

func directoryToDisplayRecord(directory directoryResource) directoryTextRecord {
	const missing = "n/a"

	record := directoryTextRecord{
		ID:                    missing,
		Name:                  missing,
		Description:           missing,
		AllowAllControlPlanes: missing,
		AllowedControlPlanes:  missing,
		TTLSecs:               missing,
		NegativeTTLSecs:       missing,
		LocalCreatedTime:      missing,
		LocalUpdatedTime:      missing,
	}
	if strings.TrimSpace(directory.ID) != "" {
		record.ID = util.AbbreviateUUID(directory.ID)
	}
	if strings.TrimSpace(directory.Name) != "" {
		record.Name = directory.Name
	}
	if strings.TrimSpace(directory.Description) != "" {
		record.Description = directory.Description
	}
	if directory.AllowAllControlPlanes != nil {
		record.AllowAllControlPlanes = fmt.Sprintf("%t", *directory.AllowAllControlPlanes)
	}
	if len(directory.AllowedControlPlanes) > 0 {
		record.AllowedControlPlanes = strings.Join(directory.AllowedControlPlanes, ", ")
	}
	if directory.TTLSecs != nil {
		record.TTLSecs = fmt.Sprintf("%d", *directory.TTLSecs)
	}
	if directory.NegativeTTLSecs != nil {
		record.NegativeTTLSecs = fmt.Sprintf("%d", *directory.NegativeTTLSecs)
	}
	if directory.CreatedAt != nil && !directory.CreatedAt.IsZero() {
		record.LocalCreatedTime = directory.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if directory.UpdatedAt != nil && !directory.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = directory.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func summarizeMap(labels map[string]string) string {
	const missing = "n/a"

	switch {
	case labels == nil:
		return missing
	case len(labels) == 0:
		return "{}"
	default:
		keys := slices.Sorted(maps.Keys(labels))
		pairs := make([]string, 0, len(labels))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, labels[key]))
		}
		return strings.Join(pairs, ", ")
	}
}

func summarizeStringSlice(values []string, missing string) string {
	if len(values) == 0 {
		return missing
	}
	return strings.Join(values, ", ")
}

func directoryDetailView(directory directoryResource) string {
	const missing = "n/a"

	boolValue := missing
	if directory.AllowAllControlPlanes != nil {
		boolValue = fmt.Sprintf("%t", *directory.AllowAllControlPlanes)
	}
	ttl := missing
	if directory.TTLSecs != nil {
		ttl = fmt.Sprintf("%d", *directory.TTLSecs)
	}
	negativeTTL := missing
	if directory.NegativeTTLSecs != nil {
		negativeTTL = fmt.Sprintf("%d", *directory.NegativeTTLSecs)
	}
	created := missing
	if directory.CreatedAt != nil && !directory.CreatedAt.IsZero() {
		created = directory.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	updated := missing
	if directory.UpdatedAt != nil && !directory.UpdatedAt.IsZero() {
		updated = directory.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", valueOrMissing(directory.ID))
	fmt.Fprintf(&b, "name: %s\n", valueOrMissing(directory.Name))
	fmt.Fprintf(&b, "description: %s\n", valueOrMissing(directory.Description))
	fmt.Fprintf(&b, "allow_all_control_planes: %s\n", boolValue)
	fmt.Fprintf(&b, "allowed_control_planes: %s\n", summarizeStringSlice(directory.AllowedControlPlanes, missing))
	fmt.Fprintf(&b, "ttl_secs: %s\n", ttl)
	fmt.Fprintf(&b, "negative_ttl_secs: %s\n", negativeTTL)
	fmt.Fprintf(&b, "labels: %s\n", summarizeMap(directory.Labels))
	fmt.Fprintf(&b, "managed_by: %s\n", summarizeMap(directory.ManagedBy))
	if directory.RealmConfig != nil {
		realmTTL := missing
		if directory.RealmConfig.TTL != nil {
			realmTTL = fmt.Sprintf("%d", *directory.RealmConfig.TTL)
		}
		realmNegativeTTL := missing
		if directory.RealmConfig.NegativeTTL != nil {
			realmNegativeTTL = fmt.Sprintf("%d", *directory.RealmConfig.NegativeTTL)
		}
		fmt.Fprintf(&b, "realm_config.ttl: %s\n", realmTTL)
		fmt.Fprintf(&b, "realm_config.negative_ttl: %s\n", realmNegativeTTL)
		fmt.Fprintf(&b, "realm_config.consumer_groups: %s\n",
			summarizeStringSlice(directory.RealmConfig.ConsumerGroups, missing))
	}
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return strings.TrimRight(b.String(), "\n")
}

func valueOrMissing(value string) string {
	const missing = "n/a"

	if strings.TrimSpace(value) == "" {
		return missing
	}
	return value
}

func runDirectoryList(
	api helpers.IdentityDirectoryAPI,
	helper cmd.Helper,
	cfg config.Hook,
) ([]directoryResource, error) {
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}
	sortByName := "name"
	var pageAfter *string
	var directories []directoryResource

	for {
		page := &kkComps.CursorPageParameters{Size: &pageSize}
		if pageAfter != nil {
			page.After = pageAfter
		}

		res, err := api.ListKongDirectories(helper.GetContext(), page, &sortByName)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list identity directories", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.GetListKongDirectories() == nil {
			return directories, nil
		}

		list := res.GetListKongDirectories()
		for _, directory := range list.GetData() {
			directories = append(directories, normalizeDirectory(directory))
		}

		nextCursor := pagination.ExtractPageAfterCursor(list.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return directories, nil
}

func runDirectoryByIdentifier(
	identifier string,
	api helpers.IdentityDirectoryAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*directoryResource, error) {
	identifier = strings.TrimSpace(identifier)
	directories, err := runDirectoryList(api, helper, cfg)
	if err != nil {
		return nil, err
	}

	for i := range directories {
		if directories[i].ID == identifier || directories[i].Name == identifier {
			return &directories[i], nil
		}
	}

	return nil, fmt.Errorf("identity directory with name or ID %q not found", identifier)
}

func (c *getDirectoryCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing identity directories requires 0 or 1 arguments (name or ID)"),
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

func (c *getDirectoryCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	if len(helper.GetArgs()) == 1 {
		directory, err := runDirectoryByIdentifier(helper.GetArgs()[0], sdk.GetIdentityDirectoryAPI(), helper, cfg)
		if err != nil {
			return err
		}

		realm, err := sdk.GetIdentityDirectoryAPI().GetRealmConfig(helper.GetContext(), directory.ID)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get identity directory realm config", err, helper.GetCmd(), attrs...)
		}
		if realm != nil {
			directory.RealmConfig = normalizeRealmConfig(realm.GetKongDirectoryRealmValues())
		}

		return tableview.RenderForFormat(
			helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			directoryToDisplayRecord(*directory),
			directory,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	directories, err := runDirectoryList(sdk.GetIdentityDirectoryAPI(), helper, cfg)
	if err != nil {
		return err
	}

	return renderDirectoryList(helper, helper.GetCmd().Name(), outType, printer, directories)
}

func renderDirectoryList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	directories []directoryResource,
) error {
	displayRecords := make([]directoryTextRecord, 0, len(directories))
	for i := range directories {
		displayRecords = append(displayRecords, directoryToDisplayRecord(directories[i]))
	}

	childView := buildDirectoryChildView(directories)
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
		directories,
		"",
		options...,
	)
}

func buildDirectoryChildView(directories []directoryResource) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(directories))
	for i := range directories {
		record := directoryToDisplayRecord(directories[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.AllowAllControlPlanes})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(directories) {
			return ""
		}
		return directoryDetailView(directories[index])
	}

	return tableview.ChildView{
		Headers:        []string{"id", "name", "allow_all_control_planes"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Identity Directories",
		ParentType:     common.ViewParentIdentityDirectory,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(directories) {
				return nil
			}
			return directories[index]
		},
	}
}

func newDirectoryCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	baseCmd := &cobra.Command{
		Use:     "directory [id|name]",
		Short:   getDirectoriesShort,
		Long:    directoryCommandLong(verb),
		Example: directoryCommandExample(verb),
		Aliases: []string{"directories", "dir", "dirs"},
	}

	rv := getDirectoryCmd{Command: baseCmd}
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	if verb == verbs.Get || verb == verbs.List {
		rv.AddCommand(newPrincipalCmd(verb, addParentFlags, parentPreRun))
	}

	return rv.Command
}
