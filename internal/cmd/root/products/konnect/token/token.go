package token

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	utilviper "github.com/kong/kongctl/internal/util/viper"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	PATCommandName  = "pat"
	SPATCommandName = "spat"

	flagName              = "name"
	flagUserID            = "user-id"
	flagExpiresIn         = "expires-in"
	flagExpiresAt         = "expires-at"
	flagSystemAccountID   = "system-account-id"
	flagSystemAccountName = "system-account-name"

	secondsPerDay = int64(24 * 60 * 60)

	minTokenTTLDays = int64(1)
	maxTokenTTLDays = int64(365)

	minTokenTTLSeconds = minTokenTTLDays * secondsPerDay
	maxTokenTTLSeconds = maxTokenTTLDays * secondsPerDay

	expiresInHelp = "Token lifetime. Use a duration between 1 day and 365 days (12 months). " +
		"Supported units are ns, us, ms, s, m, h, and d (days). Examples: 24h, 36h, 1d, 30d."
	expiresAtHelp = "Token expiration timestamp in RFC3339 format, for example 2026-06-24T12:00:00Z " +
		"or 2026-06-24T12:00:00+02:00. Fractional seconds are accepted. " +
		"Must be between 1 day and 365 days (12 months) from now."
	expiresInExpectation       = "use a valid duration with a unit suffix"
	createExpiresInExpectation = "use a duration between 1 day and 365 days (12 months) " +
		"with units ns, us, ms, s, m, h, or d (days)"
)

type expiration struct {
	ExpiresAt  *time.Time
	TTLSeconds *int64
}

type createTokenRecord struct {
	Type              string `json:"type"                          yaml:"type"`
	ID                string `json:"id,omitempty"                  yaml:"id,omitempty"`
	Name              string `json:"name,omitempty"                yaml:"name,omitempty"`
	Token             string `json:"token"                         yaml:"token"`
	UserID            string `json:"user_id,omitempty"             yaml:"user_id,omitempty"`
	SystemAccountID   string `json:"system_account_id,omitempty"   yaml:"system_account_id,omitempty"`
	SystemAccountName string `json:"system_account_name,omitempty" yaml:"system_account_name,omitempty"`
	State             string `json:"state,omitempty"               yaml:"state,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"          yaml:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"          yaml:"updated_at,omitempty"`
	LastUsedAt        string `json:"last_used_at,omitempty"        yaml:"last_used_at,omitempty"`
	ExpiresAt         string `json:"expires_at,omitempty"          yaml:"expires_at,omitempty"`
}

type getTokenRecord struct {
	Type              string `json:"type"                          yaml:"type"`
	ID                string `json:"id,omitempty"                  yaml:"id,omitempty"`
	Name              string `json:"name,omitempty"                yaml:"name,omitempty"`
	UserID            string `json:"user_id,omitempty"             yaml:"user_id,omitempty"`
	SystemAccountID   string `json:"system_account_id,omitempty"   yaml:"system_account_id,omitempty"`
	SystemAccountName string `json:"system_account_name,omitempty" yaml:"system_account_name,omitempty"`
	State             string `json:"state,omitempty"               yaml:"state,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"          yaml:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"          yaml:"updated_at,omitempty"`
	LastUsedAt        string `json:"last_used_at,omitempty"        yaml:"last_used_at,omitempty"`
	ExpiresAt         string `json:"expires_at,omitempty"          yaml:"expires_at,omitempty"`
}

type deleteTokenRecord struct {
	Type   string `json:"type"           yaml:"type"`
	ID     string `json:"id,omitempty"   yaml:"id,omitempty"`
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Status string `json:"status"         yaml:"status"`
}

type patOptions struct {
	name      string
	userID    string
	expiresIn string
	expiresAt string
}

type spatOptions struct {
	name              string
	systemAccountID   string
	systemAccountName string
	expiresIn         string
	expiresAt         string
}

func NewPATCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     PATCommandName + " [id|name]",
		Short:   "Manage Konnect personal access tokens",
		Aliases: []string{"pats"},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}
	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	opts := &patOptions{}
	if verb == verbs.Create {
		cmd.Use = PATCommandName
		cmd.Short = "Create a Konnect personal access token"
		cmd.Example = fmt.Sprintf(`  %[1]s create pat --name ci --expires-in 30d -o token
  %[1]s create pat --name ci --expires-in 7d --jq -r '.token'`, meta.CLIName)
		cmd.Args = createTokenArgs
		addPATCreateFlags(cmd, opts)
		cmdcommon.AllowExtraOutputFormats(cmd, cmdcommon.TOKEN.String(), cmdcommon.ENV.String())
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return runCreatePAT(c, args, opts)
		}
		return cmd, nil
	}
	if verb == verbs.Get {
		cmd.Short = "List or get Konnect personal access tokens"
		cmd.Example = fmt.Sprintf(`  %[1]s get pat
  %[1]s get pat <id|name>
  %[1]s get pat --jq '.[] | {id,name,expires_at}'`, meta.CLIName)
		cmd.Args = cobra.MaximumNArgs(1)
		cmd.Flags().StringVar(&opts.userID, flagUserID, "", "Konnect user ID. Defaults to the authenticated user.")
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return runGetPAT(c, args, opts)
		}
		return cmd, nil
	}
	if verb == verbs.Delete {
		cmd.Short = "Delete a Konnect personal access token"
		cmd.Example = fmt.Sprintf(`  %[1]s delete pat <id|name> --auto-approve`, meta.CLIName)
		cmd.Args = cobra.ExactArgs(1)
		cmd.Flags().StringVar(&opts.userID, flagUserID, "", "Konnect user ID. Defaults to the authenticated user.")
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return runDeletePAT(c, args, opts)
		}
		return cmd, nil
	}

	return cmd, nil
}

func NewSPATCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     SPATCommandName + " [id|name]",
		Short:   "Manage Konnect system account access tokens",
		Aliases: []string{"spats"},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}
	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	opts := &spatOptions{}
	if verb == verbs.Create {
		cmd.Use = SPATCommandName
		cmd.Short = "Create a Konnect system account access token"
		cmd.Example = fmt.Sprintf(
			`  %[1]s create spat --system-account-name ci-bot --name ci --expires-in 30d -o env`,
			meta.CLIName,
		)
		cmd.Args = createTokenArgs
		addSPATCreateFlags(cmd, opts)
		addSystemAccountFlags(cmd, opts)
		cmdcommon.AllowExtraOutputFormats(cmd, cmdcommon.TOKEN.String(), cmdcommon.ENV.String())
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return runCreateSPAT(c, args, opts)
		}
		return cmd, nil
	}
	if verb == verbs.Get {
		cmd.Short = "List or get Konnect system account access tokens"
		cmd.Example = fmt.Sprintf(`  %[1]s get spat --system-account-id <system-account-id>
  %[1]s get spat --system-account-name ci-bot <id|name>`, meta.CLIName)
		cmd.Args = cobra.MaximumNArgs(1)
		addSystemAccountFlags(cmd, opts)
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return runGetSPAT(c, args, opts)
		}
		return cmd, nil
	}
	if verb == verbs.Delete {
		cmd.Short = "Delete a Konnect system account access token"
		cmd.Example = fmt.Sprintf(`  %[1]s delete spat ci --system-account-name ci-bot --auto-approve`, meta.CLIName)
		cmd.Args = cobra.ExactArgs(1)
		addSystemAccountFlags(cmd, opts)
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return runDeleteSPAT(c, args, opts)
		}
		return cmd, nil
	}

	return cmd, nil
}

func addPATCreateFlags(cmd *cobra.Command, opts *patOptions) {
	cmd.Flags().StringVar(&opts.name, flagName, "", "Token name")
	cmd.Flags().StringVar(&opts.userID, flagUserID, "", "Konnect user ID. Defaults to the authenticated user.")
	cmd.Flags().StringVar(&opts.expiresIn, flagExpiresIn, "", expiresInHelp)
	cmd.Flags().StringVar(&opts.expiresAt, flagExpiresAt, "", expiresAtHelp)
}

func addSPATCreateFlags(cmd *cobra.Command, opts *spatOptions) {
	cmd.Flags().StringVar(&opts.name, flagName, "", "Token name")
	cmd.Flags().StringVar(&opts.expiresIn, flagExpiresIn, "", expiresInHelp)
	cmd.Flags().StringVar(&opts.expiresAt, flagExpiresAt, "", expiresAtHelp)
}

func addSystemAccountFlags(cmd *cobra.Command, opts *spatOptions) {
	cmd.Flags().StringVar(&opts.systemAccountID, flagSystemAccountID, "", "Konnect system account ID")
	cmd.Flags().StringVar(&opts.systemAccountName, flagSystemAccountName, "", "Konnect system account name")
}

func createTokenArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if len(args) == 1 {
		if flag := cmd.Flags().Lookup(jq.FlagName); flag != nil && flag.Changed && flag.Value.String() == "-r" {
			return nil
		}
	}
	return cobra.NoArgs(cmd, args)
}

