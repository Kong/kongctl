package eventgateway

import (
	"errors"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	gatewayIDFlagName   = "gateway-id"
	gatewayNameFlagName = "gateway-name"

	gatewayIDConfigPath   = "konnect.event-gateway.id"
	gatewayNameConfigPath = "konnect.event-gateway.name"

	backendClusterIDFlagName   = "backend-cluster-id"
	backendClusterNameFlagName = "backend-cluster-name"

	backendClusterIDConfigPath   = "konnect.event-gateway.backend-cluster.id"
	backendClusterNameConfigPath = "konnect.event-gateway.backend-cluster.name"

	virtualClusterIDFlagName   = "virtual-cluster-id"
	virtualClusterNameFlagName = "virtual-cluster-name"

	virtualClusterIDConfigPath   = "konnect.event-gateway.virtual-cluster.id"
	virtualClusterNameConfigPath = "konnect.event-gateway.virtual-cluster.name"

	listenerIDFlagName   = "listener-id"
	listenerNameFlagName = "listener-name"

	listenerIDConfigPath   = "konnect.event-gateway.listener.id"
	listenerNameConfigPath = "konnect.event-gateway.listener.name"

	dataPlaneCertIDFlagName   = "data-plane-certificate-id"
	dataPlaneCertNameFlagName = "data-plane-certificate-name"

	dataPlaneCertIDConfigPath   = "konnect.event-gateway.data-plane-certificate.id"
	dataPlaneCertNameConfigPath = "konnect.event-gateway.data-plane-certificate.name"

	schemaRegistryIDFlagName   = "schema-registry-id"
	schemaRegistryNameFlagName = "schema-registry-name"

	schemaRegistryIDConfigPath   = "konnect.event-gateway.schema-registry.id"
	schemaRegistryNameConfigPath = "konnect.event-gateway.schema-registry.name"

	staticKeyIDFlagName   = "static-key-id"
	staticKeyNameFlagName = "static-key-name"

	staticKeyIDConfigPath   = "konnect.event-gateway.static-key.id"
	staticKeyNameConfigPath = "konnect.event-gateway.static-key.name"

	tlsTrustBundleIDFlagName   = "tls-trust-bundle-id"
	tlsTrustBundleNameFlagName = "tls-trust-bundle-name"

	tlsTrustBundleIDConfigPath   = "konnect.event-gateway.tls-trust-bundle.id"
	tlsTrustBundleNameConfigPath = "konnect.event-gateway.tls-trust-bundle.name"

	valueNA = "n/a"
)

// formatEnabledBool converts an optional bool pointer to a display string.
func formatEnabledBool(enabled *bool) string {
	if enabled == nil {
		return valueNA
	}
	if *enabled {
		return "true"
	}
	return "false"
}

func addEventGatewayChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		gatewayIDFlagName, gatewayIDConfigPath,
		"The ID of the event gateway that owns the resource.",
		gatewayNameFlagName, gatewayNameConfigPath,
		"The name of the event gateway that owns the resource.",
	)
}

func bindEventGatewayChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{gatewayIDFlagName, gatewayIDConfigPath},
		{gatewayNameFlagName, gatewayNameConfigPath},
	})
}

func getEventGatewayIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(gatewayIDConfigPath), cfg.GetString(gatewayNameConfigPath)
}

func addBackendClusterChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		backendClusterIDFlagName, backendClusterIDConfigPath,
		"The ID of the backend cluster to retrieve.",
		backendClusterNameFlagName, backendClusterNameConfigPath,
		"The name of the backend cluster to retrieve.",
	)
}

func bindBackendClusterChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{backendClusterIDFlagName, backendClusterIDConfigPath},
		{backendClusterNameFlagName, backendClusterNameConfigPath},
	})
}

func getBackendClusterIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(backendClusterIDConfigPath), cfg.GetString(backendClusterNameConfigPath)
}

func addVirtualClusterChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		virtualClusterIDFlagName, virtualClusterIDConfigPath,
		"The ID of the virtual cluster to retrieve.",
		virtualClusterNameFlagName, virtualClusterNameConfigPath,
		"The name of the virtual cluster to retrieve.",
	)
}

func bindVirtualClusterChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{virtualClusterIDFlagName, virtualClusterIDConfigPath},
		{virtualClusterNameFlagName, virtualClusterNameConfigPath},
	})
}

func getVirtualClusterIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(virtualClusterIDConfigPath), cfg.GetString(virtualClusterNameConfigPath)
}

func addListenerChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		listenerIDFlagName, listenerIDConfigPath,
		"The ID of the listener to retrieve.",
		listenerNameFlagName, listenerNameConfigPath,
		"The name of the listener to retrieve.",
	)
}

func bindListenerChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{listenerIDFlagName, listenerIDConfigPath},
		{listenerNameFlagName, listenerNameConfigPath},
	})
}

func getListenerIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(listenerIDConfigPath), cfg.GetString(listenerNameConfigPath)
}

func addDataPlaneCertChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		dataPlaneCertIDFlagName, dataPlaneCertIDConfigPath,
		"The ID of the data plane certificate to retrieve.",
		dataPlaneCertNameFlagName, dataPlaneCertNameConfigPath,
		"The name of the data plane certificate to retrieve.",
	)
}

func bindDataPlaneCertChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{dataPlaneCertIDFlagName, dataPlaneCertIDConfigPath},
		{dataPlaneCertNameFlagName, dataPlaneCertNameConfigPath},
	})
}

func getDataPlaneCertIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(dataPlaneCertIDConfigPath), cfg.GetString(dataPlaneCertNameConfigPath)
}

func addSchemaRegistryChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		schemaRegistryIDFlagName, schemaRegistryIDConfigPath,
		"The ID of the schema registry to retrieve.",
		schemaRegistryNameFlagName, schemaRegistryNameConfigPath,
		"The name of the schema registry to retrieve.",
	)
}

func bindSchemaRegistryChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{schemaRegistryIDFlagName, schemaRegistryIDConfigPath},
		{schemaRegistryNameFlagName, schemaRegistryNameConfigPath},
	})
}

func getSchemaRegistryIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(schemaRegistryIDConfigPath), cfg.GetString(schemaRegistryNameConfigPath)
}

func addStaticKeyChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		staticKeyIDFlagName, staticKeyIDConfigPath,
		"The ID of the static key to retrieve.",
		staticKeyNameFlagName, staticKeyNameConfigPath,
		"The name of the static key to retrieve.",
	)
}

func bindStaticKeyChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{staticKeyIDFlagName, staticKeyIDConfigPath},
		{staticKeyNameFlagName, staticKeyNameConfigPath},
	})
}

func getStaticKeyIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(staticKeyIDConfigPath), cfg.GetString(staticKeyNameConfigPath)
}

func addTLSTrustBundleChildFlags(cmd *cobra.Command) {
	addChildFlags(
		cmd,
		tlsTrustBundleIDFlagName, tlsTrustBundleIDConfigPath,
		"The ID of the TLS trust bundle to retrieve.",
		tlsTrustBundleNameFlagName, tlsTrustBundleNameConfigPath,
		"The name of the TLS trust bundle to retrieve.",
	)
}

func bindTLSTrustBundleChildFlags(c *cobra.Command, args []string) error {
	return bindChildFlags(c, args, []flagBinding{
		{tlsTrustBundleIDFlagName, tlsTrustBundleIDConfigPath},
		{tlsTrustBundleNameFlagName, tlsTrustBundleNameConfigPath},
	})
}

func addChildFlags(
	cmd *cobra.Command,
	idFlagName, idConfigPath, idDesc string,
	nameFlagName, nameConfigPath, nameDesc string,
) {
	cmd.Flags().String(idFlagName, "",
		fmt.Sprintf("%s\n- Config path: [ %s ]", idDesc, idConfigPath))
	cmd.Flags().String(nameFlagName, "",
		fmt.Sprintf("%s\n- Config path: [ %s ]", nameDesc, nameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(idFlagName, nameFlagName)
}

func bindChildFlags(c *cobra.Command, args []string, bindings []flagBinding) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	for _, b := range bindings {
		if err := bindFlag(cfg, c.Flags(), b.flag, b.config); err != nil {
			return err
		}
	}

	return nil
}

func getTLSTrustBundleIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(tlsTrustBundleIDConfigPath), cfg.GetString(tlsTrustBundleNameConfigPath)
}

func resolveEventGatewayIDByName(
	name string,
	gatewayClient helpers.EGWControlPlaneAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	gateway, err := runListByName(name, gatewayClient, helper, cfg)
	if err != nil {
		var execErr *cmd.ExecutionError
		if errors.As(err, &execErr) {
			return "", err
		}
		return "", &cmd.ConfigurationError{Err: err}
	}

	if gateway == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("event gateway %q not found", name),
		}
	}

	return gateway.ID, nil
}

func bindFlag(cfg config.Hook, flags *pflag.FlagSet, flagName, configPath string) error {
	if f := flags.Lookup(flagName); f != nil {
		return cfg.BindFlag(configPath, f)
	}
	return nil
}

type flagBinding struct {
	flag   string
	config string
}
