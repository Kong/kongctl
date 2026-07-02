package aigateway

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func init() {
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldProviders, loadAIGatewayProviders)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldPolicies, loadAIGatewayPolicies)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldAgents, loadAIGatewayAgents)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldConsumers, loadAIGatewayConsumers)
	tableview.RegisterChildLoader(
		common.ViewParentAIGatewayConsumer,
		common.ViewFieldCredentials,
		loadAIGatewayConsumerCredentials,
	)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldConsumerGroups, loadAIGatewayConsumerGroups)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldModels, loadAIGatewayModels)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldMCPServers, loadAIGatewayMCPServers)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldVaults, loadAIGatewayVaults)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldNodes, loadAIGatewayNodes)
	tableview.RegisterChildLoader(
		common.ViewParentAIGateway,
		common.ViewFieldDataPlaneCertificates,
		loadAIGatewayDataPlaneCertificates,
	)
}

func loadAIGatewayProviders(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	providerAPI := sdk.GetAIGatewayProvidersAPI()
	if providerAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Providers client is not available")
	}

	providers, err := fetchAIGatewayProviders(helper, providerAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayProviderChildView(providers), nil
}

func loadAIGatewayPolicies(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	policyAPI := sdk.GetAIGatewayPoliciesAPI()
	if policyAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Policies client is not available")
	}

	policies, err := fetchAIGatewayPolicies(helper, policyAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayPolicyChildView(policies), nil
}

func loadAIGatewayAgents(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	agentAPI := sdk.GetAIGatewayAgentsAPI()
	if agentAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Agents client is not available")
	}

	agents, err := fetchAIGatewayAgents(helper, agentAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayAgentChildView(agents), nil
}

func loadAIGatewayConsumers(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	consumerAPI := sdk.GetAIGatewayConsumersAPI()
	if consumerAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Consumers client is not available")
	}

	consumers, err := fetchAIGatewayConsumers(helper, consumerAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayConsumerChildView(gatewayID, consumers), nil
}

func loadAIGatewayConsumerCredentials(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, consumerID, err := aiGatewayConsumerCredentialParentIDs(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	consumerAPI := sdk.GetAIGatewayConsumersAPI()
	if consumerAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Consumers client is not available")
	}

	credentials, err := fetchAIGatewayConsumerCredentials(helper, consumerAPI, gatewayID, consumerID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayConsumerCredentialChildView(credentials), nil
}

func loadAIGatewayConsumerGroups(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	groupAPI := sdk.GetAIGatewayConsumerGroupsAPI()
	if groupAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Consumer Groups client is not available")
	}

	groups, err := fetchAIGatewayConsumerGroups(helper, groupAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayConsumerGroupChildView(groups), nil
}

func loadAIGatewayModels(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	modelAPI := sdk.GetAIGatewayModelAPI()
	if modelAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Models client is not available")
	}

	models, err := listAIGatewayModels(helper, modelAPI, gatewayID)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayModelChildView(models), nil
}

func loadAIGatewayMCPServers(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	serverAPI := sdk.GetAIGatewayMCPServersAPI()
	if serverAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway MCP Servers client is not available")
	}

	servers, err := fetchAIGatewayMCPServers(helper, serverAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayMCPServerChildView(servers), nil
}

func loadAIGatewayVaults(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	vaultAPI := sdk.GetAIGatewayVaultsAPI()
	if vaultAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Vaults client is not available")
	}

	vaults, err := fetchAIGatewayVaults(helper, vaultAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayVaultChildView(vaults), nil
}

func loadAIGatewayNodes(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	nodeAPI := sdk.GetAIGatewayNodesAPI()
	if nodeAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Nodes client is not available")
	}

	nodes, err := fetchAIGatewayNodes(helper, nodeAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayNodeChildView(nodes), nil
}

func loadAIGatewayDataPlaneCertificates(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	certAPI := sdk.GetAIGatewayDataPlaneCertificatesAPI()
	if certAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway data plane certificates client is not available")
	}

	certs, err := fetchAIGatewayDataPlaneCertificates(helper, certAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayDataPlaneCertificateChildView(certs), nil
}

func aiGatewayConsumerCredentialParentIDs(parent any) (string, string, error) {
	if parent == nil {
		return "", "", fmt.Errorf("AI Gateway Consumer parent is nil")
	}

	switch p := parent.(type) {
	case *aiGatewayConsumerDetailContext:
		gatewayID := strings.TrimSpace(p.GatewayID)
		consumerID := strings.TrimSpace(p.Consumer.ID)
		if gatewayID == "" {
			return "", "", fmt.Errorf("AI Gateway identifier is missing")
		}
		if consumerID == "" {
			return "", "", fmt.Errorf("AI Gateway Consumer identifier is missing")
		}
		return gatewayID, consumerID, nil
	case aiGatewayConsumerDetailContext:
		gatewayID := strings.TrimSpace(p.GatewayID)
		consumerID := strings.TrimSpace(p.Consumer.ID)
		if gatewayID == "" {
			return "", "", fmt.Errorf("AI Gateway identifier is missing")
		}
		if consumerID == "" {
			return "", "", fmt.Errorf("AI Gateway Consumer identifier is missing")
		}
		return gatewayID, consumerID, nil
	default:
		return "", "", fmt.Errorf("unexpected parent type %T", parent)
	}
}

func aiGatewayIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("AI Gateway parent is nil")
	}

	switch p := parent.(type) {
	case *kkComps.AIGateway:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("AI Gateway identifier is missing")
		}
		return id, nil
	case kkComps.AIGateway:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("AI Gateway identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}