func applyCreateJQExpressionArg(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return nil
	}
	flag := cmd.Flags().Lookup(jq.FlagName)
	if flag == nil || !flag.Changed || flag.Value.String() != "-r" {
		return nil
	}
	if err := cmd.Flags().Set(jq.FlagName, args[0]); err != nil {
		return err
	}
	return cmd.Flags().Set(jq.RawOutputFlagName, "true")
}

func runCreatePAT(c *cobra.Command, args []string, opts *patOptions) error {
	helper := cmdpkg.BuildHelper(c, args)
	if err := applyCreateJQExpressionArg(c, args); err != nil {
		return err
	}
	if err := validateCreatePATFlags(opts); err != nil {
		return err
	}

	exp, err := parseCreateTokenExpiration(opts.expiresIn, opts.expiresAt)
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	userID, err := resolveUserID(opts.userID, sdk.GetMeAPI(), helper)
	if err != nil {
		return err
	}

	api := sdk.GetPersonalAccessTokenAPI()
	if api == nil {
		return cmdpkg.PrepareExecutionErrorMsg(helper, "personal access token API is unavailable")
	}

	request := patCreateRequest(strings.TrimSpace(opts.name), exp)
	res, err := api.CreatePersonalAccessToken(helper.GetContext(), userID, &request)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return cmdpkg.PrepareExecutionError("Failed to create personal access token", err, helper.GetCmd(), attrs...)
	}
	record := createPATRecord(res.GetPersonalAccessTokenCreateResponse())
	return renderCreateRecord(helper, record)
}

func runGetPAT(c *cobra.Command, args []string, opts *patOptions) error {
	helper := cmdpkg.BuildHelper(c, args)
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	userID, err := resolveUserID(opts.userID, sdk.GetMeAPI(), helper)
	if err != nil {
		return err
	}
	api := sdk.GetPersonalAccessTokenAPI()
	if api == nil {
		return cmdpkg.PrepareExecutionErrorMsg(helper, "personal access token API is unavailable")
	}

	if len(args) == 0 {
		tokens, err := listPATs(api, helper, userID)
		if err != nil {
			return err
		}
		records := make([]getTokenRecord, 0, len(tokens))
		for i := range tokens {
			records = append(records, patRecord(&tokens[i]))
		}
		return renderGetRecords(helper, records)
	}

	token, err := resolvePAT(args[0], api, helper, userID)
	if err != nil {
		return err
	}
	return renderGetRecords(helper, patRecord(token))
}

func runDeletePAT(c *cobra.Command, args []string, opts *patOptions) error {
	helper := cmdpkg.BuildHelper(c, args)
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	userID, err := resolveUserID(opts.userID, sdk.GetMeAPI(), helper)
	if err != nil {
		return err
	}
	api := sdk.GetPersonalAccessTokenAPI()
	if api == nil {
		return cmdpkg.PrepareExecutionErrorMsg(helper, "personal access token API is unavailable")
	}

	token, err := resolvePAT(args[0], api, helper, userID)
	if err != nil {
		return err
	}
	record := patDeleteRecord(token)
	if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("personal access token %q", record.Name)); err != nil {
		return err
	}
	if _, err := api.DeletePersonalAccessToken(helper.GetContext(), userID, token.ID); err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return cmdpkg.PrepareExecutionError("Failed to delete personal access token", err, helper.GetCmd(), attrs...)
	}
	record.Status = "deleted"
	return renderDeleteRecord(helper, record)
}

func runCreateSPAT(c *cobra.Command, args []string, opts *spatOptions) error {
	helper := cmdpkg.BuildHelper(c, args)
	if err := applyCreateJQExpressionArg(c, args); err != nil {
		return err
	}
	if err := validateCreateSPATFlags(opts); err != nil {
		return err
	}

	exp, err := parseCreateTokenExpiration(opts.expiresIn, opts.expiresAt)
	if err != nil {
		return err
	}
	expiresAt := expirationToTime(exp)

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	accountID, accountName, err := resolveSystemAccount(opts.systemAccountID, opts.systemAccountName, sdk, helper, cfg)
	if err != nil {
		return err
	}
	api := sdk.GetSystemAccountAccessTokenAPI()
	if api == nil {
		return cmdpkg.PrepareExecutionErrorMsg(helper, "system account access token API is unavailable")
	}

	request := &kkComps.CreateSystemAccountAccessToken{
		Name:      strings.TrimSpace(opts.name),
		ExpiresAt: expiresAt,
	}
	res, err := api.PostSystemAccountsIDAccessTokens(helper.GetContext(), accountID, request)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return cmdpkg.PrepareExecutionError(
			"Failed to create system account access token",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}
	record := createSPATRecord(res.GetSystemAccountAccessTokenCreated(), accountID, accountName)
	return renderCreateRecord(helper, record)
}

