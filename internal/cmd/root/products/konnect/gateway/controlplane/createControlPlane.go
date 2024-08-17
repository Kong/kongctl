package controlplane

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kong-cli/internal/cmd"
	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/common"
	"github.com/kong/kong-cli/internal/konnect/auth"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	CreateCpDescriptionFlagName = "description"
	CreateCpClusterTypeFlagName = "cluster-type"
	CreateCpAuthTypeFlagName    = "auth-type"
	CreateCpIsCloudGwFlagName   = "is-cloud-gateway"
	CreateCpProxyUrlsFlagName   = "proxy-urls"
	CreateCpLabelsFlagName      = "labels"
)

var (
	createControlPlanesUse   = fmt.Sprintf("%s [%s]", CommandName, "name")
	createControlPlanesShort = i18n.T("root.products.konnect.gateway.controlplane.createControlPlanesShort",
		"Create a new Konnect Kong Gateway control plane")
	createControlPlanesLong = i18n.T("root.products.konnect.gateway.controlplane.createControlPlanesLong",
		`Use the create verb with the control-plane command to create a new Konnect Kong Gateway control plane resource.
See flags for creation options.`)
	createControlPlanesExample = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.gateway.controlplane.createControlPlaneExamples",
			fmt.Sprintf(`
# Create a new control plane with default options and the name 'my-control-plane' 
%[1]s create konnect gateway control-plane my-control-plane
# Create a new control plane with the name 'my-control-plane' and specifying all the available options
%[1]s create konnect gateway control-plane my-control-plane --description "full description" --cluster-type hybrid 
	`, meta.CLIName)))

	createCpDescriptionConfigPath = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, CreateCpDescriptionFlagName)
	createCpClusterTypeConfigPath = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, CreateCpClusterTypeFlagName)
	createCpAuthTypeConfigPath    = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, CreateCpAuthTypeFlagName)
	createCpIsCloudGwConfigPath   = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, CreateCpIsCloudGwFlagName)
	createCpProxyUrlsConfigPath   = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, CreateCpProxyUrlsFlagName)
	createCpLabelsConfigPath      = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, CreateCpLabelsFlagName)
)

type createControlPlaneCmd struct {
	*cobra.Command
	clusterType *cmd.FlagEnum
	authType    *cmd.FlagEnum
	proxyUrls   []string
}

func (c *createControlPlaneCmd) validate(_ cmd.Helper) error {
	//argsRequired := 1
	//if len(helper.GetArgs()) != argsRequired {
	//	return &cmd.ConfigurationError{
	//		Err: fmt.Errorf("creating a control plane requires %d argument (name)", argsRequired),
	//	}
	//}
	return nil
}

// Manually added mapping between "CLI Friendly" names and the SDK based values which
//	are less friendly.  This may be better served with some type of automation or generation
//	out of the SDK / specification.

func (c *createControlPlaneCmd) convertClusterType(cfgClusterType string) (kkComps.ClusterType, error) {
	switch cfgClusterType {
	case "hybrid":
		return kkComps.ClusterTypeClusterTypeHybrid, nil
	case "kic":
		return kkComps.ClusterTypeClusterTypeK8SIngressController, nil
	case "group":
		return kkComps.ClusterTypeClusterTypeControlPlaneGroup, nil
	case "serverless":
		return kkComps.ClusterTypeClusterTypeServerless, nil
	default:
		return "", fmt.Errorf("invalid value for ClusterType: %v", cfgClusterType)
	}
}

func convertAuthType(cfgAuthType string) (kkComps.AuthType, error) {
	switch cfgAuthType {
	case "pinned":
		return kkComps.AuthTypePinnedClientCerts, nil
	case "pki":
		return kkComps.AuthTypePkiClientCerts, nil
	default:
		return "", fmt.Errorf("invalid value for AuthType: %v", cfgAuthType)
	}
}

func convertLabels(labels []string) (map[string]string, error) {
	rv := make(map[string]string)
	for _, label := range labels {
		parts := strings.Split(label, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s", label)
		}
		rv[parts[0]] = parts[1]
	}
	return rv, nil
}

func convertProxyUrls(urls []string) ([]kkComps.ProxyURL, error) {
	var rv []kkComps.ProxyURL
	for _, urlStr := range urls {
		u, e := url.Parse(urlStr)
		if e != nil {
			return nil, e
		}
		p, e := strconv.ParseInt(u.Port(), 10, 64)
		if e != nil {
			return nil, e
		}

		rv = append(rv, kkComps.ProxyURL{
			Protocol: u.Scheme,
			Host:     u.Hostname(),
			Port:     p,
		})
	}
	return rv, nil
}

