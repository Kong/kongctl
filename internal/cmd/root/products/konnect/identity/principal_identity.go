package identity

import (
	"fmt"
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

var (
	principalIdentitiesShort = i18n.T("root.products.konnect.identity.principalIdentitiesShort",
		"List or get Kong Identity principal identities")
	principalIdentitiesLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.identity.principalIdentitiesLong",
		`Use the identities command to list or retrieve identities for a specific Kong Identity principal.`,
	))
	principalIdentitiesExample = normalizers.Examples(
		i18n.T("root.products.konnect.identity.principalIdentitiesExamples",
			fmt.Sprintf(`
# List identities for a principal
%[1]s get identity directory principals identities --directory-id <directory-id> --principal-id <principal-id>
# Get a specific principal identity by ID
%[1]s get identity directory principals identities \
  --directory-id <directory-id> --principal-id <principal-id> <identity-id>
`, meta.CLIName)),
	)
)

type principalIdentityTextRecord struct {
	ID               string
	Type             string
	Summary          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type principalIdentityResource struct {
	ID             string            `json:"id"                          yaml:"id"`
	DirectoryID    string            `json:"directory_id"                yaml:"directory_id"`
	PrincipalID    string            `json:"principal_id"                yaml:"principal_id"`
	Type           string            `json:"type"                        yaml:"type"`
	Labels         map[string]string `json:"labels,omitempty"            yaml:"labels,omitempty"`
	Issuer         string            `json:"issuer,omitempty"            yaml:"issuer,omitempty"`
	ClaimName      string            `json:"claim_name,omitempty"        yaml:"claim_name,omitempty"`
	ClaimValue     string            `json:"claim_value,omitempty"       yaml:"claim_value,omitempty"`
	AuthServerID   string            `json:"auth_server_id,omitempty"    yaml:"auth_server_id,omitempty"`
	ClientID       string            `json:"client_id,omitempty"         yaml:"client_id,omitempty"`
	ControlPlaneID string            `json:"control_plane_id,omitempty" yaml:"control_plane_id,omitempty"`
	ConsumerID     string            `json:"consumer_id,omitempty"       yaml:"consumer_id,omitempty"`
	WorkspaceID    *string           `json:"workspace_id,omitempty"      yaml:"workspace_id,omitempty"`
	Key            string            `json:"key,omitempty"               yaml:"key,omitempty"`
	Value          string            `json:"value,omitempty"             yaml:"value,omitempty"`
	CreatedAt      *time.Time        `json:"created_at,omitempty"        yaml:"created_at,omitempty"`
	UpdatedAt      *time.Time        `json:"updated_at,omitempty"        yaml:"updated_at,omitempty"`
}

type getPrincipalIdentityCmd struct {
	*cobra.Command
}

func normalizePrincipalIdentity(
	directoryID string,
	principalID string,
	identity kkComps.KongPrincipalIdentity,
) principalIdentityResource {
	result := principalIdentityResource{
		DirectoryID: directoryID,
		PrincipalID: principalID,
		Type:        string(identity.Type),
	}

	switch identity.Type {
	case kkComps.KongPrincipalIdentityTypeOidc:
		if oidc := identity.KongPrincipalIdentityOIDCResponse; oidc != nil {
			result.ID = oidc.ID
			result.Labels = oidc.Labels
			result.Issuer = oidc.Issuer
			result.ClaimName = oidc.Claim.Name
			result.ClaimValue = oidc.Claim.Value
			setPrincipalIdentityTimes(&result, oidc.CreatedAt, oidc.UpdatedAt)
		}
	case kkComps.KongPrincipalIdentityTypeAuthServerClient:
		if authServerClient := identity.KongPrincipalIdentityAuthServerClientResponse; authServerClient != nil {
			result.ID = authServerClient.ID
			result.Labels = authServerClient.Labels
			result.AuthServerID = authServerClient.AuthServerID
			result.ClientID = authServerClient.ClientID
			setPrincipalIdentityTimes(&result, authServerClient.CreatedAt, authServerClient.UpdatedAt)
		}
	case kkComps.KongPrincipalIdentityTypeControlPlaneConsumer:
		if consumer := identity.KongPrincipalIdentityCPConsumerResponse; consumer != nil {
			result.ID = consumer.ID
			result.Labels = consumer.Labels
			result.ControlPlaneID = consumer.ControlPlaneID
			result.ConsumerID = consumer.ConsumerID
			result.WorkspaceID = consumer.WorkspaceID
			setPrincipalIdentityTimes(&result, consumer.CreatedAt, consumer.UpdatedAt)
		}
	case kkComps.KongPrincipalIdentityTypeCustom:
		if custom := identity.KongPrincipalIdentityCustomResponse; custom != nil {
			result.ID = custom.ID
			result.Labels = custom.Labels
			result.Key = custom.Key
			result.Value = custom.Value
			setPrincipalIdentityTimes(&result, custom.CreatedAt, custom.UpdatedAt)
		}
	}

	return result
}

func setPrincipalIdentityTimes(result *principalIdentityResource, created time.Time, updated time.Time) {
	if !created.IsZero() {
		result.CreatedAt = &created
	}
	if !updated.IsZero() {
		result.UpdatedAt = &updated
	}
}

func principalIdentityToDisplayRecord(identity principalIdentityResource) principalIdentityTextRecord {
	const missing = "n/a"

	record := principalIdentityTextRecord{
		ID:               missing,
		Type:             missing,
		Summary:          missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}
	if strings.TrimSpace(identity.ID) != "" {
		record.ID = util.AbbreviateUUID(identity.ID)
	}
	if strings.TrimSpace(identity.Type) != "" {
		record.Type = identity.Type
	}
	if summary := principalIdentitySummary(identity); summary != "" {
		record.Summary = summary
	}
	if identity.CreatedAt != nil && !identity.CreatedAt.IsZero() {
		record.LocalCreatedTime = identity.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if identity.UpdatedAt != nil && !identity.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = identity.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func principalIdentitySummary(identity principalIdentityResource) string {
	switch identity.Type {
	case string(kkComps.KongPrincipalIdentityTypeOidc):
		return strings.TrimSpace(fmt.Sprintf("%s %s=%s", identity.Issuer, identity.ClaimName, identity.ClaimValue))
	case string(kkComps.KongPrincipalIdentityTypeAuthServerClient):
		return strings.TrimSpace(fmt.Sprintf("auth_server_id=%s client_id=%s", identity.AuthServerID, identity.ClientID))
	case string(kkComps.KongPrincipalIdentityTypeControlPlaneConsumer):
		return strings.TrimSpace(fmt.Sprintf("control_plane_id=%s consumer_id=%s",
			identity.ControlPlaneID, identity.ConsumerID))
	case string(kkComps.KongPrincipalIdentityTypeCustom):
		return strings.TrimSpace(fmt.Sprintf("%s=%s", identity.Key, identity.Value))
	default:
		return ""
	}
}

func principalIdentityDetailView(identity principalIdentityResource) string {
	const missing = "n/a"

	created := missing
	if identity.CreatedAt != nil && !identity.CreatedAt.IsZero() {
		created = identity.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	updated := missing
	if identity.UpdatedAt != nil && !identity.UpdatedAt.IsZero() {
		updated = identity.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	workspaceID := missing
	if identity.WorkspaceID != nil && strings.TrimSpace(*identity.WorkspaceID) != "" {
		workspaceID = *identity.WorkspaceID
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", valueOrMissing(identity.ID))
	fmt.Fprintf(&b, "directory_id: %s\n", valueOrMissing(identity.DirectoryID))
	fmt.Fprintf(&b, "principal_id: %s\n", valueOrMissing(identity.PrincipalID))
	fmt.Fprintf(&b, "type: %s\n", valueOrMissing(identity.Type))
	fmt.Fprintf(&b, "labels: %s\n", summarizeMap(identity.Labels))

	switch identity.Type {
	case string(kkComps.KongPrincipalIdentityTypeOidc):
		fmt.Fprintf(&b, "issuer: %s\n", valueOrMissing(identity.Issuer))
		fmt.Fprintf(&b, "claim.name: %s\n", valueOrMissing(identity.ClaimName))
		fmt.Fprintf(&b, "claim.value: %s\n", valueOrMissing(identity.ClaimValue))
	case string(kkComps.KongPrincipalIdentityTypeAuthServerClient):
		fmt.Fprintf(&b, "auth_server_id: %s\n", valueOrMissing(identity.AuthServerID))
		fmt.Fprintf(&b, "client_id: %s\n", valueOrMissing(identity.ClientID))
	case string(kkComps.KongPrincipalIdentityTypeControlPlaneConsumer):
		fmt.Fprintf(&b, "control_plane_id: %s\n", valueOrMissing(identity.ControlPlaneID))
		fmt.Fprintf(&b, "consumer_id: %s\n", valueOrMissing(identity.ConsumerID))
		fmt.Fprintf(&b, "workspace_id: %s\n", workspaceID)
	case string(kkComps.KongPrincipalIdentityTypeCustom):
		fmt.Fprintf(&b, "key: %s\n", valueOrMissing(identity.Key))
		fmt.Fprintf(&b, "value: %s\n", valueOrMissing(identity.Value))
	}

	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return strings.TrimRight(b.String(), "\n")
}

func resolvePrincipalID(helper cmd.Helper) (string, error) {
	principalID, _ := helper.GetCmd().Flags().GetString(principalIDFlagName)
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("a principal identifier is required. Provide --%s", principalIDFlagName),
		}
	}
	return principalID, nil
}

func runPrincipalIdentityList(
	api helpers.IdentityPrincipalIdentityAPI,
	helper cmd.Helper,
	cfg config.Hook,
	directoryID string,
	principalID string,
) ([]principalIdentityResource, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principal identities client is not available")
	}

	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}
	sortByType := "type"
	var pageAfter *string
	var identities []principalIdentityResource

	for {
		page := &kkComps.CursorPageParameters{Size: &pageSize}
		if pageAfter != nil {
			page.After = pageAfter
		}

		res, err := api.ListIdentities(helper.GetContext(), kkOps.ListIdentitiesRequest{
			DirectoryID: directoryID,
			PrincipalID: principalID,
			Page:        page,
			Sort:        &sortByType,
		})
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list identity principal identities", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.GetListKongPrincipalIdentities() == nil {
			return identities, nil
		}

		list := res.GetListKongPrincipalIdentities()
		for _, identity := range list.GetData() {
			identities = append(identities, normalizePrincipalIdentity(directoryID, principalID, identity))
		}

		nextCursor := pagination.ExtractPageAfterCursor(list.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return identities, nil
}

func runPrincipalIdentityByID(
	api helpers.IdentityPrincipalIdentityAPI,
	helper cmd.Helper,
	directoryID string,
	principalID string,
	identityID string,
) (*principalIdentityResource, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principal identities client is not available")
	}

	res, err := api.GetIdentity(helper.GetContext(), kkOps.GetIdentityRequest{
		DirectoryID: directoryID,
		PrincipalID: principalID,
		IdentityID:  identityID,
	})
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get identity principal identity", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.GetKongPrincipalIdentity() == nil {
		return nil, fmt.Errorf("identity principal identity with ID %q not found", identityID)
	}

	identity := normalizePrincipalIdentity(directoryID, principalID, *res.GetKongPrincipalIdentity())
	return &identity, nil
}

func (c *getPrincipalIdentityCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing identity principal identities requires 0 or 1 arguments (identity ID)"),
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

func (c *getPrincipalIdentityCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	principalID, err := resolvePrincipalID(helper)
	if err != nil {
		return err
	}

	if len(helper.GetArgs()) == 1 {
		identity, err := runPrincipalIdentityByID(
			sdk.GetIdentityPrincipalIdentityAPI(),
			helper,
			directoryID,
			principalID,
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
			principalIdentityToDisplayRecord(*identity),
			identity,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	identities, err := runPrincipalIdentityList(
		sdk.GetIdentityPrincipalIdentityAPI(),
		helper,
		cfg,
		directoryID,
		principalID,
	)
	if err != nil {
		return err
	}

	return renderPrincipalIdentityList(helper, helper.GetCmd().Name(), outType, printer, identities)
}

func renderPrincipalIdentityList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	identities []principalIdentityResource,
) error {
	displayRecords := make([]principalIdentityTextRecord, 0, len(identities))
	for i := range identities {
		displayRecords = append(displayRecords, principalIdentityToDisplayRecord(identities[i]))
	}

	childView := buildPrincipalIdentityChildView(identities)
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
		identities,
		"",
		options...,
	)
}

func buildPrincipalIdentityChildView(identities []principalIdentityResource) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(identities))
	for i := range identities {
		record := principalIdentityToDisplayRecord(identities[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Type, record.Summary})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(identities) {
			return ""
		}
		return principalIdentityDetailView(identities[index])
	}

	return tableview.ChildView{
		Headers:        []string{"id", "type", "summary"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Identity Principal Identities",
		ParentType:     common.ViewParentIdentityPrincipalIdentity,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(identities) {
				return nil
			}
			return identities[index]
		},
	}
}

func newPrincipalIdentityCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	baseCmd := &cobra.Command{
		Use:     "identities [identity-id]",
		Short:   principalIdentitiesShort,
		Long:    principalIdentitiesLong,
		Example: principalIdentitiesExample,
		Aliases: []string{"identity"},
	}

	rv := getPrincipalIdentityCmd{Command: baseCmd}
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	addIdentityDirectoryScopeFlags(rv.Command)
	rv.Flags().String(principalIDFlagName, "", "Principal ID")

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return rv.Command
}