func runGetSPAT(c *cobra.Command, args []string, opts *spatOptions) error {
	helper := cmdpkg.BuildHelper(c, args)
	if err := validateSystemAccountSelector(opts.systemAccountID, opts.systemAccountName); err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	accountID, accountName, err := resolveSystemAccount(opts.systemAccountID, opts.systemAccountName, sdk, helper, cfg)
	if err != nil {
		return err
	}
	api := sdk.GetSystemAccountAccessTokenAPI()
	if api == nil {
		return cmdpkg.PrepareExecutionErrorMsg(helper, "system account access token API is unavailable")
	}

	if len(args) == 0 {
		tokens, err := listSPATs(api, helper, cfg, accountID, "")
		if err != nil {
			return err
		}
		records := make([]getTokenRecord, 0, len(tokens))
		for i := range tokens {
			records = append(records, spatRecord(&tokens[i], accountID, accountName))
		}
		return renderGetRecords(helper, records)
	}

	token, err := resolveSPAT(args[0], api, helper, cfg, accountID)
	if err != nil {
		return err
	}
	return renderGetRecords(helper, spatRecord(token, accountID, accountName))
}

func runDeleteSPAT(c *cobra.Command, args []string, opts *spatOptions) error {
	helper := cmdpkg.BuildHelper(c, args)
	if err := validateSystemAccountSelector(opts.systemAccountID, opts.systemAccountName); err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	accountID, _, err := resolveSystemAccount(opts.systemAccountID, opts.systemAccountName, sdk, helper, cfg)
	if err != nil {
		return err
	}
	api := sdk.GetSystemAccountAccessTokenAPI()
	if api == nil {
		return cmdpkg.PrepareExecutionErrorMsg(helper, "system account access token API is unavailable")
	}

	token, err := resolveSPAT(args[0], api, helper, cfg, accountID)
	if err != nil {
		return err
	}
	record := spatDeleteRecord(token)
	deleteLabel := deleteRecordLabel(record)
	if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("system account access token %q", deleteLabel)); err != nil {
		return err
	}
	tokenID := pointerValue(token.ID)
	if _, err := api.DeleteSystemAccountsIDAccessTokensID(helper.GetContext(), accountID, tokenID); err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return cmdpkg.PrepareExecutionError(
			"Failed to delete system account access token",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}
	record.Status = "deleted"
	return renderDeleteRecord(helper, record)
}

func resolveUserID(userID string, meAPI helpers.MeAPI, helper cmdpkg.Helper) (string, error) {
	if value := strings.TrimSpace(userID); value != "" {
		return value, nil
	}
	if meAPI == nil {
		return "", cmdpkg.PrepareExecutionErrorMsg(helper, "current user lookup API is unavailable")
	}

	res, err := meAPI.GetUsersMe(helper.GetContext())
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return "", cmdpkg.PrepareExecutionError("Failed to get current user", err, helper.GetCmd(), attrs...)
	}
	user := res.GetUser()
	if user == nil || user.GetID() == nil || strings.TrimSpace(*user.GetID()) == "" {
		return "", cmdpkg.PrepareExecutionErrorMsg(helper, "current user response did not include an id")
	}
	return strings.TrimSpace(*user.GetID()), nil
}

//nolint:staticcheck
func patCreateRequest(name string, exp expiration) kkComps.PersonalAccessTokenCreateRequest {
	if exp.TTLSeconds != nil {
		return kkComps.CreatePersonalAccessTokenCreateRequestPersonalAccessTokenCreateRequestWithTTL(
			kkComps.PersonalAccessTokenCreateRequestWithTTL{
				Name:       name,
				TTLSeconds: *exp.TTLSeconds,
			},
		)
	}
	return kkComps.CreatePersonalAccessTokenCreateRequestPersonalAccessTokenCreateRequestWithExpiresAt(
		kkComps.PersonalAccessTokenCreateRequestWithExpiresAt{
			Name:      name,
			ExpiresAt: *exp.ExpiresAt,
		},
	)
}

