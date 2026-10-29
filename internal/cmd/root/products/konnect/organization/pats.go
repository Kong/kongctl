package organization

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newGetOrganizationPATsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use: "pats", Aliases: []string{"pat"},
		Short: "List personal access tokens in the current organization",
		Long: "List all personal access tokens in the current organization. Requires organization administrator " +
			"access and authentication that supports /v3/organizations/me.",
		Example: fmt.Sprintf("  kongctl %s org pats\n  kongctl %s org pats -o json", verb, verb),
		Args:    cobra.NoArgs, PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			return runGetOrganizationPATs(cmd.BuildHelper(c, args))
		},
	}
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

func runGetOrganizationPATs(helper cmd.Helper) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	baseURL, err := common.ResolveAPIBaseURL(helper.GetCmd(), cfg, "/v3/organizations")
	if err != nil {
		return err
	}
	tokenSource, err := common.GetAccessTokenSource(cfg, logger)
	if err != nil {
		return err
	}
	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize <= 0 {
		pageSize = common.DefaultRequestPageSize
	}
	tokens, err := fetchOrganizationPATs(helper.GetContext(), httpclient.NewLoggingHTTPClient(logger),
		baseURL, tokenSource, pageSize)
	if err != nil {
		return cmd.PrepareExecutionError("Failed to list organization personal access tokens", err, helper.GetCmd())
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
	type row struct {
		ID     string
		Name   string
		UserID string
		State  string
	}
	rows := make([]row, 0, len(tokens))
	tableRows := make([]table.Row, 0, len(tokens))
	for _, token := range tokens {
		rows = append(rows, row{token.ID, token.Name, token.UserID, string(token.State)})
		tableRows = append(tableRows, table.Row{token.ID, token.Name, token.UserID, string(token.State)})
	}
	return tableview.RenderForFormat(helper, false, outType, printer, helper.GetStreams(),
		rows, tokens, "Organization Personal Access Tokens",
		tableview.WithExactCustomTable([]string{"ID", "NAME", "USER ID", "STATE"}, tableRows))
}

func fetchOrganizationPATs(
	ctx context.Context, client apiutil.Doer, baseURL string, tokenSource apiutil.TokenSource,
	pageSize int,
) ([]kkComps.PersonalAccessToken, error) {
	get := func(endpoint string, payload any) error {
		res, err := apiutil.RequestWithTokenSource(ctx, client, http.MethodGet, baseURL,
			endpoint, tokenSource, nil, nil)
		if err != nil {
			return err
		}
		if res.StatusCode >= 400 {
			return fmt.Errorf("GET %s failed with status %d: %s", endpoint, res.StatusCode, strings.TrimSpace(string(res.Body)))
		}
		if err := json.Unmarshal(res.Body, payload); err != nil {
			return fmt.Errorf("decode %s: %w", endpoint, err)
		}
		return nil
	}
	var org struct {
		ID string `json:"id"`
	}
	if err := get("/v3/organizations/me", &org); err != nil {
		return nil, fmt.Errorf("current organization lookup requires /v3/organizations/me support: %w", err)
	}
	if org.ID == "" {
		return nil, fmt.Errorf("current organization response has no ID")
	}
	path := "/v3/organizations/" + url.PathEscape(org.ID) + "/personal-access-tokens"
	query := url.Values{"page[size]": {strconv.Itoa(pageSize)}}
	tokens := make([]kkComps.PersonalAccessToken, 0)
	var cursors pagination.CursorTracker
	for {
		var page struct {
			Data []kkComps.PersonalAccessToken `json:"data"`
			Meta struct {
				Next *string `json:"next"`
				Page struct {
					Next *string `json:"next"`
				} `json:"page"`
			} `json:"meta"`
		}
		if err := get(path+"?"+query.Encode(), &page); err != nil {
			return nil, err
		}
		tokens = append(tokens, page.Data...)
		nextLink := page.Meta.Next
		if nextLink == nil {
			nextLink = page.Meta.Page.Next
		}
		after, err := cursors.Next(nextLink)
		if err != nil {
			return nil, fmt.Errorf("PAT pagination: %w", err)
		}
		if after == "" {
			return tokens, nil
		}
		query.Set("page[after]", after)
	}
}
