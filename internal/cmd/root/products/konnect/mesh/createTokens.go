package mesh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

// Token endpoints on the control plane. A user token endpoint exists in Kuma
// but is not registered on a Konnect hosted control plane, which authenticates
// through Konnect instead, so it is not offered here. See Phase 3.
const (
	dataplaneTokenPath = "/tokens/dataplane"
	zoneTokenPath      = "/tokens/zone"
)

// controlPlaneZoneScope is the zone token scope Kong Mesh registers.
//
// It is sent by default because omitting the scope makes the control plane
// answer 500 rather than falling back to the distribution's full scope. kumactl
// defaults the same way, which is why it never meets that failure.
const controlPlaneZoneScope = "cp"

// Flag names for the token commands.
const (
	tokenNameFlagName      = "name"
	tokenValidForFlagName  = "valid-for"
	tokenTagFlagName       = "tag"
	tokenProxyTypeFlagName = "proxy-type"
	tokenWorkloadFlagName  = "workload"
	tokenZoneFlagName      = "zone"
	tokenScopeFlagName     = "scope"
)

// dataplaneTokenRequest is the payload the control plane expects. Fields are
// omitted when empty so the control plane applies its own defaults.
type dataplaneTokenRequest struct {
	Name     string              `json:"name,omitempty"`
	Mesh     string              `json:"mesh"`
	Tags     map[string][]string `json:"tags,omitempty"`
	Type     string              `json:"type,omitempty"`
	Workload string              `json:"workload,omitempty"`
	ValidFor string              `json:"validFor"`
}

// zoneTokenRequest is the payload for a zone token.
type zoneTokenRequest struct {
	Zone     string   `json:"zone"`
	Scope    []string `json:"scope,omitempty"`
	ValidFor string   `json:"validFor"`
}

var (
	dataplaneTokenShort = i18n.T("root.products.konnect.mesh.dataplaneTokenShort",
		"Issue a token that proves a dataplane's identity")

	dataplaneTokenLong = normalizers.LongDesc(i18n.T("root.products.konnect.mesh.dataplaneTokenLong",
		`Issue a dataplane token from the control plane.

A dataplane token lets kuma-dp prove its identity when it connects. Bind the
token as narrowly as the deployment allows: to a name, to a workload, or to
tags, rather than to the mesh alone.

The token is written to stdout with no trailing newline, so it can be
redirected straight into the file kuma-dp reads.`))

	dataplaneTokenExample = normalizers.Examples(i18n.T("root.products.konnect.mesh.dataplaneTokenExample",
		fmt.Sprintf(`
	# A token bound to one dataplane, written to a file
	%[1]s create mesh dataplane-token --name dp-01 --valid-for 24h > /tmp/token

	# A token bound to a mesh only
	%[1]s create mesh dataplane-token -m prod --valid-for 24h

	# A token bound to tags
	%[1]s create mesh dataplane-token --tag kuma.io/service=web --valid-for 24h
	`, meta.CLIName)))

	zoneTokenShort = i18n.T("root.products.konnect.mesh.zoneTokenShort",
		"Issue a token that proves a zone's identity")

	zoneTokenLong = normalizers.LongDesc(i18n.T("root.products.konnect.mesh.zoneTokenLong",
		`Issue a zone token from the control plane.

A zone token lets a zone control plane prove its identity to a global control
plane when it joins.

The token is written to stdout with no trailing newline, so it can be
redirected straight into a file.`))

	zoneTokenExample = normalizers.Examples(i18n.T("root.products.konnect.mesh.zoneTokenExample",
		fmt.Sprintf(`
	# A token for a zone, written to a file
	%[1]s create mesh zone-token --zone zone-1 --valid-for 24h > /tmp/zone-token
	`, meta.CLIName)))
)

// newDataplaneTokenCmd builds `create mesh dataplane-token`.
func newDataplaneTokenCmd(parentPreRun func(*cobra.Command, []string) error) *cobra.Command {
	cmdObj := &cobra.Command{
		Use:     "dataplane-token",
		Aliases: []string{"dp-token"},
		Short:   dataplaneTokenShort,
		Long:    dataplaneTokenLong,
		Example: dataplaneTokenExample,
		Args:    cobra.NoArgs,
	}
	if parentPreRun != nil {
		cmdObj.PreRunE = parentPreRun
	}

	cmdObj.Flags().String(tokenNameFlagName, "", "Name of the dataplane the token identifies.")
	cmdObj.Flags().StringToString(tokenTagFlagName, nil,
		"Tag values the dataplane must carry. Repeatable; separate multiple values for one tag with commas.")
	cmdObj.Flags().String(tokenProxyTypeFlagName, "", `Proxy type the token is for (for example "dataplane").`)
	cmdObj.Flags().String(tokenWorkloadFlagName, "", "Workload label value the dataplane must carry.")
	cmdObj.Flags().Duration(tokenValidForFlagName, 0, `How long the token remains valid, for example "24h".`)
	_ = cmdObj.MarkFlagRequired(tokenValidForFlagName)

	cmdObj.RunE = func(c *cobra.Command, args []string) error {
		helper := cmd.BuildHelper(c, args)
		return runDataplaneToken(helper, c)
	}
	return cmdObj
}