func listPATs(
	api helpers.PersonalAccessTokenAPI,
	helper cmdpkg.Helper,
	userID string,
) ([]kkComps.PersonalAccessToken, error) {
	res, err := api.ListUsersPersonalAccessTokens(helper.GetContext(), userID)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError(
			"Failed to list personal access tokens",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}
	list := res.GetPersonalAccessTokenListResponse()
	if list == nil {
		return nil, nil
	}
	return list.GetData(), nil
}

func resolvePAT(
	identifier string,
	api helpers.PersonalAccessTokenAPI,
	helper cmdpkg.Helper,
	userID string,
) (*kkComps.PersonalAccessToken, error) {
	identifier = strings.TrimSpace(identifier)
	if util.IsValidUUID(identifier) {
		res, err := api.GetPersonalAccessTokenDetails(helper.GetContext(), userID, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError(
				"Failed to get personal access token",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		return res.GetPersonalAccessToken(), nil
	}

	tokens, err := listPATs(api, helper, userID)
	if err != nil {
		return nil, err
	}
	matches := make([]kkComps.PersonalAccessToken, 0, 1)
	for i := range tokens {
		if tokens[i].Name == identifier {
			matches = append(matches, tokens[i])
		}
	}
	if len(matches) == 0 {
		return nil, cmdpkg.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("personal access token %q not found", identifier))
	}
	if len(matches) > 1 {
		return nil, cmdpkg.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("multiple personal access tokens found with name %q; use an id", identifier))
	}
	return &matches[0], nil
}

// validateCreatePATFlags reports every missing required flag at once so the user
// doesn't have to rerun the command to discover them one by one.
func validateCreatePATFlags(opts *patOptions) error {
	var errs []error
	if strings.TrimSpace(opts.name) == "" {
		errs = append(errs, fmt.Errorf("--%s is required", flagName))
	}
	if err := expirationRequiredError(opts.expiresIn, opts.expiresAt); err != nil {
		errs = append(errs, err)
	}
	return bundleConfigErrors(errs)
}

// validateCreateSPATFlags mirrors validateCreatePATFlags and adds the system
// account selector requirement.
func validateCreateSPATFlags(opts *spatOptions) error {
	var errs []error
	if strings.TrimSpace(opts.name) == "" {
		errs = append(errs, fmt.Errorf("--%s is required", flagName))
	}
	if err := systemAccountSelectorError(opts.systemAccountID, opts.systemAccountName); err != nil {
		errs = append(errs, err)
	}
	if err := expirationRequiredError(opts.expiresIn, opts.expiresAt); err != nil {
		errs = append(errs, err)
	}
	return bundleConfigErrors(errs)
}

// expirationRequiredError returns an error when neither expiry flag is set. The
// mutually-exclusive case (both set) is left to expiration parsing.
func expirationRequiredError(expiresIn, expiresAt string) error {
	if strings.TrimSpace(expiresIn) == "" && strings.TrimSpace(expiresAt) == "" {
		return fmt.Errorf("exactly one of --%s or --%s is required", flagExpiresIn, flagExpiresAt)
	}
	return nil
}

func bundleConfigErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return &cmdpkg.ConfigurationError{Err: errors.Join(errs...)}
}

func systemAccountSelectorError(id, name string) error {
	hasID := strings.TrimSpace(id) != ""
	hasName := strings.TrimSpace(name) != ""
	if hasID == hasName {
		return fmt.Errorf("exactly one of --%s or --%s is required", flagSystemAccountID, flagSystemAccountName)
	}
	return nil
}

func validateSystemAccountSelector(id, name string) error {
	if err := systemAccountSelectorError(id, name); err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	return nil
}

func resolveSystemAccount(
	id string,
	name string,
	sdk helpers.SDKAPI,
	helper cmdpkg.Helper,
	cfg config.Hook,
) (string, string, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id != "" {
		return id, name, nil
	}

	api := sdk.GetSystemAccountAPI()
	if api == nil {
		return "", "", cmdpkg.PrepareExecutionErrorMsg(helper, "system account API is unavailable")
	}
	accounts, err := listSystemAccountsByName(name, api, helper, cfg)
	if err != nil {
		return "", "", err
	}
	if len(accounts) == 0 {
		return "", "", cmdpkg.PrepareExecutionErrorMsg(helper, fmt.Sprintf("system account %q not found", name))
	}
	if len(accounts) > 1 {
		return "", "", cmdpkg.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("multiple system accounts found with name %q; use --%s", name, flagSystemAccountID))
	}
	accountID := pointerValue(accounts[0].ID)
	if accountID == "" {
		return "", "", cmdpkg.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("system account %q did not include an id", name))
	}
	return accountID, pointerValue(accounts[0].Name), nil
}