func (c *createControlPlaneCmd) run(helper cmd.Helper) error {
	name := helper.GetArgs()[0]

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	ct, e := c.convertClusterType(cfg.GetString(createCpClusterTypeConfigPath))
	if e != nil {
		return e
	}

	at, e := convertAuthType(cfg.GetString(createCpAuthTypeConfigPath))
	if e != nil {
		return e
	}

	isCGW := cfg.GetBool(createCpIsCloudGwConfigPath)

	proxyUrls, e := convertProxyUrls(cfg.GetStringSlice(createCpProxyUrlsConfigPath))
	if e != nil {
		return e
	}

	labels, e := convertLabels(cfg.GetStringSlice(createCpLabelsConfigPath))
	if e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType, helper.GetStreams().Out)
	if e != nil {
		return e
	}
	defer printer.Flush()

	req := kkComps.CreateControlPlaneRequest{
		Name:         name,
		Description:  kk.String(cfg.GetString(createCpDescriptionConfigPath)),
		ClusterType:  ct.ToPointer(),
		AuthType:     at.ToPointer(),
		CloudGateway: kk.Bool(isCGW),
		ProxyUrls:    proxyUrls,
		Labels:       labels,
	}

	token, e := common.GetAccessToken(cfg, logger)
	if e != nil {
		return e
	}

	kkClient, e := auth.GetAuthenticatedClient(token)
	if e != nil {
		return e
	}

	ctx := context.Background()

	res, e := kkClient.ControlPlanes.CreateControlPlane(ctx, req)
	if e != nil {
		attrs := cmd.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to create Control Plane", e, helper.GetCmd(), attrs...)
	}

	printer.Print(res.GetControlPlane())
	return nil
}

func (c *createControlPlaneCmd) bindFlags(args []string) error {
	helper := cmd.BuildHelper(c.Command, args)
	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	f := c.Flags().Lookup(CreateCpDescriptionFlagName)
	e = cfg.BindFlag(createCpDescriptionConfigPath, f)
	if e != nil {
		return e
	}

	f = c.Flags().Lookup(CreateCpClusterTypeFlagName)
	e = cfg.BindFlag(createCpClusterTypeConfigPath, f)
	if e != nil {
		return e
	}

	f = c.Flags().Lookup(CreateCpAuthTypeFlagName)
	e = cfg.BindFlag(createCpAuthTypeConfigPath, f)
	if e != nil {
		return e
	}

	f = c.Flags().Lookup(CreateCpIsCloudGwFlagName)
	e = cfg.BindFlag(createCpIsCloudGwConfigPath, f)
	if e != nil {
		return e
	}

	f = c.Flags().Lookup(CreateCpProxyUrlsFlagName)
	e = cfg.BindFlag(createCpProxyUrlsConfigPath, f)
	if e != nil {
		return e
	}

	f = c.Flags().Lookup(CreateCpLabelsFlagName)
	e = cfg.BindFlag(createCpLabelsConfigPath, f)
	if e != nil {
		return e
	}

	return nil
}

func (c *createControlPlaneCmd) preRunE(_ *cobra.Command, args []string) error {
	return c.bindFlags(args)
}

func (c *createControlPlaneCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	return c.run(helper)
}

func newCreateControlPlaneCmd(baseCmd *cobra.Command) *createControlPlaneCmd {
	rv := createControlPlaneCmd{
		Command:     baseCmd,
		clusterType: cmd.NewEnum([]string{"hybrid", "kic", "group", "serverless"}, "hybrid"),
		authType:    cmd.NewEnum([]string{"pinned", "pki"}, "pinned"),
	}

	baseCmd.Flags().String(CreateCpDescriptionFlagName, "",
		fmt.Sprintf(`Extended description for the new control plane.
- Config path: [ %s ]`,
			createCpDescriptionConfigPath))

	baseCmd.Flags().Var(rv.clusterType, CreateCpClusterTypeFlagName,
		fmt.Sprintf(`Specifies the Kong Gateway cluster type attached to the new control plane.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			createCpDescriptionConfigPath, strings.Join(rv.clusterType.Allowed, "|")))

	baseCmd.Flags().Var(rv.authType, CreateCpAuthTypeFlagName,
		fmt.Sprintf(`Specifies the authentication type used to secure the control plane and data plane communication.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			createCpAuthTypeConfigPath, strings.Join(rv.authType.Allowed, "|")))

	baseCmd.Flags().Bool(CreateCpIsCloudGwFlagName, false,
		fmt.Sprintf(`Specifies whether the control plane attaches to cloud gateways.
- Config path: [ %s ]`, createCpIsCloudGwConfigPath))

	baseCmd.Flags().StringSlice(CreateCpProxyUrlsFlagName, rv.proxyUrls,
		fmt.Sprintf(`Specifies URLs for which the data planes connected to this control plane can be reached.
Provide multiple URLs as a comma-separated list. URLs must be in the format: <protocol>://<host>:<port>
- Config path: [ %s ]`, createCpProxyUrlsConfigPath))

	baseCmd.Flags().StringSlice(CreateCpLabelsFlagName, nil, fmt.Sprintf(`Assign metadata labels to the new control plane.
Labels are specified as [ key=value ] pairs and can be provided in a list.
- Config path: [ %s ]`, createCpLabelsConfigPath))

	baseCmd.Use = createControlPlanesUse
	baseCmd.Args = cobra.ExactArgs(1) // new cp name
	baseCmd.Short = createControlPlanesShort
	baseCmd.Long = createControlPlanesLong
	baseCmd.Example = createControlPlanesExample
	baseCmd.PreRunE = rv.preRunE
	baseCmd.RunE = rv.runE

	return &rv
}