// newZoneTokenCmd builds `create mesh zone-token`.
func newZoneTokenCmd(parentPreRun func(*cobra.Command, []string) error) *cobra.Command {
	cmdObj := &cobra.Command{
		Use:     "zone-token",
		Short:   zoneTokenShort,
		Long:    zoneTokenLong,
		Example: zoneTokenExample,
		Args:    cobra.NoArgs,
	}
	if parentPreRun != nil {
		cmdObj.PreRunE = parentPreRun
	}

	cmdObj.Flags().String(tokenZoneFlagName, "", "Name of the zone the token identifies.")
	cmdObj.Flags().StringSlice(tokenScopeFlagName, []string{controlPlaneZoneScope},
		"Scope of resources the token can identify.")
	cmdObj.Flags().Duration(tokenValidForFlagName, 0, `How long the token remains valid, for example "24h".`)
	_ = cmdObj.MarkFlagRequired(tokenZoneFlagName)
	_ = cmdObj.MarkFlagRequired(tokenValidForFlagName)

	cmdObj.RunE = func(c *cobra.Command, args []string) error {
		helper := cmd.BuildHelper(c, args)
		return runZoneToken(helper, c)
	}
	return cmdObj
}

func runDataplaneToken(helper cmd.Helper, cmdObj *cobra.Command) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	validFor, err := requireValidFor(cmdObj)
	if err != nil {
		return err
	}

	name, err := cmdObj.Flags().GetString(tokenNameFlagName)
	if err != nil {
		return err
	}
	proxyType, err := cmdObj.Flags().GetString(tokenProxyTypeFlagName)
	if err != nil {
		return err
	}
	workload, err := cmdObj.Flags().GetString(tokenWorkloadFlagName)
	if err != nil {
		return err
	}
	rawTags, err := cmdObj.Flags().GetStringToString(tokenTagFlagName)
	if err != nil {
		return err
	}

	request := dataplaneTokenRequest{
		Name:     name,
		Mesh:     meshcommon.ResolveMesh(cfg),
		Tags:     splitTagValues(rawTags),
		Type:     proxyType,
		Workload: workload,
		ValidFor: validFor,
	}

	return issueToken(helper, dataplaneTokenPath, request, "dataplane token")
}

func runZoneToken(helper cmd.Helper, cmdObj *cobra.Command) error {
	validFor, err := requireValidFor(cmdObj)
	if err != nil {
		return err
	}

	zone, err := cmdObj.Flags().GetString(tokenZoneFlagName)
	if err != nil {
		return err
	}
	scope, err := cmdObj.Flags().GetStringSlice(tokenScopeFlagName)
	if err != nil {
		return err
	}

	request := zoneTokenRequest{
		Zone:     zone,
		Scope:    scope,
		ValidFor: validFor,
	}

	return issueToken(helper, zoneTokenPath, request, "zone token")
}

// requireValidFor reads --valid-for and renders it the way the control plane
// parses it. A token with no expiry is refused rather than sent, because the
// control plane would accept it.
func requireValidFor(cmdObj *cobra.Command) (string, error) {
	validFor, err := cmdObj.Flags().GetDuration(tokenValidForFlagName)
	if err != nil {
		return "", err
	}
	if validFor <= 0 {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("--%s must be a positive duration, for example 24h", tokenValidForFlagName),
		}
	}
	return validFor.String(), nil
}

// splitTagValues turns --tag key=a,b into the multi value form the control
// plane expects.
func splitTagValues(raw map[string]string) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	tags := make(map[string][]string, len(raw))
	for key, value := range raw {
		tags[key] = strings.Split(value, ",")
	}
	return tags
}

// issueToken posts a token request and writes the token to stdout.
//
// The response body is the token itself rather than JSON, and it is written
// without a trailing newline so that redirecting it produces a file holding
// exactly the credential.
func issueToken(helper cmd.Helper, path string, request any, description string) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode the %s request: %w", description, err)
	}

	response, _, err := send(helper, http.MethodPost, path, body)
	if err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to issue a %s", description), err, helper.GetCmd())
	}

	token := strings.TrimSpace(string(response))
	if token == "" {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("the control plane returned an empty %s", description),
			fmt.Errorf("empty response body"), helper.GetCmd())
	}

	_, err = fmt.Fprint(helper.GetStreams().Out, token)
	return err
}