func listSystemAccountsByName(
	name string,
	api helpers.SystemAccountAPI,
	helper cmdpkg.Helper,
	cfg config.Hook,
) ([]kkComps.SystemAccount, error) {
	pageSize := requestPageSize(cfg)
	pageNumber := int64(1)
	var allData []kkComps.SystemAccount

	for {
		req := kkOps.GetSystemAccountsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
			Filter: &kkOps.GetSystemAccountsQueryParamFilter{
				Name: &kkComps.LegacyStringFieldFilter{Eq: &name},
			},
		}
		res, err := api.ListSystemAccounts(helper.GetContext(), req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("Failed to list system accounts", err, helper.GetCmd(), attrs...)
		}
		collection := res.GetSystemAccountCollection()
		if collection == nil {
			return allData, nil
		}
		allData = append(allData, collection.Data...)
		if collection.Meta == nil || len(allData) >= int(collection.Meta.Page.Total) {
			return allData, nil
		}
		pageNumber++
	}
}

func listSPATs(
	api helpers.SystemAccountAccessTokenAPI,
	helper cmdpkg.Helper,
	cfg config.Hook,
	accountID string,
	name string,
) ([]kkComps.SystemAccountAccessToken, error) {
	pageSize := requestPageSize(cfg)
	pageNumber := int64(1)
	var allData []kkComps.SystemAccountAccessToken

	for {
		req := kkOps.GetSystemAccountIDAccessTokensRequest{
			AccountID:  accountID,
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
			Filter:     nil,
		}
		if name != "" {
			req.Filter = &kkOps.GetSystemAccountIDAccessTokensQueryParamFilter{
				Name: &kkComps.LegacyStringFieldFilter{Eq: &name},
			}
		}
		res, err := api.GetSystemAccountIDAccessTokens(helper.GetContext(), req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError(
				"Failed to list system account access tokens", err, helper.GetCmd(), attrs...,
			)
		}
		collection := res.GetSystemAccountAccessTokenCollection()
		if collection == nil {
			return allData, nil
		}
		allData = append(allData, collection.Data...)
		if collection.Meta == nil || len(allData) >= int(collection.Meta.Page.Total) {
			return allData, nil
		}
		pageNumber++
	}
}

func resolveSPAT(
	identifier string,
	api helpers.SystemAccountAccessTokenAPI,
	helper cmdpkg.Helper,
	cfg config.Hook,
	accountID string,
) (*kkComps.SystemAccountAccessToken, error) {
	identifier = strings.TrimSpace(identifier)
	if util.IsValidUUID(identifier) {
		res, err := api.GetSystemAccountsIDAccessTokensID(helper.GetContext(), accountID, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError(
				"Failed to get system account access token",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		return res.GetSystemAccountAccessToken(), nil
	}

	matches, err := listSPATs(api, helper, cfg, accountID, identifier)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, cmdpkg.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("system account access token %q not found", identifier))
	}
	if len(matches) > 1 {
		return nil, cmdpkg.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("multiple system account access tokens found with name %q; use an id", identifier))
	}
	return &matches[0], nil
}

func requestPageSize(cfg config.Hook) int64 {
	pageSize := int64(konnectcommon.DefaultRequestPageSize)
	if cfg != nil {
		value := int64(cfg.GetInt(konnectcommon.RequestPageSizeConfigPath))
		if value > 0 {
			pageSize = value
		}
	}
	return pageSize
}

func parseExpiration(expiresIn, expiresAt string) (expiration, error) {
	return parseExpirationWithExpectation(expiresIn, expiresAt, expiresInExpectation)
}

func parseExpirationWithExpectation(expiresIn, expiresAt string, expectation string) (expiration, error) {
	expiresIn = strings.TrimSpace(expiresIn)
	expiresAt = strings.TrimSpace(expiresAt)
	if (expiresIn == "") == (expiresAt == "") {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("exactly one of --%s or --%s is required", flagExpiresIn, flagExpiresAt),
		}
	}
	if expiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return expiration{}, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("invalid --%s value %q: %w", flagExpiresAt, expiresAt, err),
			}
		}
		return expiration{ExpiresAt: &parsed}, nil
	}

	duration, err := parseDurationWithDays(expiresIn)
	if err != nil {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("invalid --%s value %q: %s", flagExpiresIn, expiresIn, expectation),
		}
	}
	ttl := int64(duration.Seconds())
	if ttl <= 0 {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("--%s must be greater than zero; %s", flagExpiresIn, expectation),
		}
	}
	return expiration{TTLSeconds: &ttl}, nil
}

func parseCreateTokenExpiration(expiresIn, expiresAt string) (expiration, error) {
	return parseCreateTokenExpirationAt(expiresIn, expiresAt, time.Now().UTC())
}

func parseCreateTokenExpirationAt(expiresIn, expiresAt string, now time.Time) (expiration, error) {
	expiresIn = strings.TrimSpace(expiresIn)
	expiresAt = strings.TrimSpace(expiresAt)
	if (expiresIn == "") == (expiresAt == "") {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("exactly one of --%s or --%s is required", flagExpiresIn, flagExpiresAt),
		}
	}

	if expiresAt != "" {
		return parseCreateTokenExpiresAt(expiresAt, now)
	}

	duration, err := parseDurationWithDays(expiresIn)
	if err != nil {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("invalid --%s value %q: %s", flagExpiresIn, expiresIn, createExpiresInExpectation),
		}
	}
	if duration < time.Duration(minTokenTTLSeconds)*time.Second {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("minimum token lifetime is 1 day (--%s must be at least 1d)", flagExpiresIn),
		}
	}
	if duration > time.Duration(maxTokenTTLSeconds)*time.Second {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf(
				"maximum token lifetime is 365 days (12 months) (--%s must be at most 365d)",
				flagExpiresIn,
			),
		}
	}
	ttl := int64(duration / time.Second)
	return expiration{TTLSeconds: &ttl}, nil
}

func parseCreateTokenExpiresAt(expiresAt string, now time.Time) (expiration, error) {
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("invalid --%s value %q: %w", flagExpiresAt, expiresAt, err),
		}
	}
	ttlSeconds := parsed.Unix() - now.Unix()
	if ttlSeconds < minTokenTTLSeconds {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf(
				"minimum token lifetime is 1 day (--%s must be at least 1 day from now)",
				flagExpiresAt,
			),
		}
	}
	if ttlSeconds > maxTokenTTLSeconds {
		return expiration{}, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf(
				"maximum token lifetime is 365 days (12 months) (--%s must be at most 365 days from now)",
				flagExpiresAt,
			),
		}
	}
	return expiration{ExpiresAt: &parsed}, nil
}

func parseDurationWithDays(raw string) (time.Duration, error) {
	if before, ok := strings.CutSuffix(raw, "d"); ok {
		days, err := strconv.ParseFloat(before, 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(raw)
}

func expirationToTime(exp expiration) time.Time {
	if exp.ExpiresAt != nil {
		return *exp.ExpiresAt
	}
	return time.Now().UTC().Add(time.Duration(*exp.TTLSeconds) * time.Second)
}

func createPATRecord(token *kkComps.PersonalAccessTokenCreateResponse) createTokenRecord {
	if token == nil {
		return createTokenRecord{Type: PATCommandName}
	}
	return createTokenRecord{
		Type:       PATCommandName,
		ID:         token.ID,
		Name:       token.Name,
		Token:      token.KonnectToken,
		UserID:     token.UserID,
		State:      string(token.State),
		CreatedAt:  formatTime(token.CreatedAt),
		UpdatedAt:  formatTimePtr(token.UpdatedAt),
		LastUsedAt: formatTimePtr(token.LastUsedAt),
		ExpiresAt:  formatTimePtr(token.ExpiresAt),
	}
}

func createSPATRecord(
	token *kkComps.SystemAccountAccessTokenCreated,
	accountID string,
	accountName string,
) createTokenRecord {
	record := createTokenRecord{
		Type:              SPATCommandName,
		SystemAccountID:   accountID,
		SystemAccountName: accountName,
	}
	if token == nil {
		return record
	}
	record.ID = pointerValue(token.ID)
	record.Name = pointerValue(token.Name)
	record.Token = pointerValue(token.Token)
	record.CreatedAt = formatTimePtr(token.CreatedAt)
	record.UpdatedAt = formatTimePtr(token.UpdatedAt)
	record.LastUsedAt = formatTimePtr(token.LastUsedAt)
	record.ExpiresAt = formatTimePtr(token.ExpiresAt)
	return record
}

func patRecord(token *kkComps.PersonalAccessToken) getTokenRecord {
	if token == nil {
		return getTokenRecord{Type: PATCommandName}
	}
	return getTokenRecord{
		Type:       PATCommandName,
		ID:         token.ID,
		Name:       token.Name,
		UserID:     token.UserID,
		State:      string(token.State),
		CreatedAt:  formatTime(token.CreatedAt),
		UpdatedAt:  formatTime(token.UpdatedAt),
		LastUsedAt: formatTimePtr(token.LastUsedAt),
		ExpiresAt:  formatTimePtr(token.ExpiresAt),
	}
}

func spatRecord(token *kkComps.SystemAccountAccessToken, accountID, accountName string) getTokenRecord {
	record := getTokenRecord{
		Type:              SPATCommandName,
		SystemAccountID:   accountID,
		SystemAccountName: accountName,
	}
	if token == nil {
		return record
	}
	record.ID = pointerValue(token.ID)
	record.Name = pointerValue(token.Name)
	record.CreatedAt = formatTimePtr(token.CreatedAt)
	record.UpdatedAt = formatTimePtr(token.UpdatedAt)
	record.LastUsedAt = formatTimePtr(token.LastUsedAt)
	record.ExpiresAt = formatTimePtr(token.ExpiresAt)
	return record
}

func patDeleteRecord(token *kkComps.PersonalAccessToken) deleteTokenRecord {
	record := deleteTokenRecord{Type: PATCommandName, Status: "pending"}
	if token == nil {
		return record
	}
	record.ID = token.ID
	record.Name = token.Name
	return record
}

func spatDeleteRecord(token *kkComps.SystemAccountAccessToken) deleteTokenRecord {
	record := deleteTokenRecord{Type: SPATCommandName, Status: "pending"}
	if token == nil {
		return record
	}
	record.ID = pointerValue(token.ID)
	record.Name = pointerValue(token.Name)
	return record
}

func renderCreateRecord(helper cmdpkg.Helper, record createTokenRecord) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	settings, err := jq.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return err
	}
	if jq.HasFilter(settings) {
		if outType == cmdcommon.TOKEN || outType == cmdcommon.ENV {
			return &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("--%s is not supported with --output %s", jq.FlagName, outType.String()),
			}
		}
		if !outputFlagChanged(helper.GetCmd()) {
			outType = cmdcommon.JSON
		}
		filtered, handled, err := jq.ApplyToRaw(record, outType, settings, helper.GetStreams().Out)
		if err != nil {
			return cmdpkg.PrepareExecutionErrorWithHelper(helper, "jq filter failed", err)
		}
		if handled {
			return nil
		}
		return printStructured(helper, outType, filtered)
	}

	switch outType {
	case cmdcommon.TOKEN:
		_, err = fmt.Fprintln(helper.GetStreams().Out, record.Token)
		return err
	case cmdcommon.ENV:
		envName := utilviper.ProfileEnvPrefix(cfg.GetProfile()) + "_KONNECT_PAT"
		_, err = fmt.Fprintf(helper.GetStreams().Out, "export %s=%s\n", envName, shellQuote(record.Token))
		return err
	case cmdcommon.TEXT:
		_, err = fmt.Fprintf(helper.GetStreams().Out, "Created %s %q (%s)\ntoken: %s\n",
			record.Type, record.Name, record.ID, record.Token)
		return err
	case cmdcommon.JSON, cmdcommon.YAML:
		return printStructured(helper, outType, record)
	case cmdcommon.HELM:
		return fmt.Errorf("unsupported output format %s", outType.String())
	default:
		return fmt.Errorf("unsupported output format %s", outType.String())
	}
}

func renderGetRecords(helper cmdpkg.Helper, data any) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	settings, err := jq.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return err
	}
	if jq.HasFilter(settings) && !outputFlagChanged(helper.GetCmd()) {
		outType = cmdcommon.JSON
	}
	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		data,
		data,
		"",
	)
}

func renderDeleteRecord(helper cmdpkg.Helper, record deleteTokenRecord) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	if outType == cmdcommon.TEXT {
		_, err = fmt.Fprintf(helper.GetStreams().Out, "Deleted %s %q\n", record.Type, deleteRecordLabel(record))
		return err
	}
	return printStructured(helper, outType, record)
}

func deleteRecordLabel(record deleteTokenRecord) string {
	if record.Name != "" {
		return record.Name
	}
	return record.ID
}

func printStructured(helper cmdpkg.Helper, outType cmdcommon.OutputFormat, data any) error {
	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(data)
	return nil
}

func outputFlagChanged(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if f := c.Flags().Lookup(cmdcommon.OutputFlagName); f != nil && f.Changed {
			return true
		}
		if f := c.PersistentFlags().Lookup(cmdcommon.OutputFlagName); f != nil && f.Changed {
			return true
		}
	}
	return false
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
