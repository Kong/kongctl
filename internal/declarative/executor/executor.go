package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/deck"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util/normalizers"
	"golang.org/x/sync/errgroup"
)

type (
	createAIGatewayDataPlaneCertificateRequest = kkComps.CreateAIGatewayDataPlaneCertificateRequest
	createAIGatewayConsumerCredentialRequest   = kkComps.CreateAIGatewayConsumerCredentialRequest
)

// Executor handles the execution of declarative configuration plans
type Executor struct {
	client   *state.Client
	reporter ProgressReporter
	dryRun   bool
	// Track created resources during execution
	createdResources map[string]string // changeID -> resourceID
	cacheMu          sync.RWMutex
	// Track resource refs to IDs for reference resolution
	refToID map[string]map[string]string // resourceType -> ref -> resourceID
	// Unified state cache
	stateCache *state.Cache

	mu          sync.Mutex
	concurrency int

	// Resource executors
	portalExecutor       *BaseExecutor[kkComps.CreatePortal, kkComps.UpdatePortal]
	controlPlaneExecutor *BaseExecutor[kkComps.CreateControlPlaneRequest, kkComps.UpdateControlPlaneRequest]
	apiExecutor          *BaseExecutor[kkComps.CreateAPIRequest, kkComps.UpdateAPIRequest]
	authStrategyExecutor *BaseExecutor[
		kkComps.CreateAppAuthStrategyRequest,
		kkComps.UpdateAppAuthStrategyRequest,
	]
	dcrProviderExecutor *BaseExecutor[
		kkComps.CreateDcrProviderRequest,
		kkComps.UpdateDcrProviderRequest,
	]
	catalogServiceExecutor    *BaseExecutor[kkComps.CreateCatalogService, kkComps.UpdateCatalogService]
	aiGatewayExecutor         *BaseExecutor[kkComps.CreateAIGatewayRequest, kkComps.UpdateAIGatewayRequest]
	aiGatewayProviderExecutor *BaseExecutor[
		kkComps.CreateAIGatewayProviderRequest,
		kkComps.UpdateAIGatewayProviderRequest]
	aiGatewayPolicyExecutor *BaseExecutor[
		kkComps.CreateAIGatewayPolicyRequest,
		kkComps.UpdateAIGatewayPolicyRequest]
	aiGatewayAgentExecutor *BaseExecutor[
		kkComps.CreateAIGatewayAgentRequest,
		kkComps.UpdateAIGatewayAgentRequest]
	aiGatewayConsumerExecutor *BaseExecutor[
		kkComps.CreateAIGatewayConsumerRequest,
		kkComps.UpdateAIGatewayConsumerRequest]
	aiGatewayConsumerCredentialExecutor *BaseCreateDeleteExecutor[createAIGatewayConsumerCredentialRequest]
	aiGatewayConsumerGroupExecutor      *BaseExecutor[
		kkComps.CreateAIGatewayConsumerGroupRequest,
		kkComps.UpdateAIGatewayConsumerGroupRequest]
	aiGatewayModelExecutor *BaseExecutor[
		kkComps.CreateAIGatewayModelRequest, kkComps.UpdateAIGatewayModelRequest]
	aiGatewayMCPServerExecutor *BaseExecutor[
		kkComps.CreateAIGatewayMCPServerRequest, kkComps.UpdateAIGatewayMCPServerRequest]
	aiGatewayVaultExecutor *BaseExecutor[
		kkComps.CreateAIGatewayVaultRequest, kkComps.UpdateAIGatewayVaultRequest]
	aiGatewayDataPlaneCertificateExecutor  *BaseCreateDeleteExecutor[createAIGatewayDataPlaneCertificateRequest]
	dashboardExecutor                      *BaseExecutor[kkComps.DashboardUpdateRequest, kkComps.DashboardUpdateRequest]
	eventGatewayControlPlaneExecutor       *BaseExecutor[kkComps.CreateGatewayRequest, kkComps.UpdateGatewayRequest]
	organizationTeamExecutor               *BaseExecutor[kkComps.CreateTeam, kkComps.UpdateTeam]
	organizationTeamRoleExecutor           *BaseExecutor[kkComps.AssignRole, kkComps.AssignRole]
	organizationUserTeamMembershipExecutor *BaseExecutor[
		state.OrganizationUserTeamMembership,
		state.OrganizationUserTeamMembership,
	]
	organizationUserRoleExecutor                    *BaseExecutor[kkComps.AssignRole, kkComps.AssignRole]
	organizationSystemAccountTeamMembershipExecutor *BaseExecutor[
		state.OrganizationSystemAccountTeamMembership,
		state.OrganizationSystemAccountTeamMembership,
	]
	organizationSystemAccountRoleExecutor    *BaseExecutor[kkComps.AssignRole, kkComps.AssignRole]
	controlPlaneDataPlaneCertificateExecutor *BaseCreateDeleteExecutor[kkComps.DataPlaneClientCertificateRequest]

	// Event Gateway child resource executors
	eventGatewayBackendClusterExecutor *BaseExecutor[
		kkComps.CreateBackendClusterRequest, kkComps.UpdateBackendClusterRequest]
	eventGatewayVirtualClusterExecutor *BaseExecutor[
		kkComps.CreateVirtualClusterRequest, kkComps.UpdateVirtualClusterRequest]
	eventGatewayListenerExecutor *BaseExecutor[
		kkComps.CreateEventGatewayListenerRequest, kkComps.UpdateEventGatewayListenerRequest]
	eventGatewayListenerPolicyExecutor *BaseExecutor[
		kkComps.EventGatewayListenerPolicyCreate, kkComps.EventGatewayListenerPolicyUpdate]
	eventGatewayClusterPolicyExecutor *BaseExecutor[
		kkComps.EventGatewayClusterPolicyModify, kkComps.EventGatewayClusterPolicyModify]
	eventGatewayProducePolicyExecutor *BaseExecutor[
		kkComps.EventGatewayProducePolicyCreate, kkComps.EventGatewayProducePolicyUpdate]
	eventGatewayConsumePolicyExecutor *BaseExecutor[
		kkComps.EventGatewayConsumePolicyCreate, kkComps.EventGatewayConsumePolicyUpdate]
	eventGatewayDataPlaneCertificateExecutor *BaseExecutor[
		kkComps.CreateEventGatewayDataPlaneCertificateRequest,
		kkComps.UpdateEventGatewayDataPlaneCertificateRequest]
	eventGatewaySchemaRegistryExecutor *BaseExecutor[
		kkComps.SchemaRegistryCreate, kkComps.SchemaRegistryUpdate]
	eventGatewayStaticKeyExecutor *BaseExecutor[
		kkComps.EventGatewayStaticKeyCreate, kkComps.EventGatewayStaticKeyCreate]
	eventGatewayTLSTrustBundleExecutor *BaseExecutor[
		kkComps.CreateTLSTrustBundleRequest, kkComps.UpdateTLSTrustBundleRequest]

	// Portal child resource executors
	portalCustomizationExecutor    *BaseSingletonExecutor[kkComps.PortalCustomizationV3]
	portalAuthSettingsExecutor     *BaseSingletonExecutor[kkComps.PortalAuthenticationSettingsUpdateRequest]
	portalIntegrationExecutor      *BaseSingletonExecutor[kkComps.PortalIntegrations]
	portalIdentityProviderExecutor *BaseExecutor[kkComps.CreateIdentityProvider, kkComps.UpdateIdentityProvider]
	portalAssetLogoExecutor        *BaseSingletonExecutor[kkComps.ReplacePortalImageAsset]
	portalAssetFaviconExecutor     *BaseSingletonExecutor[kkComps.ReplacePortalImageAsset]
	portalDomainExecutor           *BaseExecutor[kkComps.CreatePortalCustomDomainRequest,
		kkComps.UpdatePortalCustomDomainRequest]
	portalIPAllowListExecutor *BaseExecutor[
		kkComps.CreatePortalSourceIPRestriction,
		kkComps.UpdatePortalSourceIPRestriction]
	portalPageExecutor             *BaseExecutor[kkComps.CreatePortalPageRequest, kkComps.UpdatePortalPageRequest]
	portalSnippetExecutor          *BaseExecutor[kkComps.CreatePortalSnippetRequest, kkComps.UpdatePortalSnippetRequest]
	portalTeamExecutor             *BaseExecutor[kkComps.PortalCreateTeamRequest, kkComps.PortalUpdateTeamRequest]
	portalTeamGroupMappingExecutor *PortalTeamGroupMappingExecutor
	portalTeamRoleExecutor         *BaseExecutor[kkComps.PortalAssignRoleRequest, kkComps.PortalAssignRoleRequest]
	portalEmailConfigExecutor      *BaseExecutor[kkComps.PostPortalEmailConfig, kkComps.PatchPortalEmailConfig]
	portalAuditLogWebhookExecutor  *BaseExecutor[
		kkComps.UpdatePortalAuditLogWebhook,
		kkComps.UpdatePortalAuditLogWebhook]
	portalEmailTemplateExecutor *BaseExecutor[kkOps.UpdatePortalCustomEmailTemplateRequest,
		kkOps.UpdatePortalCustomEmailTemplateRequest]

	// API child resource executors
	apiVersionExecutor     *BaseExecutor[kkComps.CreateAPIVersionRequest, kkComps.APIVersionRequest]
	apiPublicationExecutor *BaseCreateDeleteExecutor[kkComps.APIPublication]
	apiDocumentExecutor    *BaseExecutor[kkComps.CreateAPIDocumentRequest, kkComps.APIDocument]
	// API implementation is not yet supported by SDK but we include adapter for completeness
	apiImplementationExecutor *BaseCreateDeleteExecutor[kkComps.APIImplementation]

	deckRunner         deck.Runner
	konnectToken       string
	konnectTokenSource deck.KonnectTokenSource
	konnectBaseURL     string
	executionMode      planner.PlanMode
	planBaseDir        string
}

// DefaultMaxConcurrency is the default --max-concurrency value.
// At ~200ms/request, 5 concurrent workers sustain ~1500 req/min, staying under
// a 2000 req/min budget with comfortable headroom.
const DefaultMaxConcurrency = 5

// MaxConcurrency is the maximum allowed concurrent operations.
// Assuming a 200ms response time and 6000 req/min rate limit, this runs out in 6s.
const MaxConcurrency = 200

// MinConcurrency is the minimum allowed concurrent operations.
const MinConcurrency = 1

// Options configures executor behavior.
type Options struct {
	DeckRunner         deck.Runner
	KonnectToken       string
	KonnectTokenSource deck.KonnectTokenSource
	KonnectBaseURL     string
	Mode               planner.PlanMode
	PlanBaseDir        string
	// MaxConcurrency sets the maximum number of concurrent operations. Defaults to DefaultMaxConcurrency.
	MaxConcurrency int
}

// New creates a new Executor instance with default options.
func New(client *state.Client, reporter ProgressReporter, dryRun bool) *Executor {
	return NewWithOptions(client, reporter, dryRun, Options{})
}

// NewWithOptions creates a new Executor instance.
func NewWithOptions(client *state.Client, reporter ProgressReporter, dryRun bool, opts Options) *Executor {
	deckRunner := opts.DeckRunner
	if deckRunner == nil {
		deckRunner = deck.NewRunner()
	}
	e := &Executor{
		client:             client,
		reporter:           reporter,
		dryRun:             dryRun,
		createdResources:   make(map[string]string),
		refToID:            make(map[string]map[string]string),
		stateCache:         state.NewCache(),
		deckRunner:         deckRunner,
		konnectToken:       opts.KonnectToken,
		konnectTokenSource: opts.KonnectTokenSource,
		konnectBaseURL:     opts.KonnectBaseURL,
		executionMode:      opts.Mode,
		planBaseDir:        strings.TrimSpace(opts.PlanBaseDir),
	}

	e.concurrency = DefaultMaxConcurrency
	// If user has overridden MaxConcurrency, use it. Ensure it's within allowed bounds.
	if opts.MaxConcurrency > 0 {
		if opts.MaxConcurrency > MaxConcurrency {
			e.concurrency = MaxConcurrency
		} else if opts.MaxConcurrency < MinConcurrency {
			e.concurrency = MinConcurrency
		} else {
			e.concurrency = opts.MaxConcurrency
		}
	}

	// Initialize resource executors
	e.portalExecutor = NewBaseExecutor[kkComps.CreatePortal, kkComps.UpdatePortal](
		NewPortalAdapter(client),
		client,
		dryRun,
	)
	e.controlPlaneExecutor = NewBaseExecutor[kkComps.CreateControlPlaneRequest, kkComps.UpdateControlPlaneRequest](
		NewControlPlaneAdapter(client),
		client,
		dryRun,
	)
	e.apiExecutor = NewBaseExecutor[kkComps.CreateAPIRequest, kkComps.UpdateAPIRequest](
		NewAPIAdapter(client),
		client,
		dryRun,
	)
	e.authStrategyExecutor = NewBaseExecutor[kkComps.CreateAppAuthStrategyRequest, kkComps.UpdateAppAuthStrategyRequest](
		NewAuthStrategyAdapter(client),
		client,
		dryRun,
	)
	e.dcrProviderExecutor = NewBaseExecutor[kkComps.CreateDcrProviderRequest, kkComps.UpdateDcrProviderRequest](
		NewDCRProviderAdapter(client),
		client,
		dryRun,
	)
	e.catalogServiceExecutor = NewBaseExecutor[kkComps.CreateCatalogService, kkComps.UpdateCatalogService](
		NewCatalogServiceAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayExecutor = NewBaseExecutor[kkComps.CreateAIGatewayRequest, kkComps.UpdateAIGatewayRequest](
		NewAIGatewayAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayProviderExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayProviderRequest,
		kkComps.UpdateAIGatewayProviderRequest](
		NewAIGatewayProviderAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayPolicyExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayPolicyRequest,
		kkComps.UpdateAIGatewayPolicyRequest](
		NewAIGatewayPolicyAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayAgentExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayAgentRequest,
		kkComps.UpdateAIGatewayAgentRequest](
		NewAIGatewayAgentAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayConsumerExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayConsumerRequest,
		kkComps.UpdateAIGatewayConsumerRequest](
		NewAIGatewayConsumerAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayConsumerCredentialExecutor = NewBaseCreateDeleteExecutor[createAIGatewayConsumerCredentialRequest](
		NewAIGatewayConsumerCredentialAdapter(client),
		dryRun,
	)
	e.aiGatewayConsumerGroupExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayConsumerGroupRequest,
		kkComps.UpdateAIGatewayConsumerGroupRequest](
		NewAIGatewayConsumerGroupAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayModelExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayModelRequest, kkComps.UpdateAIGatewayModelRequest](
		NewAIGatewayModelAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayMCPServerExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayMCPServerRequest, kkComps.UpdateAIGatewayMCPServerRequest](
		NewAIGatewayMCPServerAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayVaultExecutor = NewBaseExecutor[
		kkComps.CreateAIGatewayVaultRequest, kkComps.UpdateAIGatewayVaultRequest](
		NewAIGatewayVaultAdapter(client),
		client,
		dryRun,
	)
	e.aiGatewayDataPlaneCertificateExecutor = NewBaseCreateDeleteExecutor[createAIGatewayDataPlaneCertificateRequest](
		NewAIGatewayDataPlaneCertificateAdapter(client),
		dryRun,
	)
	e.dashboardExecutor = NewBaseExecutor[kkComps.DashboardUpdateRequest, kkComps.DashboardUpdateRequest](
		NewDashboardAdapter(client),
		client,
		dryRun,
	)
	e.eventGatewayControlPlaneExecutor = NewBaseExecutor[kkComps.CreateGatewayRequest, kkComps.UpdateGatewayRequest](
		NewEventGatewayControlPlaneControlPlaneAdapter(client),
		client,
		dryRun,
	)
	e.organizationTeamExecutor = NewBaseExecutor[kkComps.CreateTeam, kkComps.UpdateTeam](
		NewOrganizationTeamAdapter(client),
		client,
		dryRun,
	)
	e.organizationTeamRoleExecutor = NewBaseExecutor[kkComps.AssignRole, kkComps.AssignRole](
		NewOrganizationTeamRoleAdapter(client),
		client,
		dryRun,
	)
	e.organizationUserTeamMembershipExecutor = NewBaseExecutor[
		state.OrganizationUserTeamMembership, state.OrganizationUserTeamMembership](
		NewOrganizationUserTeamMembershipAdapter(client),
		client,
		dryRun,
	)
	e.organizationUserRoleExecutor = NewBaseExecutor[kkComps.AssignRole, kkComps.AssignRole](
		NewOrganizationUserRoleAdapter(client),
		client,
		dryRun,
	)
	e.organizationSystemAccountTeamMembershipExecutor = NewBaseExecutor[
		state.OrganizationSystemAccountTeamMembership, state.OrganizationSystemAccountTeamMembership](
		NewOrganizationSystemAccountTeamMembershipAdapter(client),
		client,
		dryRun,
	)
	e.organizationSystemAccountRoleExecutor = NewBaseExecutor[kkComps.AssignRole, kkComps.AssignRole](
		NewOrganizationSystemAccountRoleAdapter(client),
		client,
		dryRun,
	)
	e.controlPlaneDataPlaneCertificateExecutor = NewBaseCreateDeleteExecutor[kkComps.DataPlaneClientCertificateRequest](
		NewControlPlaneDataPlaneCertificateAdapter(client),
		dryRun,
	)

	// Initialize event gateway child resource executors
	e.eventGatewayBackendClusterExecutor = NewBaseExecutor[
		kkComps.CreateBackendClusterRequest, kkComps.UpdateBackendClusterRequest](
		NewEventGatewayBackendClusterAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayVirtualClusterExecutor = NewBaseExecutor[
		kkComps.CreateVirtualClusterRequest, kkComps.UpdateVirtualClusterRequest](
		NewEventGatewayVirtualClusterAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayListenerExecutor = NewBaseExecutor[
		kkComps.CreateEventGatewayListenerRequest, kkComps.UpdateEventGatewayListenerRequest](
		NewEventGatewayListenerAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayListenerPolicyExecutor = NewBaseExecutor[
		kkComps.EventGatewayListenerPolicyCreate, kkComps.EventGatewayListenerPolicyUpdate](
		NewEventGatewayListenerPolicyAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayClusterPolicyExecutor = NewBaseExecutor[
		kkComps.EventGatewayClusterPolicyModify, kkComps.EventGatewayClusterPolicyModify](
		NewEventGatewayClusterPolicyAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayProducePolicyExecutor = NewBaseExecutor[
		kkComps.EventGatewayProducePolicyCreate, kkComps.EventGatewayProducePolicyUpdate](
		NewEventGatewayProducePolicyAdapter(client),
		client,
		dryRun,
	)
	e.eventGatewayConsumePolicyExecutor = NewBaseExecutor[
		kkComps.EventGatewayConsumePolicyCreate, kkComps.EventGatewayConsumePolicyUpdate](
		NewEventGatewayConsumePolicyAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayDataPlaneCertificateExecutor = NewBaseExecutor[
		kkComps.CreateEventGatewayDataPlaneCertificateRequest,
		kkComps.UpdateEventGatewayDataPlaneCertificateRequest](
		NewEventGatewayDataPlaneCertificateAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewaySchemaRegistryExecutor = NewBaseExecutor[
		kkComps.SchemaRegistryCreate, kkComps.SchemaRegistryUpdate](
		NewEventGatewaySchemaRegistryAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayStaticKeyExecutor = NewBaseExecutor[
		kkComps.EventGatewayStaticKeyCreate, kkComps.EventGatewayStaticKeyCreate](
		NewEventGatewayStaticKeyAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayTLSTrustBundleExecutor = NewBaseExecutor[
		kkComps.CreateTLSTrustBundleRequest, kkComps.UpdateTLSTrustBundleRequest](
		NewEventGatewayTLSTrustBundleAdapter(client),
		client,
		dryRun,
	)

	// Initialize portal child resource executors
	e.portalCustomizationExecutor = NewBaseSingletonExecutor[kkComps.PortalCustomizationV3](
		NewPortalCustomizationAdapter(client),
		dryRun,
	)
	e.portalAuthSettingsExecutor = NewBaseSingletonExecutor[kkComps.PortalAuthenticationSettingsUpdateRequest](
		NewPortalAuthSettingsAdapter(client),
		dryRun,
	)
	e.portalIntegrationExecutor = NewBaseSingletonExecutor[kkComps.PortalIntegrations](
		NewPortalIntegrationAdapter(client),
		dryRun,
	)
	e.portalIdentityProviderExecutor = NewBaseExecutor[kkComps.CreateIdentityProvider, kkComps.UpdateIdentityProvider](
		NewPortalIdentityProviderAdapter(client),
		client,
		dryRun,
	)
	e.portalAssetLogoExecutor = NewBaseSingletonExecutor[kkComps.ReplacePortalImageAsset](
		NewPortalAssetLogoAdapter(client),
		dryRun,
	)
	e.portalAssetFaviconExecutor = NewBaseSingletonExecutor[kkComps.ReplacePortalImageAsset](
		NewPortalAssetFaviconAdapter(client),
		dryRun,
	)
	e.portalDomainExecutor = NewBaseExecutor[kkComps.CreatePortalCustomDomainRequest,
		kkComps.UpdatePortalCustomDomainRequest](
		NewPortalDomainAdapter(client),
		client,
		dryRun,
	)
	e.portalIPAllowListExecutor = NewBaseExecutor[
		kkComps.CreatePortalSourceIPRestriction,
		kkComps.UpdatePortalSourceIPRestriction,
	](
		NewPortalIPAllowListAdapter(client),
		client,
		dryRun,
	)
	e.portalPageExecutor = NewBaseExecutor[kkComps.CreatePortalPageRequest, kkComps.UpdatePortalPageRequest](
		NewPortalPageAdapter(client),
		client,
		dryRun,
	)
	e.portalSnippetExecutor = NewBaseExecutor[kkComps.CreatePortalSnippetRequest, kkComps.UpdatePortalSnippetRequest](
		NewPortalSnippetAdapter(client),
		client,
		dryRun,
	)
	e.portalTeamExecutor = NewBaseExecutor[kkComps.PortalCreateTeamRequest, kkComps.PortalUpdateTeamRequest](
		NewPortalTeamAdapter(client),
		client,
		dryRun,
	)
	e.portalTeamGroupMappingExecutor = NewPortalTeamGroupMappingExecutor(client, dryRun)
	e.portalTeamRoleExecutor = NewBaseExecutor[kkComps.PortalAssignRoleRequest, kkComps.PortalAssignRoleRequest](
		NewPortalTeamRoleAdapter(client),
		client,
		dryRun,
	)
	e.portalEmailConfigExecutor = NewBaseExecutor[kkComps.PostPortalEmailConfig, kkComps.PatchPortalEmailConfig](
		NewPortalEmailConfigAdapter(client),
		client,
		dryRun,
	)
	e.portalAuditLogWebhookExecutor = NewBaseExecutor[
		kkComps.UpdatePortalAuditLogWebhook,
		kkComps.UpdatePortalAuditLogWebhook,
	](
		NewPortalAuditLogWebhookAdapter(client),
		client,
		dryRun,
	)
	e.portalEmailTemplateExecutor = NewBaseExecutor[kkOps.UpdatePortalCustomEmailTemplateRequest,
		kkOps.UpdatePortalCustomEmailTemplateRequest](
		NewPortalEmailTemplateAdapter(client),
		client,
		dryRun,
	)

	// Initialize API child resource executors
	e.apiVersionExecutor = NewBaseExecutor[kkComps.CreateAPIVersionRequest, kkComps.APIVersionRequest](
		NewAPIVersionAdapter(client),
		client,
		dryRun,
	)
	e.apiPublicationExecutor = NewBaseCreateDeleteExecutor[kkComps.APIPublication](
		NewAPIPublicationAdapter(client),
		dryRun,
	)
	e.apiDocumentExecutor = NewBaseExecutor[kkComps.CreateAPIDocumentRequest, kkComps.APIDocument](
		NewAPIDocumentAdapter(client),
		client,
		dryRun,
	)

	e.apiImplementationExecutor = NewBaseCreateDeleteExecutor[kkComps.APIImplementation](
		NewAPIImplementationAdapter(client),
		dryRun,
	)

	return e
}

// getRef retrieves a resource ID from the refToID cache (thread-safe).
func (e *Executor) getRef(resourceType, ref string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if m, ok := e.refToID[resourceType]; ok {
		id, found := m[ref]
		return id, found
	}
	return "", false
}

// getRefAny retrieves a resource ID from refToID, trying each ref in order (thread-safe).
func (e *Executor) getRefAny(resourceType string, refs ...string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.refToID[resourceType]
	if !ok {
		return "", false
	}
	for _, ref := range refs {
		if id, found := m[ref]; found {
			return id, true
		}
	}
	return "", false
}

// setRef stores a resource ID in the refToID cache (thread-safe).
func (e *Executor) setRef(resourceType, ref, id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.refToID[resourceType] == nil {
		e.refToID[resourceType] = make(map[string]string)
	}
	e.refToID[resourceType][ref] = id
}

// Execute runs the plan and returns the execution result
func (e *Executor) Execute(ctx context.Context, plan *planner.Plan) *ExecutionResult {
	ctx = withExecutorHTTPLogContext(ctx, e.executionMode)

	result := &ExecutionResult{
		DryRun: e.dryRun,
	}

	// Notify reporter of execution start
	if e.reporter != nil {
		e.reporter.StartExecution(plan)
	}

	// Choose execution strategy.
	// Planner is the source of truth for dependency ordering and concurrency groups.
	// If groups are present, execute by groups; otherwise execute sequentially in
	// the provided ExecutionOrder (legacy plans).
	// Sequential execution is retained for backwards compatibility for existing plans.
	if len(plan.ExecutionGroups) > 0 {
		e.executeGroupsConcurrent(ctx, plan, result)
	} else {
		for _, changeID := range plan.ExecutionOrder {
			// Find the change by ID
			var change *planner.PlannedChange
			for j := range plan.Changes {
				if plan.Changes[j].ID == changeID {
					change = &plan.Changes[j]
					break
				}
			}

			if change == nil {
				// This shouldn't happen, but handle gracefully
				err := fmt.Errorf("change with ID %s not found in plan", changeID)
				result.Errors = append(result.Errors, ExecutionError{
					ChangeID: changeID,
					Error:    err.Error(),
				})
				result.FailureCount++
				continue
			}

			// Execute the change, the error will be captured in result
			changeCtx := withExecutorChangeHTTPLogContext(ctx, change)
			_ = e.executeChange(changeCtx, result, change, plan)
		}
	}

	// Notify reporter of execution completion
	if e.reporter != nil {
		e.reporter.FinishExecution(result)
	}

	return result
}

// executeGroupsConcurrent executes plan.ExecutionGroups in strict order: all changes
// in group N must complete before group N+1 starts. Within each group, changes run
// concurrently up to e.concurrency. Changes whose direct dependencies failed or were
// blocked are recorded as "blocked" and skipped rather than executed.
func (e *Executor) executeGroupsConcurrent(
	ctx context.Context,
	plan *planner.Plan,
	result *ExecutionResult,
) {
	changeByID := make(map[string]*planner.PlannedChange, len(plan.Changes))
	for i := range plan.Changes {
		changeByID[plan.Changes[i].ID] = &plan.Changes[i]
	}

	// blockedOrFailed tracks IDs of changes that failed or were blocked so that
	// downstream changes in later groups can be identified and skipped.
	blockedOrFailed := make(map[string]bool)

	for _, group := range plan.ExecutionGroups {
		var runnableIDs []string

		// Classify each change in this group: blocked (dependency failed) or runnable.
		for _, changeID := range group {
			change := changeByID[changeID]
			if change == nil {
				e.mu.Lock()
				result.Errors = append(result.Errors, ExecutionError{
					ChangeID: changeID,
					Error:    fmt.Sprintf("change with ID %s not found in plan", changeID),
				})
				result.FailureCount++
				e.mu.Unlock()
				blockedOrFailed[changeID] = true
				continue
			}
			blockers := findBlockers(change, blockedOrFailed)
			if len(blockers) > 0 {
				blockedOrFailed[changeID] = true
				e.mu.Lock()
				result.SkippedCount++
				e.mu.Unlock()
				if e.reporter != nil {
					e.reporter.SkipChange(*change,
						fmt.Sprintf("blocked by failed dependencies: %v", blockers))
				}
			} else {
				runnableIDs = append(runnableIDs, changeID)
			}
		}

		if len(runnableIDs) == 0 {
			continue
		}

		// Track which changes in THIS group fail so they can be added to blockedOrFailed
		// after the group completes (preventing data races on the shared map).
		var groupMu sync.Mutex
		groupFailed := make(map[string]bool)

		var g errgroup.Group
		g.SetLimit(e.concurrency)

		for _, changeID := range runnableIDs {
			ch := changeByID[changeID]

			g.Go(func() error {
				changeCtx := withExecutorChangeHTTPLogContext(ctx, ch)
				err := e.executeChange(changeCtx, result, ch, plan)
				if err != nil {
					groupMu.Lock()
					groupFailed[ch.ID] = true
					groupMu.Unlock()
				}

				// Never return an error to errgroup — errors are already stored in result.
				return nil
			})
		}

		_ = g.Wait()

		// Promote this group's failures into the shared blockedOrFailed set.
		groupMu.Lock()
		for id := range groupFailed {
			blockedOrFailed[id] = true
		}
		groupMu.Unlock()
	}
}

// findBlockers returns the subset of change.DependsOn whose IDs are present in
// blockedOrFailed. A non-empty return value means this change must be skipped.
func findBlockers(change *planner.PlannedChange, blockedOrFailed map[string]bool) []string {
	if change == nil || len(change.DependsOn) == 0 {
		return nil
	}
	var blockers []string
	for _, dep := range change.DependsOn {
		if blockedOrFailed[dep] {
			blockers = append(blockers, dep)
		}
	}
	return blockers
}

func withExecutorHTTPLogContext(ctx context.Context, mode planner.PlanMode) context.Context {
	return log.WithHTTPLogContext(ctx, log.HTTPLogContext{
		Workflow:      "declarative",
		WorkflowPhase: "executor",
		WorkflowMode:  string(mode),
	})
}

func withExecutorChangeHTTPLogContext(ctx context.Context, change *planner.PlannedChange) context.Context {
	if change == nil {
		return ctx
	}

	update := log.HTTPLogContext{
		WorkflowComponent: change.ResourceType,
		WorkflowAction:    strings.ToLower(string(change.Action)),
		WorkflowChangeID:  change.ID,
		WorkflowResource:  change.ResourceType,
		WorkflowRef:       change.ResourceRef,
	}

	if namespace := strings.TrimSpace(change.Namespace); namespace != "" {
		update.WorkflowNamespace = namespace
	}

	return log.WithHTTPLogContext(ctx, update)
}

// executeChange executes a single change from the plan
func (e *Executor) executeChange(ctx context.Context, result *ExecutionResult, change *planner.PlannedChange,
	plan *planner.Plan,
) error {
	// Notify reporter of change start
	if e.reporter != nil {
		e.reporter.StartChange(*change)
	}

	resourceName := change.ResourceRef
	if resolvedName := getResourceName(change.Fields); resolvedName != "" && !tags.IsEnvPlaceholder(resolvedName) {
		resourceName = resolvedName
	}

	if err := e.resolveDeferredEnvPlaceholders(change); err != nil {
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		e.mu.Lock()
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
		e.mu.Unlock()

		if e.reporter != nil {
			e.reporter.CompleteChange(*change, err)
		}

		return err
	}

	if resolvedName := getResourceName(change.Fields); resolvedName != "" {
		resourceName = resolvedName
	}

	// Hydrate unresolved refs/parent IDs from already-completed dependency creates.
	// This restores deterministic downstream ID propagation without mutating
	// future groups concurrently.
	e.hydrateKnownReferenceIDs(change, plan)

	// Pre-execution validation (always performed, even in dry-run)
	if err := e.validateChangePreExecution(ctx, *change); err != nil {
		// Record error
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		e.mu.Lock()
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
		// In dry-run, also record validation result
		if e.dryRun {
			result.ValidationResults = append(result.ValidationResults, ValidationResult{
				ChangeID:     change.ID,
				ResourceType: change.ResourceType,
				ResourceName: resourceName,
				ResourceRef:  change.ResourceRef,
				Action:       string(change.Action),
				Status:       "would_fail",
				Validation:   "failed",
				Message:      err.Error(),
			})
		}
		e.mu.Unlock()

		// Notify reporter
		if e.reporter != nil {
			e.reporter.CompleteChange(*change, err)
		}

		return err
	}

	// If dry-run, skip actual execution
	if e.dryRun {
		e.mu.Lock()
		result.SkippedCount++
		result.ValidationResults = append(result.ValidationResults, ValidationResult{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Status:       "would_succeed",
			Validation:   "passed",
		})
		e.mu.Unlock()

		if e.reporter != nil {
			e.reporter.SkipChange(*change, "dry-run mode")
		}

		return nil
	}

	// Execute the actual change
	var err error
	var resourceID string

	switch change.Action {
	case planner.ActionCreate:
		if change.ResourceType == planner.ResourceTypeDeck {
			err = e.executeDeckStep(ctx, change, plan)
		} else {
			resourceID, err = e.createResource(ctx, change)
		}
	case planner.ActionExternalTool:
		if change.ResourceType != planner.ResourceTypeDeck {
			err = fmt.Errorf("external tool action is only supported for %s resources", planner.ResourceTypeDeck)
		} else {
			err = e.executeDeckStep(ctx, change, plan)
		}
	case planner.ActionUpdate:
		resourceID, err = e.updateResource(ctx, change)
	case planner.ActionDelete:
		err = e.deleteResource(ctx, change)
		resourceID = change.ResourceID
	default:
		err = fmt.Errorf("unknown action: %s", change.Action)
	}

	// Record result
	e.mu.Lock()
	if err != nil {
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
	} else {
		result.SuccessCount++
		result.ChangesApplied = append(result.ChangesApplied, AppliedChange{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			ResourceID:   resourceID,
		})

		// Track created resources for dependencies
		if change.Action == planner.ActionCreate && resourceID != "" {
			e.createdResources[change.ID] = resourceID

			// Track by resource type and ref so resolve*Ref helpers can find IDs
			// created during this execution without making additional API calls.
			if e.refToID[change.ResourceType] == nil {
				e.refToID[change.ResourceType] = make(map[string]string)
			}
			e.refToID[change.ResourceType][change.ResourceRef] = resourceID
		}
	}
	e.mu.Unlock()

	// Notify reporter
	if e.reporter != nil {
		e.reporter.CompleteChange(*change, err)
	}

	return err
}

// hydrateKnownReferenceIDs fills unresolved parent/reference IDs in-place using
// IDs from already executed dependency CREATE changes.
func (e *Executor) hydrateKnownReferenceIDs(change *planner.PlannedChange, plan *planner.Plan) {
	if change == nil || plan == nil {
		return
	}

	for field, refInfo := range change.References {
		if change.Fields != nil && !unresolvedReferenceID(refInfo.ID) {
			setResolvedFieldValue(change.Fields, field, refInfo.ID)
		}
	}

	if len(change.DependsOn) == 0 {
		return
	}

	depRefs := make(map[string]createdDependencyReference, len(change.DependsOn))
	for _, depID := range change.DependsOn {
		createdID, ok := e.getCreatedResourceID(depID)
		if !ok || createdID == "" {
			continue
		}

		depChange := findPlannedChangeByID(plan, depID)
		if depChange == nil || depChange.ResourceRef == "" {
			continue
		}

		depRefs[depChange.ResourceRef] = createdDependencyReference{
			id:          createdID,
			resourceRef: depChange.ResourceRef,
			fields:      depChange.Fields,
		}
	}

	if len(depRefs) == 0 {
		return
	}

	if change.Parent != nil && unresolvedReferenceID(change.Parent.ID) {
		if dep, ok := depRefs[normalizedRefValue(change.Parent.Ref)]; ok {
			change.Parent.ID = dep.id
		}
	}

	for field, refInfo := range change.References {
		updated := false

		if unresolvedReferenceID(refInfo.ID) {
			if dep, ok := depRefs[normalizedRefValue(refInfo.Ref)]; ok {
				resolvedValue := dep.referenceValue(refInfo.Ref)
				refInfo.ID = resolvedValue
				updated = true
				if change.Fields != nil {
					setResolvedFieldValue(change.Fields, field, resolvedValue)
				}
			}
		}

		if refInfo.IsArray && len(refInfo.Refs) > 0 {
			if len(refInfo.ResolvedIDs) < len(refInfo.Refs) {
				resized := make([]string, len(refInfo.Refs))
				copy(resized, refInfo.ResolvedIDs)
				refInfo.ResolvedIDs = resized
			}

			for i, ref := range refInfo.Refs {
				if refInfo.ResolvedIDs[i] != "" {
					continue
				}
				if dep, ok := depRefs[normalizedRefValue(ref)]; ok {
					refInfo.ResolvedIDs[i] = dep.referenceValue(ref)
					updated = true
				}
			}
		}

		if updated {
			change.References[field] = refInfo
		}
	}
}

type createdDependencyReference struct {
	id          string
	resourceRef string
	fields      map[string]any
}

func (r createdDependencyReference) referenceValue(ref string) string {
	field := planner.FieldID
	if tags.IsRefPlaceholder(ref) {
		if _, parsedField, ok := tags.ParseRefPlaceholder(ref); ok && parsedField != "" {
			field = parsedField
		}
	}

	switch field {
	case planner.FieldID, "ID":
		return r.id
	case planner.FieldName:
		if name := common.ExtractResourceName(r.fields); name != "" && name != resources.UnknownReferenceID {
			return name
		}
		if r.resourceRef != "" {
			return r.resourceRef
		}
		return r.id
	default:
		if value, ok := stringFieldPathValue(r.fields, field); ok {
			return value
		}
		return r.id
	}
}

func stringFieldPathValue(fields map[string]any, fieldPath string) (string, bool) {
	if fields == nil || fieldPath == "" {
		return "", false
	}
	value, ok := fieldPathValue(fields, strings.Split(fieldPath, "."))
	if !ok {
		return "", false
	}
	if fieldChange, ok := value.(planner.FieldChange); ok {
		value = fieldChange.New
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	case fmt.Stringer:
		return typed.String(), true
	default:
		return "", false
	}
}

func fieldPathValue(current any, segments []string) (any, bool) {
	if len(segments) == 0 {
		return current, true
	}
	if fieldChange, ok := current.(planner.FieldChange); ok {
		return fieldPathValue(fieldChange.New, segments)
	}
	fields, ok := current.(map[string]any)
	if !ok {
		return nil, false
	}
	next, ok := fields[segments[0]]
	if !ok {
		return nil, false
	}
	return fieldPathValue(next, segments[1:])
}

func (e *Executor) getCreatedResourceID(changeID string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	id, ok := e.createdResources[changeID]
	return id, ok
}

func findPlannedChangeByID(plan *planner.Plan, changeID string) *planner.PlannedChange {
	if plan == nil {
		return nil
	}
	for i := range plan.Changes {
		if plan.Changes[i].ID == changeID {
			return &plan.Changes[i]
		}
	}
	return nil
}

func normalizedRefValue(ref string) string {
	if tags.IsRefPlaceholder(ref) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(ref); ok && parsedRef != "" {
			return parsedRef
		}
	}
	return ref
}

// validateChangePreExecution performs validation before executing a change
func (e *Executor) validateChangePreExecution(ctx context.Context, change planner.PlannedChange) error {
	switch change.Action {
	case planner.ActionCreate:
		// Create operations proceed directly to execution. Resource-specific
		// create handlers are responsible for any required validation.
		return nil
	case planner.ActionExternalTool:
		return nil
	case planner.ActionUpdate, planner.ActionDelete:
		// For update/delete, verify resource still exists and check protection
		// Special case: singleton portal children without their own ID
		if change.ResourceID == "" &&
			change.ResourceType != planner.ResourceTypePortalCustomization &&
			change.ResourceType != planner.ResourceTypePortalAuthSettings &&
			change.ResourceType != planner.ResourceTypePortalIntegration &&
			change.ResourceType != planner.ResourceTypePortalAssetLogo &&
			change.ResourceType != planner.ResourceTypePortalAssetFavicon &&
			change.ResourceType != planner.ResourceTypePortalTeamGroupMapping {
			return fmt.Errorf("resource ID required for %s operation", change.Action)
		}

		if err := e.validateInheritedProtection(ctx, change); err != nil {
			return err
		}

		// Skip validation for updates/deletes with ResourceID - planner already verified existence
		// and actual operations handle protection checks
		if change.ResourceID != "" {
			return nil
		}

		// Perform resource-specific validation for updates/deletes without ResourceID
		// (This is mainly for portal_customization which is a singleton)
		switch change.ResourceType {
		case planner.ResourceTypePortal:
			if e.client != nil {
				portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
				if err != nil {
					return fmt.Errorf("failed to fetch portal: %w", err)
				}
				if portal == nil {
					return fmt.Errorf("portal no longer exists")
				}

				// Check protection status using common utility
				isProtected := common.GetProtectionStatus(portal.NormalizedLabels)
				isProtectionChange := common.IsProtectionChange(change.Protection)

				// Validate protection using common utility
				resourceName := common.ExtractResourceName(change.Fields)
				if err := common.ValidateResourceProtection(
					"portal", resourceName, isProtected, change, isProtectionChange,
				); err != nil {
					return err
				}
			}
		case planner.FieldAPI:
			if e.client != nil {
				api, err := e.client.GetAPIByName(ctx, getResourceName(change.Fields))
				if err != nil {
					return fmt.Errorf("failed to fetch API: %w", err)
				}
				if api == nil {
					return fmt.Errorf("API no longer exists")
				}

				// Check protection status
				isProtected := api.NormalizedLabels[labels.ProtectedKey] == "true"

				// For updates, check if this is a protection change (which is allowed)
				isProtectionChange := false
				if change.Action == planner.ActionUpdate {
					// Check if this is a protection change
					switch p := change.Protection.(type) {
					case planner.ProtectionChange:
						isProtectionChange = true
					case map[string]any:
						// From JSON deserialization
						if _, hasOld := p["old"].(bool); hasOld {
							if _, hasNew := p["new"].(bool); hasNew {
								isProtectionChange = true
							}
						}
					}
				}

				// Block protected resources unless it's a protection change
				if isProtected && !isProtectionChange &&
					(change.Action == planner.ActionUpdate || change.Action == planner.ActionDelete) {
					return fmt.Errorf("resource is protected and cannot be %s",
						actionToVerb(change.Action))
				}
			}
		}

	}

	return nil
}

// resolveAuthStrategyRef resolves an auth strategy reference to its ID
func (e *Executor) resolveAuthStrategyRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if id, ok := e.getRefAny(planner.ResourceTypeApplicationAuthStrategy, lookupRef, refInfo.Ref); ok {
		return id, nil
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Otherwise, look it up from the API
	strategy, err := e.client.GetAuthStrategyByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get auth strategy by name: %w", err)
	}
	if strategy == nil {
		return "", fmt.Errorf("auth strategy not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	return strategy.ID, nil
}

func (e *Executor) resolveDCRProviderRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	if id, ok := e.getRefAny(planner.ResourceTypeDCRProvider, lookupRef, refInfo.Ref); ok {
		return id, nil
	}

	provider, err := e.client.GetDCRProviderByName(ctx, lookupRef)
	if err != nil {
		return "", fmt.Errorf("failed to get dcr provider by name: %w", err)
	}
	if provider == nil {
		return "", fmt.Errorf("dcr provider not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupRef)
	}

	return provider.ID, nil
}

func unresolvedReferenceID(id string) bool {
	return id == "" || id == resources.UnknownReferenceID
}

func (e *Executor) syncResolvedRef(
	ctx context.Context,
	change *planner.PlannedChange,
	fieldName string,
	resolver func(context.Context, planner.ReferenceInfo) (string, error),
	errMsg string,
) error {
	if change == nil {
		return nil
	}

	ref, ok := change.References[fieldName]
	if !ok {
		return nil
	}

	if unresolvedReferenceID(ref.ID) {
		id, err := resolver(ctx, ref)
		if err != nil {
			return fmt.Errorf("%s: %w", errMsg, err)
		}
		ref.ID = id
		change.References[fieldName] = ref
	}

	if change.Fields != nil && !setResolvedFieldValue(change.Fields, fieldName, ref.ID) {
		change.Fields[fieldName] = ref.ID
	}

	return nil
}

func (e *Executor) syncResolvedEventGatewayProducePolicyConfigRefs(
	ctx context.Context,
	change *planner.PlannedChange,
) error {
	if err := e.syncResolvedEventGatewaySchemaRegistryConfigRef(ctx, change); err != nil {
		return err
	}
	if err := e.syncResolvedEventGatewayStaticKeyConfigRef(ctx, change); err != nil {
		return err
	}
	return nil
}

func (e *Executor) syncResolvedEventGatewaySchemaRegistryConfigRef(
	ctx context.Context,
	change *planner.PlannedChange,
) error {
	const fieldName = planner.FieldConfig + ".schema_registry." + planner.FieldID
	ref, ok := change.References[fieldName]
	if !ok {
		return nil
	}

	if unresolvedReferenceID(ref.ID) {
		if e.client == nil {
			return fmt.Errorf("state client not configured")
		}
		gatewayID, err := eventGatewayIDFromChange(change)
		if err != nil {
			return err
		}
		name := referenceLookupName(ref)
		registry, err := e.client.GetEventGatewaySchemaRegistryByName(ctx, gatewayID, name)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway schema registry reference: %w", err)
		}
		if registry == nil {
			return fmt.Errorf("event gateway schema registry not found: ref=%s", ref.Ref)
		}
		ref.ID = registry.ID
		change.References[fieldName] = ref
	}

	if change.Fields != nil {
		setResolvedFieldValue(change.Fields, fieldName, ref.ID)
	}
	return nil
}

func (e *Executor) syncResolvedEventGatewayStaticKeyConfigRef(
	ctx context.Context,
	change *planner.PlannedChange,
) error {
	const fieldName = planner.FieldConfig + ".encryption_key.key." + planner.FieldID
	var keys []state.EventGatewayStaticKey
	var keysLoaded bool

	for currentFieldName, ref := range change.References {
		if currentFieldName != fieldName && !isEventGatewayEncryptFieldsStaticKeyRef(currentFieldName) {
			continue
		}

		if unresolvedReferenceID(ref.ID) {
			if e.client == nil {
				return fmt.Errorf("state client not configured")
			}
			gatewayID, err := eventGatewayIDFromChange(change)
			if err != nil {
				return err
			}
			name := referenceLookupName(ref)
			if !keysLoaded {
				var err error
				keys, err = e.client.ListEventGatewayStaticKeys(ctx, gatewayID)
				if err != nil {
					return fmt.Errorf("failed to resolve event gateway static key reference: %w", err)
				}
				keysLoaded = true
			}
			for _, key := range keys {
				if key.Name == name {
					ref.ID = key.ID
					change.References[currentFieldName] = ref
					break
				}
			}
			if unresolvedReferenceID(ref.ID) {
				return fmt.Errorf("event gateway static key not found: ref=%s", ref.Ref)
			}
		}

		if change.Fields != nil {
			setResolvedFieldValue(change.Fields, currentFieldName, ref.ID)
		}
	}
	return nil
}

func isEventGatewayEncryptFieldsStaticKeyRef(fieldName string) bool {
	return strings.HasPrefix(fieldName, planner.FieldConfig+".encrypt_fields.") &&
		strings.HasSuffix(fieldName, ".encryption_key.key."+planner.FieldID)
}

func eventGatewayIDFromChange(change *planner.PlannedChange) (string, error) {
	if change == nil {
		return "", fmt.Errorf("event gateway reference is required")
	}
	ref, ok := change.References[planner.FieldEventGatewayID]
	if !ok || unresolvedReferenceID(ref.ID) {
		return "", fmt.Errorf("event gateway reference is required")
	}
	return ref.ID, nil
}

func referenceLookupName(ref planner.ReferenceInfo) string {
	if ref.LookupFields != nil {
		if name := strings.TrimSpace(ref.LookupFields[planner.FieldName]); name != "" {
			return name
		}
	}
	return normalizedRefValue(ref.Ref)
}

func (e *Executor) syncResolvedDCRProviderID(
	ctx context.Context,
	change *planner.PlannedChange,
) error {
	return e.syncResolvedRef(
		ctx,
		change,
		planner.FieldDCRProviderID,
		e.resolveDCRProviderRef,
		"failed to resolve DCR provider reference",
	)
}

func (e *Executor) syncResolvedPortalDefaultAuthStrategyID(
	ctx context.Context,
	change *planner.PlannedChange,
) error {
	return e.syncResolvedRef(
		ctx,
		change,
		planner.FieldDefaultApplicationStrategyID,
		e.resolveAuthStrategyRef,
		"failed to resolve auth strategy reference",
	)
}

func (e *Executor) syncResolvedAIGatewayID(
	ctx context.Context,
	change *planner.PlannedChange,
) error {
	return e.syncResolvedRef(
		ctx,
		change,
		planner.FieldAIGatewayID,
		e.resolveAIGatewayRef,
		"failed to resolve AI Gateway reference",
	)
}

// resolvePortalRef resolves a portal reference to its ID
func (e *Executor) resolvePortalRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	// First check if the reference already has a resolved ID
	if !unresolvedReferenceID(refInfo.ID) {
		return refInfo.ID, nil
	}

	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// Check if it was created in this execution
	if id, ok := e.getRef(planner.ResourceTypePortal, lookupRef); ok {
		return id, nil
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Otherwise, look it up from the API
	portal, err := e.client.GetPortalByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get portal by name: %w", err)
	}
	if portal == nil {
		return "", fmt.Errorf("portal not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	// Cache the resolved ID
	e.setRef(planner.ResourceTypePortal, lookupRef, portal.ID)

	return portal.ID, nil
}

func (e *Executor) resolvePortalTeamRef(
	ctx context.Context,
	portalID string,
	refInfo planner.ReferenceInfo,
) (string, error) {
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required to resolve portal team")
	}

	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	if id, ok := e.getRef(planner.ResourceTypePortalTeam, lookupRef); ok && id != "" {
		return id, nil
	}

	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	portalTeams, err := e.client.ListPortalTeams(ctx, portalID)
	if err != nil {
		return "", fmt.Errorf("failed to list portal teams: %w", err)
	}

	for _, team := range portalTeams {
		if team.Name == lookupValue {
			return team.ID, nil
		}
	}

	return "", fmt.Errorf("portal team not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
}

func (e *Executor) resolveOrganizationTeamRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	if refInfo.ID != "" {
		return refInfo.ID, nil
	}

	if teams, ok := e.refToID[planner.ResourceTypeOrganizationTeam]; ok {
		if id, found := teams[refInfo.Ref]; found && id != "" {
			return id, nil
		}
	}

	lookupValue := refInfo.Ref
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	team, err := e.client.GetOrganizationTeamByNameUnfiltered(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get organization team by name: %w", err)
	}
	if team == nil {
		return "", fmt.Errorf("organization team not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	if team.ID == nil {
		return "", fmt.Errorf("organization team %s has no ID", lookupValue)
	}

	return *team.ID, nil
}

func (e *Executor) resolveControlPlaneRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	if id, ok := e.getRef(planner.ResourceTypeControlPlane, lookupRef); ok &&
		id != "" &&
		id != resources.UnknownReferenceID {
		return id, nil
	}

	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, ok := refInfo.LookupFields[planner.FieldName]; ok && name != "" {
			lookupValue = name
		}
	}

	cp, err := e.client.GetControlPlaneByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get control plane by name: %w", err)
	}
	if cp == nil {
		return "", fmt.Errorf("control plane not found: ref=%s, lookup=%s", refInfo.Ref, lookupValue)
	}

	return cp.ID, nil
}

func (e *Executor) resolveRoleEntityRef(ctx context.Context, change *planner.PlannedChange) error {
	if change == nil {
		return nil
	}

	entityRef, ok := change.References[planner.FieldEntityID]
	if !ok || !unresolvedReferenceID(entityRef.ID) {
		return nil
	}

	entityTypeName, _ := change.Fields[planner.FieldEntityTypeName].(string)
	entityResourceType, ok := resources.RoleEntityResourceType(entityTypeName)
	if !ok {
		return fmt.Errorf("failed to resolve entity reference: unsupported entity_type_name %q", entityTypeName)
	}

	var (
		entityID string
		err      error
	)
	switch entityResourceType { //nolint:exhaustive
	case resources.ResourceTypeAPI:
		entityID, err = e.resolveAPIRef(ctx, entityRef)
	case resources.ResourceTypePortal:
		entityID, err = e.resolvePortalRef(ctx, entityRef)
	case resources.ResourceTypeControlPlane:
		entityID, err = e.resolveControlPlaneRef(ctx, entityRef)
	default:
		return fmt.Errorf(
			"failed to resolve entity reference: unsupported entity resource type %s",
			entityResourceType,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to resolve entity reference: %w", err)
	}

	entityRef.ID = entityID
	change.References[planner.FieldEntityID] = entityRef
	return nil
}

func (e *Executor) syncControlPlaneGroupMembers(
	ctx context.Context,
	change *planner.PlannedChange,
	controlPlaneID string,
) error {
	field, ok := change.Fields[planner.FieldMembers]
	if !ok {
		return nil
	}

	desiredIDs, err := extractMemberIDsFromField(field)
	if err != nil {
		return fmt.Errorf("failed to extract control plane group members: %w", err)
	}
	if desiredIDs == nil {
		return nil
	}

	resolved := make([]string, len(desiredIDs))
	copy(resolved, desiredIDs)

	refInfo, hasRefs := change.References[planner.FieldMembers]
	for idx, id := range desiredIDs {
		if !tags.IsRefPlaceholder(id) {
			continue
		}

		if !hasRefs || !refInfo.IsArray {
			return fmt.Errorf("missing reference information for control plane group member %q", id)
		}

		resolvedID, err := e.resolveMemberReference(ctx, id, refInfo, idx)
		if err != nil {
			return err
		}
		resolved[idx] = resolvedID
	}

	for _, id := range resolved {
		if tags.IsRefPlaceholder(id) {
			return fmt.Errorf("unable to resolve control plane group member reference %q", id)
		}
	}

	normalized := normalizers.NormalizeMemberIDs(resolved)
	if e.dryRun {
		return nil
	}

	return e.client.UpsertControlPlaneGroupMemberships(ctx, controlPlaneID, normalized)
}

func (e *Executor) detachControlPlaneGroupMembers(ctx context.Context, change *planner.PlannedChange) error {
	field, ok := change.Fields[planner.FieldMembers]
	if !ok {
		return nil
	}

	memberIDs, err := extractMemberIDsFromField(field)
	if err != nil {
		return fmt.Errorf("failed to extract control plane group members: %w", err)
	}

	normalized := normalizers.NormalizeMemberIDs(memberIDs)
	if len(normalized) == 0 {
		return nil
	}
	if e.dryRun {
		return nil
	}

	return e.client.RemoveControlPlaneGroupMemberships(ctx, change.ResourceID, normalized)
}

func (e *Executor) resolveMemberReference(
	ctx context.Context,
	placeholder string,
	refInfo planner.ReferenceInfo,
	index int,
) (string, error) {
	targetIndex := -1
	if index < len(refInfo.Refs) && refInfo.Refs[index] == placeholder {
		targetIndex = index
	} else {
		for i, ref := range refInfo.Refs {
			if ref == placeholder {
				targetIndex = i
				break
			}
		}
	}

	if targetIndex == -1 {
		return "", fmt.Errorf("control plane membership reference %q not found", placeholder)
	}

	lookupFields := buildLookupFieldsForIndex(refInfo, targetIndex)
	lookupRef := planner.ReferenceInfo{
		Ref:          refInfo.Refs[targetIndex],
		LookupFields: lookupFields,
	}

	return e.resolveControlPlaneRef(ctx, lookupRef)
}

func buildLookupFieldsForIndex(refInfo planner.ReferenceInfo, index int) map[string]string {
	if refInfo.LookupArrays == nil {
		return nil
	}

	fields := make(map[string]string)
	if names, ok := refInfo.LookupArrays["names"]; ok {
		if index < len(names) && names[index] != "" {
			fields[planner.FieldName] = names[index]
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func extractMemberIDsFromField(field any) ([]string, error) {
	switch v := field.(type) {
	case []map[string]string:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			ids = append(ids, item[planner.FieldID])
		}
		return ids, nil
	case []map[string]any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			id, ok := item[planner.FieldID].(string)
			if !ok {
				return nil, fmt.Errorf("control plane member entry missing id")
			}
			ids = append(ids, id)
		}
		return ids, nil
	case []any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			switch entry := item.(type) {
			case map[string]string:
				ids = append(ids, entry[planner.FieldID])
			case map[string]any:
				id, ok := entry[planner.FieldID].(string)
				if !ok {
					return nil, fmt.Errorf("control plane member entry missing id")
				}
				ids = append(ids, id)
			default:
				return nil, fmt.Errorf("unsupported control plane member entry type %T", item)
			}
		}
		return ids, nil
	default:
		return nil, nil
	}
}

// resolveAPIRef resolves an API reference to its ID
func (e *Executor) resolveAPIRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if id, ok := e.getRef(planner.FieldAPI, lookupRef); ok {
		slog.Debug(
			"Resolved API reference from created resources",
			"api_ref", lookupRef,
			"api_id", id,
		)
		return id, nil
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the API in Konnect with retry for eventual consistency
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		api, err := e.client.GetAPIByName(ctx, lookupValue)
		if err == nil && api != nil {
			apiID := api.ID
			slog.Debug(
				"Resolved API reference from Konnect",
				"api_ref", refInfo.Ref,
				"lookup_value", lookupValue,
				"api_id", apiID,
				"attempt", attempt+1,
			)

			// Cache this resolution
			e.setRef(planner.FieldAPI, refInfo.Ref, apiID)
			return apiID, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("failed to resolve API reference %s (lookup: %s) after 3 attempts: %w",
		refInfo.Ref, lookupValue, lastErr)
}

// resolveEventGatewayRef resolves an event gateway reference to its ID
func (e *Executor) resolveEventGatewayRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if id, ok := e.getRef(planner.ResourceTypeEventGatewayControlPlane, lookupRef); ok {
		slog.Debug(
			"Resolved event gateway reference from created resources",
			"gateway_ref", lookupRef,
			"gateway_id", id,
		)
		return id, nil
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the event gateway in Konnect
	gateway, err := e.client.GetEventGatewayControlPlaneByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get event gateway by name: %w", err)
	}
	if gateway == nil {
		return "", fmt.Errorf("event gateway not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	gatewayID := gateway.ID
	slog.Debug(
		"Resolved event gateway reference from Konnect",
		"gateway_ref", refInfo.Ref,
		"lookup_value", lookupValue,
		"gateway_id", gatewayID,
	)

	// Cache this resolution
	e.setRef(planner.ResourceTypeEventGatewayControlPlane, refInfo.Ref, gatewayID)

	return gatewayID, nil
}

// resolveAIGatewayRef resolves an AI Gateway reference to its ID.
func (e *Executor) resolveAIGatewayRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	if !unresolvedReferenceID(refInfo.ID) {
		return refInfo.ID, nil
	}

	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	if id, ok := e.getRefAny(planner.ResourceTypeAIGateway, lookupRef, refInfo.Ref); ok {
		return id, nil
	}

	lookupValue := lookupRef
	lookupByName := false
	if refInfo.LookupFields != nil {
		if name := strings.TrimSpace(refInfo.LookupFields[planner.FieldName]); name != "" {
			lookupValue = name
			lookupByName = true
		} else if displayName := strings.TrimSpace(refInfo.LookupFields[planner.FieldDisplayName]); displayName != "" {
			lookupValue = displayName
		}
	}

	var gateway *state.AIGateway
	var err error
	if lookupByName {
		gateway, err = e.client.GetAIGatewayByName(ctx, lookupValue)
	} else {
		gateway, err = e.client.GetAIGatewayByDisplayName(ctx, lookupValue)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get AI Gateway: %w", err)
	}
	if gateway == nil {
		lookupField := planner.FieldDisplayName
		if lookupByName {
			lookupField = planner.FieldName
		}
		return "", fmt.Errorf("AI Gateway not found: ref=%s, looked up by %s=%s", refInfo.Ref, lookupField, lookupValue)
	}

	e.setRef(planner.ResourceTypeAIGateway, lookupRef, gateway.ID)
	if lookupRef != refInfo.Ref {
		e.setRef(planner.ResourceTypeAIGateway, refInfo.Ref, gateway.ID)
	}
	return gateway.ID, nil
}

// resolveEventGatewayBackendClusterRef resolves an event gateway reference to its ID
func (e *Executor) resolveEventGatewayBackendClusterRef(
	ctx context.Context, gatewayID string, refInfo planner.ReferenceInfo,
) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if id, ok := e.getRef(planner.ResourceTypeEventGatewayBackendCluster, lookupRef); ok {
		slog.Debug(
			"Resolved event gateway backend cluster reference from created resources",
			"backend_cluster_ref", lookupRef,
			"backend_cluster_id", id,
		)
		return id, nil
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the event gateway backend cluster in Konnect
	backendCluster, err := e.client.GetEventGatewayBackendClusterByName(ctx, gatewayID, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get event gateway backend cluster by name: %w", err)
	}
	if backendCluster == nil {
		return "", fmt.Errorf("event gateway backend cluster not found: ref=%s, looked up by name=%s",
			refInfo.Ref, lookupValue)
	}

	backendClusterID := backendCluster.ID
	slog.Debug(
		"Resolved event gateway backend cluster reference from Konnect",
		"backend_cluster_ref", refInfo.Ref,
		"lookup_value", lookupValue,
		"gateway_id", backendClusterID,
	)

	// Cache this resolution
	e.setRef(planner.ResourceTypeEventGatewayBackendCluster, refInfo.Ref, backendClusterID)

	return backendClusterID, nil
}

// resolveEventGatewayVirtualClusterRef resolves an event gateway virtual cluster reference to its ID.
// This requires a gateway ID to search within.
func (e *Executor) resolveEventGatewayVirtualClusterRef(
	ctx context.Context, gatewayID string, refInfo planner.ReferenceInfo,
) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if id, ok := e.getRef(planner.ResourceTypeEventGatewayVirtualCluster, lookupRef); ok {
		slog.Debug(
			"Resolved event gateway virtual cluster reference from created resources",
			"virtual_cluster_ref", lookupRef,
			"virtual_cluster_id", id,
		)
		return id, nil
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the event gateway virtual cluster in Konnect
	virtualCluster, err := e.client.GetEventGatewayVirtualClusterByName(ctx, gatewayID, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get event gateway virtual cluster by name: %w", err)
	}
	if virtualCluster == nil {
		return "", fmt.Errorf("event gateway virtual cluster not found: ref=%s, looked up by name=%s",
			refInfo.Ref, lookupValue)
	}

	virtualClusterID := virtualCluster.ID
	slog.Debug(
		"Resolved event gateway virtual cluster reference from Konnect",
		"virtual_cluster_ref", refInfo.Ref,
		"lookup_value", lookupValue,
		"virtual_cluster_id", virtualClusterID,
	)

	// Cache this resolution
	e.setRef(planner.ResourceTypeEventGatewayVirtualCluster, refInfo.Ref, virtualClusterID)

	return virtualClusterID, nil
}

// resolveEventGatewayListenerRef resolves an event gateway listener reference to its ID.
// This requires a gateway ID to search within, which must be available in the change references.
func (e *Executor) resolveEventGatewayListenerRef(
	ctx context.Context, change *planner.PlannedChange, refInfo planner.ReferenceInfo,
) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if id, ok := e.getRef(planner.ResourceTypeEventGatewayListener, lookupRef); ok {
		slog.Debug(
			"Resolved event gateway listener reference from created resources",
			"listener_ref", lookupRef,
			"listener_id", id,
		)
		return id, nil
	}

	// Need the gateway ID to look up listeners
	var gatewayID string
	if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID != "" {
		gatewayID = gatewayRef.ID
	}
	if gatewayID == "" && change.Parent != nil && change.Parent.ID != "" {
		gatewayID = change.Parent.ID
	}
	if gatewayID == "" {
		return "", fmt.Errorf("cannot resolve listener ref without gateway ID")
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields[planner.FieldName]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the listener in Konnect by listing and matching by name
	listeners, err := e.client.ListEventGatewayListeners(ctx, gatewayID)
	if err != nil {
		return "", fmt.Errorf("failed to list event gateway listeners: %w", err)
	}

	for _, listener := range listeners {
		if listener.Name == lookupValue {
			slog.Debug(
				"Resolved event gateway listener reference from Konnect",
				"listener_ref", refInfo.Ref,
				"lookup_value", lookupValue,
				"listener_id", listener.ID,
			)

			// Cache this resolution
			e.setRef(planner.ResourceTypeEventGatewayListener, refInfo.Ref, listener.ID)

			return listener.ID, nil
		}
	}

	return "", fmt.Errorf(
		"event gateway listener not found: ref=%s, looked up by name=%s in gateway=%s",
		refInfo.Ref, lookupValue, gatewayID,
	)
}

// populatePortalPages fetches and caches all pages for a portal
func (e *Executor) populatePortalPages(ctx context.Context, portalID string) error {
	e.cacheMu.Lock()
	if _, exists := e.stateCache.Portals[portalID]; !exists {
		e.stateCache.Portals[portalID] = &state.CachedPortal{
			Pages: make(map[string]*state.CachedPortalPage),
		}
	}
	e.cacheMu.Unlock()

	// Fetch all pages
	pages, err := e.client.ListManagedPortalPages(ctx, portalID)
	if err != nil {
		return fmt.Errorf("failed to list portal pages: %w", err)
	}

	// First pass: create all pages
	pageMap := make(map[string]*state.CachedPortalPage)
	for _, page := range pages {
		cachedPage := &state.CachedPortalPage{
			PortalPage: page,
			Children:   make(map[string]*state.CachedPortalPage),
		}
		pageMap[page.ID] = cachedPage
	}

	// Second pass: establish parent-child relationships
	rootPages := make(map[string]*state.CachedPortalPage)
	for _, page := range pages {
		cachedPage := pageMap[page.ID]

		if page.ParentPageID == "" {
			// Root page
			rootPages[page.ID] = cachedPage
		} else if parent, ok := pageMap[page.ParentPageID]; ok {
			// Child page
			parent.Children[page.ID] = cachedPage
		}
	}

	e.cacheMu.Lock()
	if portal, exists := e.stateCache.Portals[portalID]; exists {
		portal.Pages = rootPages
	}
	e.cacheMu.Unlock()

	return nil
}

// populateAPIDocuments fetches and caches all documents for an API
func (e *Executor) populateAPIDocuments(ctx context.Context, apiID string) error {
	if apiID == "" {
		return fmt.Errorf("API ID is required to populate documents")
	}

	e.cacheMu.Lock()
	cachedAPI, exists := e.stateCache.APIs[apiID]
	if !exists {
		cachedAPI = &state.CachedAPI{
			Documents:       make(map[string]*state.CachedAPIDocument),
			Versions:        make(map[string]*state.APIVersion),
			Publications:    make(map[string]*state.APIPublication),
			Implementations: make(map[string]*state.APIImplementation),
		}
		e.stateCache.APIs[apiID] = cachedAPI
	}

	if cachedAPI.Documents == nil {
		cachedAPI.Documents = make(map[string]*state.CachedAPIDocument)
	}

	if len(cachedAPI.Documents) > 0 {
		e.cacheMu.Unlock()
		return nil
	}
	e.cacheMu.Unlock()

	documents, err := e.client.ListAPIDocuments(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list API documents: %w", err)
	}

	docMap := make(map[string]*state.CachedAPIDocument)
	for _, doc := range documents {
		cachedDoc := &state.CachedAPIDocument{
			APIDocument: doc,
			Children:    make(map[string]*state.CachedAPIDocument),
		}
		docMap[doc.ID] = cachedDoc
	}

	rootDocs := make(map[string]*state.CachedAPIDocument)
	for _, cachedDoc := range docMap {
		if cachedDoc.ParentDocumentID == "" {
			rootDocs[cachedDoc.ID] = cachedDoc
			continue
		}

		parent, ok := docMap[cachedDoc.ParentDocumentID]
		if !ok {
			rootDocs[cachedDoc.ID] = cachedDoc
			continue
		}

		if parent.Children == nil {
			parent.Children = make(map[string]*state.CachedAPIDocument)
		}
		parent.Children[cachedDoc.ID] = cachedDoc
	}

	e.cacheMu.Lock()
	if cachedAPI, exists := e.stateCache.APIs[apiID]; exists {
		cachedAPI.Documents = rootDocs
	}
	e.cacheMu.Unlock()

	return nil
}

// resolvePortalPageRef resolves a portal page reference to its ID
func (e *Executor) resolvePortalPageRef(
	ctx context.Context, portalID string, pageRef string, lookupFields map[string]string,
) (string, error) {
	// First check if it was created in this execution
	if id, ok := e.getRef(planner.ResourceTypePortalPage, pageRef); ok {
		return id, nil
	}

	// Ensure portal pages are cached
	e.cacheMu.RLock()
	portal, exists := e.stateCache.Portals[portalID]
	needsPopulate := !exists || portal.Pages == nil
	e.cacheMu.RUnlock()

	if needsPopulate {
		if err := e.populatePortalPages(ctx, portalID); err != nil {
			return "", err
		}
	}

	e.cacheMu.RLock()
	portal = e.stateCache.Portals[portalID]
	if portal == nil {
		e.cacheMu.RUnlock()
		return "", fmt.Errorf("portal %s not found in cache", portalID)
	}
	defer e.cacheMu.RUnlock()

	// If we have a parent path, use it for more accurate matching
	if lookupFields != nil && lookupFields[planner.FieldParentPath] != "" {
		targetPath := lookupFields[planner.FieldParentPath]

		if page := portal.FindPageBySlugPath(targetPath); page != nil {
			return page.ID, nil
		}
	}

	// Fallback: search all pages for matching slug
	var searchPages func(pages map[string]*state.CachedPortalPage) string
	searchPages = func(pages map[string]*state.CachedPortalPage) string {
		for _, page := range pages {
			normalizedSlug := strings.TrimPrefix(page.Slug, "/")
			if normalizedSlug == pageRef {
				return page.ID
			}
			// Search children
			if childID := searchPages(page.Children); childID != "" {
				return childID
			}
		}
		return ""
	}

	if pageID := searchPages(portal.Pages); pageID != "" {
		return pageID, nil
	}

	return "", fmt.Errorf("portal page not found: ref=%s in portal=%s", pageRef, portalID)
}

// resolveAPIDocumentRef resolves an API document reference to its ID
func (e *Executor) resolveAPIDocumentRef(
	ctx context.Context, apiID string, refInfo planner.ReferenceInfo,
) (string, error) {
	if refInfo.HasResolvedID() {
		return refInfo.ID, nil
	}

	actualRef := refInfo.Ref
	if strings.HasPrefix(actualRef, tags.RefPlaceholderPrefix) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(actualRef); ok {
			actualRef = parsedRef
		}
	}

	if id, ok := e.getRef(planner.ResourceTypeAPIDocument, actualRef); ok {
		return id, nil
	}

	if apiID == "" {
		return "", fmt.Errorf("API ID is required to resolve document reference")
	}

	if err := e.populateAPIDocuments(ctx, apiID); err != nil {
		return "", err
	}

	e.cacheMu.RLock()
	cachedAPI, ok := e.stateCache.APIs[apiID]
	if !ok {
		e.cacheMu.RUnlock()
		return "", fmt.Errorf("API %s not found in cache", apiID)
	}
	defer e.cacheMu.RUnlock()

	if refInfo.LookupFields != nil {
		if path, ok := refInfo.LookupFields[planner.FieldSlugPath]; ok && path != "" {
			if doc := findCachedAPIDocumentByPath(cachedAPI.Documents, path); doc != nil {
				return doc.ID, nil
			}
		}
		if slug, ok := refInfo.LookupFields[planner.FieldSlug]; ok && slug != "" {
			if doc := findCachedAPIDocumentByPath(cachedAPI.Documents, slug); doc != nil {
				return doc.ID, nil
			}
		}
	}

	return "", fmt.Errorf("failed to resolve API document reference %q", actualRef)
}

func findCachedAPIDocumentByPath(
	documents map[string]*state.CachedAPIDocument, path string,
) *state.CachedAPIDocument {
	cleanPath := strings.Trim(path, "/")
	if cleanPath == "" {
		return nil
	}

	segments := strings.Split(cleanPath, "/")
	for _, doc := range documents {
		if found := traverseCachedAPIDocument(doc, segments); found != nil {
			return found
		}
	}

	return nil
}

func traverseCachedAPIDocument(
	doc *state.CachedAPIDocument, segments []string,
) *state.CachedAPIDocument {
	if doc == nil || len(segments) == 0 {
		return nil
	}

	slug := strings.Trim(strings.TrimPrefix(doc.Slug, "/"), "/")
	if slug != segments[0] {
		return nil
	}

	if len(segments) == 1 {
		return doc
	}

	for _, child := range doc.Children {
		if found := traverseCachedAPIDocument(child, segments[1:]); found != nil {
			return found
		}
	}

	return nil
}

// Resource operations

func (e *Executor) createResource(ctx context.Context, change *planner.PlannedChange) (string, error) {
	// Note: ExecutionContext is now passed explicitly to executors instead of using context.WithValue

	switch change.ResourceType {
	case planner.ResourceTypePortal:
		if err := e.syncResolvedPortalDefaultAuthStrategyID(ctx, change); err != nil {
			return "", err
		}
		return e.portalExecutor.Create(ctx, *change)
	case planner.ResourceTypeControlPlane:
		id, err := e.controlPlaneExecutor.Create(ctx, *change)
		if err != nil {
			return "", err
		}
		if err := e.syncControlPlaneGroupMembers(ctx, change, id); err != nil {
			return "", err
		}
		return id, nil
	case planner.ResourceTypeControlPlaneDataPlaneCertificate:
		if controlPlaneRef, ok := change.References[planner.FieldControlPlaneID]; ok && controlPlaneRef.ID == "" {
			controlPlaneID, err := e.resolveControlPlaneRef(ctx, controlPlaneRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve control plane reference: %w", err)
			}
			controlPlaneRef.ID = controlPlaneID
			change.References[planner.FieldControlPlaneID] = controlPlaneRef
		}
		return e.controlPlaneDataPlaneCertificateExecutor.Create(ctx, *change)
	case planner.FieldAPI:
		// No references to resolve for api
		return e.apiExecutor.Create(ctx, *change)
	case planner.ResourceTypeCatalogService:
		return e.catalogServiceExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGateway:
		return e.aiGatewayExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayProvider:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayProviderExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayPolicy:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayPolicyExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayAgent:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayAgentExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumer:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayConsumerExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumerCredential:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayConsumerCredentialExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumerGroup:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayConsumerGroupExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayModel:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayModelExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayMCPServer:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayMCPServerExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayVault:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayVaultExecutor.Create(ctx, *change)
	case planner.ResourceTypeAIGatewayDataPlaneCertificate:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayDataPlaneCertificateExecutor.Create(ctx, *change)
	case planner.ResourceTypeDashboard:
		return e.dashboardExecutor.Create(ctx, *change)
	case planner.ResourceTypeDCRProvider:
		return e.dcrProviderExecutor.Create(ctx, *change)
	case planner.ResourceTypeAPIVersion:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		return e.apiVersionExecutor.Create(ctx, *change)
	case planner.ResourceTypeAPIPublication:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		// Also resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		// Resolve auth_strategy_ids array references if needed
		if authStrategyRefs, ok := change.References[planner.FieldAuthStrategyIDs]; ok && authStrategyRefs.IsArray {
			resolvedIDs := make([]string, 0, len(authStrategyRefs.Refs))

			for i, ref := range authStrategyRefs.Refs {
				var resolvedID string
				var err error

				// Check if already resolved
				if authStrategyRefs.ResolvedIDs != nil && i < len(authStrategyRefs.ResolvedIDs) &&
					authStrategyRefs.ResolvedIDs[i] != "" {
					resolvedID = authStrategyRefs.ResolvedIDs[i]
				} else {
					// Construct ReferenceInfo for the auth strategy
					refInfo := planner.ReferenceInfo{
						Ref: ref,
					}
					// Add lookup fields if available
					if names, ok := authStrategyRefs.LookupArrays["names"]; ok && i < len(names) {
						refInfo.LookupFields = map[string]string{
							planner.FieldName: names[i],
						}
					}

					resolvedID, err = e.resolveAuthStrategyRef(ctx, refInfo)
					if err != nil {
						return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", ref, err)
					}
				}

				if resolvedID == "" {
					return "", fmt.Errorf("failed to resolve auth strategy reference %q", ref)
				}

				resolvedIDs = append(resolvedIDs, resolvedID)
			}

			// Update the reference with resolved IDs
			authStrategyRefs.ResolvedIDs = resolvedIDs
			change.References[planner.FieldAuthStrategyIDs] = authStrategyRefs
		}
		return e.apiPublicationExecutor.Create(ctx, *change)
	case planner.ResourceTypeAPIImplementation:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		return e.apiImplementationExecutor.Create(ctx, *change)
	case planner.ResourceTypeAPIDocument:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		if parentRef, ok := change.References[planner.FieldParentDocumentID]; ok &&
			parentRef.Ref != "" && parentRef.ID == "" {
			apiID := ""
			if apiInfo, exists := change.References[planner.FieldAPIID]; exists {
				apiID = apiInfo.ID
			}
			if apiID == "" && change.Parent != nil {
				apiID = change.Parent.ID
			}
			resolvedParentID, err := e.resolveAPIDocumentRef(ctx, apiID, parentRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent document reference: %w", err)
			}
			parentRef.ID = resolvedParentID
			change.References[planner.FieldParentDocumentID] = parentRef
		}
		return e.apiDocumentExecutor.Create(ctx, *change)
	case planner.ResourceTypeApplicationAuthStrategy:
		if err := e.syncResolvedDCRProviderID(ctx, change); err != nil {
			return "", err
		}
		return e.authStrategyExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalCustomization:
		// Portal customization is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalCustomizationExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalAuthSettings:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalAuthSettingsExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalIPAllowList:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalIPAllowListExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalIntegration:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalIntegrationExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalIdentityProvider:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalIdentityProviderExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalAssetLogo:
		// Portal asset logo is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalAssetLogoExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalAssetFavicon:
		// Portal asset favicon is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalAssetFaviconExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalCustomDomain:
		// Resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalDomainExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalPage:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		// Handle parent page reference resolution if needed
		if parentPageRef, ok := change.References[planner.FieldParentPageID]; ok && parentPageRef.ID == "" {
			portalID := change.References[planner.FieldPortalID].ID
			parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent page reference: %w", err)
			}
			// Create a new reference with the resolved ID
			parentPageRef.ID = parentPageID
			change.References[planner.FieldParentPageID] = parentPageRef
		}
		return e.portalPageExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalSnippet:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalSnippetExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalTeam:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalTeamExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalTeamRole:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}

		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			portalID := ""
			if portalInfo, exists := change.References[planner.FieldPortalID]; exists {
				portalID = portalInfo.ID
			}
			if portalID == "" && change.Parent != nil {
				portalID = change.Parent.ID
			}

			teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
		}

		if err := e.resolveRoleEntityRef(ctx, change); err != nil {
			return "", err
		}

		return e.portalTeamRoleExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalEmailConfig:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalEmailConfigExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalAuditLogWebhook:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalAuditLogWebhookExecutor.Create(ctx, *change)
	case planner.ResourceTypePortalEmailTemplate:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalEmailTemplateExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayControlPlane:
		return e.eventGatewayControlPlaneExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayBackendCluster:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			// Update the reference with the resolved ID
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayBackendClusterExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayVirtualCluster:
		// Resolve event gateway reference if needed.
		// When the gateway was already created at plan time, its ID is in change.Parent.ID.
		// When the gateway was being created in the same plan run, change.Parent is nil and
		// the ID is stored in change.References["event_gateway_id"] after resolution below.
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok &&
			unresolvedReferenceID(gatewayRef.ID) {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			// Update the reference with the resolved ID
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}

		// Determine the effective gateway ID for backend cluster resolution.
		// Prefer the resolved reference over change.Parent (which is nil when the gateway
		// was not yet created at plan time).
		effectiveGatewayID := ""
		if change.Parent != nil {
			effectiveGatewayID = change.Parent.ID
		}
		if ref, ok := change.References[planner.FieldEventGatewayID]; ok && ref.ID != "" {
			effectiveGatewayID = ref.ID
		}

		// Resolve event gateway backend cluster reference if needed
		if backendClusterRef, ok := change.References[planner.FieldEventGatewayBackendClusterID]; ok &&
			unresolvedReferenceID(backendClusterRef.ID) {
			backendClusterID, err := e.resolveEventGatewayBackendClusterRef(ctx, effectiveGatewayID, backendClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway backend cluster reference: %w", err)
			}
			// Update the reference with the resolved ID
			backendClusterRef.ID = backendClusterID
			change.References[planner.FieldEventGatewayBackendClusterID] = backendClusterRef
		}
		return e.eventGatewayVirtualClusterExecutor.Create(ctx, *change)
	case planner.ResourceTypeOrganizationTeam:
		return e.organizationTeamExecutor.Create(ctx, *change)
	case planner.ResourceTypeOrganizationTeamRole:
		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			teamID, err := e.resolveOrganizationTeamRef(ctx, teamRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve organization team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
		}
		if err := e.resolveRoleEntityRef(ctx, change); err != nil {
			return "", err
		}
		return e.organizationTeamRoleExecutor.Create(ctx, *change)
	case planner.ResourceTypeOrganizationUserTeamMembership:
		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			teamID, err := e.resolveOrganizationTeamRef(ctx, teamRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve organization team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
		}
		return e.organizationUserTeamMembershipExecutor.Create(ctx, *change)
	case planner.ResourceTypeOrganizationUserRole:
		if err := e.resolveRoleEntityRef(ctx, change); err != nil {
			return "", err
		}
		return e.organizationUserRoleExecutor.Create(ctx, *change)
	case planner.ResourceTypeOrganizationSystemAccountTeamMembership:
		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			teamID, err := e.resolveOrganizationTeamRef(ctx, teamRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve organization team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
		}
		return e.organizationSystemAccountTeamMembershipExecutor.Create(ctx, *change)
	case planner.ResourceTypeOrganizationSystemAccountRole:
		if err := e.resolveRoleEntityRef(ctx, change); err != nil {
			return "", err
		}
		return e.organizationSystemAccountRoleExecutor.Create(ctx, *change)

	case planner.ResourceTypeEventGatewayListener:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			// Update the reference with the resolved ID
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayListenerExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayListenerPolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway listener reference if needed
		if listenerRef, ok := change.References[planner.FieldEventGatewayListenerID]; ok && listenerRef.ID == "" {
			listenerID, err := e.resolveEventGatewayListenerRef(ctx, change, listenerRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway listener reference: %w", err)
			}
			listenerRef.ID = listenerID
			change.References[planner.FieldEventGatewayListenerID] = listenerRef
		}
		// Resolve event gateway virtual cluster reference if needed (for forward_to_virtual_cluster policies)
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			unresolvedReferenceID(virtualClusterRef.ID) {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		return e.eventGatewayListenerPolicyExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayDataPlaneCertificate:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayDataPlaneCertificateExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewaySchemaRegistry:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewaySchemaRegistryExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayStaticKey:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayStaticKeyExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayTLSTrustBundle:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayTLSTrustBundleExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayClusterPolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway virtual cluster reference if needed
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			virtualClusterRef.ID == "" {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		return e.eventGatewayClusterPolicyExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayProducePolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway virtual cluster reference if needed
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			virtualClusterRef.ID == "" {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		if err := e.syncResolvedEventGatewayProducePolicyConfigRefs(ctx, change); err != nil {
			return "", err
		}
		return e.eventGatewayProducePolicyExecutor.Create(ctx, *change)
	case planner.ResourceTypeEventGatewayConsumePolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway virtual cluster reference if needed
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			virtualClusterRef.ID == "" {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		return e.eventGatewayConsumePolicyExecutor.Create(ctx, *change)
	default:
		return "", fmt.Errorf("create operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) updateResource(ctx context.Context, change *planner.PlannedChange) (string, error) {
	// Note: ExecutionContext is now passed explicitly to executors instead of using context.WithValue

	switch change.ResourceType {
	case planner.ResourceTypePortal:
		if err := e.syncResolvedPortalDefaultAuthStrategyID(ctx, change); err != nil {
			return "", err
		}
		return e.portalExecutor.Update(ctx, *change)
	case planner.ResourceTypeControlPlane:
		id, err := e.controlPlaneExecutor.Update(ctx, *change)
		if err != nil {
			return "", err
		}
		if err := e.syncControlPlaneGroupMembers(ctx, change, id); err != nil {
			return "", err
		}
		return id, nil
	case planner.FieldAPI:
		return e.apiExecutor.Update(ctx, *change)
	case planner.ResourceTypeCatalogService:
		return e.catalogServiceExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGateway:
		return e.aiGatewayExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayProvider:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayProviderExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayPolicy:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayPolicyExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayAgent:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayAgentExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumer:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayConsumerExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumerGroup:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayConsumerGroupExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayModel:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayModelExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayMCPServer:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayMCPServerExecutor.Update(ctx, *change)
	case planner.ResourceTypeAIGatewayVault:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return "", err
		}
		return e.aiGatewayVaultExecutor.Update(ctx, *change)
	case planner.ResourceTypeDashboard:
		return e.dashboardExecutor.Update(ctx, *change)
	case planner.ResourceTypeAPIDocument:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		if parentRef, ok := change.References[planner.FieldParentDocumentID]; ok &&
			parentRef.Ref != "" && parentRef.ID == "" {
			apiID := ""
			if apiInfo, exists := change.References[planner.FieldAPIID]; exists {
				apiID = apiInfo.ID
			}
			if apiID == "" && change.Parent != nil {
				apiID = change.Parent.ID
			}
			resolvedParentID, err := e.resolveAPIDocumentRef(ctx, apiID, parentRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent document reference: %w", err)
			}
			parentRef.ID = resolvedParentID
			change.References[planner.FieldParentDocumentID] = parentRef
		}
		return e.apiDocumentExecutor.Update(ctx, *change)
	case planner.ResourceTypeAPIPublication:
		// API publications use PUT for both create and update
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		// Also resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		// Resolve auth strategy references if present
		if authStrategyRefs, ok := change.References[planner.FieldAuthStrategyIDs]; ok && authStrategyRefs.IsArray {
			resolvedIDs := make([]string, 0, len(authStrategyRefs.Refs))
			for _, ref := range authStrategyRefs.Refs {
				strategyRef := planner.ReferenceInfo{
					Ref:          ref,
					LookupFields: make(map[string]string),
				}
				// Copy lookup fields if available
				if authStrategyRefs.LookupArrays != nil && len(authStrategyRefs.LookupArrays["names"]) > 0 {
					// Find corresponding name for this ref
					for i, r := range authStrategyRefs.Refs {
						if r == ref && i < len(authStrategyRefs.LookupArrays["names"]) {
							strategyRef.LookupFields[planner.FieldName] = authStrategyRefs.LookupArrays["names"][i]
							break
						}
					}
				}
				resolvedID, err := e.resolveAuthStrategyRef(ctx, strategyRef)
				if err != nil {
					return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", ref, err)
				}
				resolvedIDs = append(resolvedIDs, resolvedID)
			}
			// Update the reference with resolved IDs
			authStrategyRefs.ResolvedIDs = resolvedIDs
			change.References[planner.FieldAuthStrategyIDs] = authStrategyRefs
		}
		// Use Create method which handles PUT (both create and update)
		return e.apiPublicationExecutor.Create(ctx, *change)
	case planner.ResourceTypeApplicationAuthStrategy:
		if err := e.syncResolvedDCRProviderID(ctx, change); err != nil {
			return "", err
		}
		return e.authStrategyExecutor.Update(ctx, *change)
	case planner.ResourceTypeDCRProvider:
		return e.dcrProviderExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalCustomization:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalCustomizationExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalAuthSettings:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalAuthSettingsExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalIPAllowList:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalIPAllowListExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalIntegration:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalIntegrationExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalIdentityProvider:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalIdentityProviderExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalEmailConfig:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalEmailConfigExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalAuditLogWebhook:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalAuditLogWebhookExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalEmailTemplate:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalEmailTemplateExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalAssetLogo:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalAssetLogoExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalAssetFavicon:
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return e.portalAssetFaviconExecutor.Update(ctx, *change, portalID)
	case planner.ResourceTypePortalCustomDomain:
		return e.portalDomainExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalPage:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		// Handle parent page reference resolution if needed
		if parentPageRef, ok := change.References[planner.FieldParentPageID]; ok && parentPageRef.ID == "" {
			portalID := change.References[planner.FieldPortalID].ID
			parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent page reference: %w", err)
			}
			// Create a new reference with the resolved ID
			parentPageRef.ID = parentPageID
			change.References[planner.FieldParentPageID] = parentPageRef
		}
		return e.portalPageExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalSnippet:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalSnippetExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalTeam:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalTeamExecutor.Update(ctx, *change)
	case planner.ResourceTypePortalTeamGroupMapping:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			portalID := ""
			if portalInfo, exists := change.References[planner.FieldPortalID]; exists {
				portalID = portalInfo.ID
			}
			if portalID == "" && change.Parent != nil {
				portalID = change.Parent.ID
			}
			teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
			change.Fields[planner.FieldTeamID] = teamID
			change.ResourceID = teamID
		}
		return e.portalTeamGroupMappingExecutor.Update(ctx, *change)
	case planner.ResourceTypeAPIVersion:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		return e.apiVersionExecutor.Update(ctx, *change)
	// Note: api_publication and api_implementation don't support update
	case planner.ResourceTypeEventGatewayControlPlane:
		return e.eventGatewayControlPlaneExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayBackendCluster:
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayBackendClusterExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayVirtualCluster:
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway backend cluster reference if needed
		if backendClusterRef, ok := change.References[planner.FieldEventGatewayBackendClusterID]; ok &&
			backendClusterRef.ID == "" {
			backendClusterID, err := e.resolveEventGatewayBackendClusterRef(ctx, change.Parent.ID, backendClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway backend cluster reference: %w", err)
			}
			backendClusterRef.ID = backendClusterID
			change.References[planner.FieldEventGatewayBackendClusterID] = backendClusterRef
		}
		return e.eventGatewayVirtualClusterExecutor.Update(ctx, *change)
	case planner.ResourceTypeOrganizationTeam:
		return e.organizationTeamExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayListener:
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayListenerExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayListenerPolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway listener reference if needed
		if listenerRef, ok := change.References[planner.FieldEventGatewayListenerID]; ok && listenerRef.ID == "" {
			listenerID, err := e.resolveEventGatewayListenerRef(ctx, change, listenerRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway listener reference: %w", err)
			}
			listenerRef.ID = listenerID
			change.References[planner.FieldEventGatewayListenerID] = listenerRef
		}
		// Resolve event gateway virtual cluster reference if needed (for forward_to_virtual_cluster policies)
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			unresolvedReferenceID(virtualClusterRef.ID) {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		return e.eventGatewayListenerPolicyExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayDataPlaneCertificate:
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayDataPlaneCertificateExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewaySchemaRegistry:
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewaySchemaRegistryExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayTLSTrustBundle:
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		return e.eventGatewayTLSTrustBundleExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayClusterPolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway virtual cluster reference if needed
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			virtualClusterRef.ID == "" {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		return e.eventGatewayClusterPolicyExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayProducePolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway virtual cluster reference if needed
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			virtualClusterRef.ID == "" {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		if err := e.syncResolvedEventGatewayProducePolicyConfigRefs(ctx, change); err != nil {
			return "", err
		}
		return e.eventGatewayProducePolicyExecutor.Update(ctx, *change)
	case planner.ResourceTypeEventGatewayConsumePolicy:
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References[planner.FieldEventGatewayID] = gatewayRef
		}
		// Resolve event gateway virtual cluster reference if needed
		if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
			virtualClusterRef.ID == "" {
			gatewayID := change.References[planner.FieldEventGatewayID].ID
			virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
			}
			virtualClusterRef.ID = virtualClusterID
			change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
		}
		return e.eventGatewayConsumePolicyExecutor.Update(ctx, *change)
	default:
		return "", fmt.Errorf("update operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) deleteResource(ctx context.Context, change *planner.PlannedChange) error {
	// Note: ExecutionContext is now passed explicitly to executors instead of using context.WithValue

	switch change.ResourceType {
	case planner.ResourceTypePortal:
		// No references to resolve for portal
		return e.portalExecutor.Delete(ctx, *change)
	case planner.ResourceTypeControlPlane:
		if err := e.detachControlPlaneGroupMembers(ctx, change); err != nil {
			return fmt.Errorf("failed to detach control plane group members: %w", err)
		}
		return e.controlPlaneExecutor.Delete(ctx, *change)
	case planner.ResourceTypeControlPlaneDataPlaneCertificate:
		return e.controlPlaneDataPlaneCertificateExecutor.Delete(ctx, *change)
	case planner.FieldAPI:
		// No references to resolve for api
		return e.apiExecutor.Delete(ctx, *change)
	case planner.ResourceTypeCatalogService:
		return e.catalogServiceExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGateway:
		return e.aiGatewayExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayProvider:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayProviderExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayPolicy:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayPolicyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayAgent:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayAgentExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumer:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayConsumerExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumerCredential:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayConsumerCredentialExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayConsumerGroup:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayConsumerGroupExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayModel:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayModelExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayMCPServer:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayMCPServerExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayVault:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayVaultExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAIGatewayDataPlaneCertificate:
		if err := e.syncResolvedAIGatewayID(ctx, change); err != nil {
			return err
		}
		return e.aiGatewayDataPlaneCertificateExecutor.Delete(ctx, *change)
	case planner.ResourceTypeDashboard:
		return e.dashboardExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAPIVersion:
		// No references to resolve for api_version delete
		return e.apiVersionExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAPIPublication:
		// No references to resolve for api_publication delete
		return e.apiPublicationExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAPIImplementation:
		// No references to resolve for api_implementation delete
		return e.apiImplementationExecutor.Delete(ctx, *change)
	case planner.ResourceTypeAPIDocument:
		// First resolve API reference if needed
		if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References[planner.FieldAPIID] = apiRef
		}
		return e.apiDocumentExecutor.Delete(ctx, *change)
	case planner.ResourceTypeApplicationAuthStrategy:
		return e.authStrategyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeDCRProvider:
		return e.dcrProviderExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalCustomDomain:
		// No references to resolve for portal_custom_domain
		return e.portalDomainExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalIPAllowList:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalIPAllowListExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalIdentityProvider:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalIdentityProviderExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalPage:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalPageExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalSnippet:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalSnippetExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalTeam:
		// First resolve portal reference if needed
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalTeamExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalTeamRole:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			portalID := ""
			if portalInfo, exists := change.References[planner.FieldPortalID]; exists {
				portalID = portalInfo.ID
			}
			if portalID == "" && change.Parent != nil {
				portalID = change.Parent.ID
			}
			teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
		}
		return e.portalTeamRoleExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalEmailConfig:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalEmailConfigExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalAuditLogWebhook:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalAuditLogWebhookExecutor.Delete(ctx, *change)
	case planner.ResourceTypePortalEmailTemplate:
		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References[planner.FieldPortalID] = portalRef
		}
		return e.portalEmailTemplateExecutor.Delete(ctx, *change)
	// Note: portal_customization is a singleton resource and cannot be deleted
	case planner.ResourceTypeEventGatewayControlPlane:
		return e.eventGatewayControlPlaneExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayBackendCluster:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayBackendClusterExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayVirtualCluster:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayVirtualClusterExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayListener:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayListenerExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayListenerPolicy:
		// Both gateway ID and listener ID should be in References for delete
		return e.eventGatewayListenerPolicyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayDataPlaneCertificate:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayDataPlaneCertificateExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewaySchemaRegistry:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewaySchemaRegistryExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayStaticKey:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayStaticKeyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayTLSTrustBundle:
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayTLSTrustBundleExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayClusterPolicy:
		// Both gateway ID and virtual cluster ID should be in References for delete
		return e.eventGatewayClusterPolicyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayProducePolicy:
		// Both gateway ID and virtual cluster ID should be in References for delete
		return e.eventGatewayProducePolicyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeEventGatewayConsumePolicy:
		// Both gateway ID and virtual cluster ID should be in References for delete
		return e.eventGatewayConsumePolicyExecutor.Delete(ctx, *change)
	case planner.ResourceTypeOrganizationTeam:
		return e.organizationTeamExecutor.Delete(ctx, *change)
	case planner.ResourceTypeOrganizationTeamRole:
		if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
			teamID, err := e.resolveOrganizationTeamRef(ctx, teamRef)
			if err != nil {
				return fmt.Errorf("failed to resolve organization team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References[planner.FieldTeamID] = teamRef
		}
		return e.organizationTeamRoleExecutor.Delete(ctx, *change)
	case planner.ResourceTypeOrganizationUserTeamMembership:
		return e.organizationUserTeamMembershipExecutor.Delete(ctx, *change)
	case planner.ResourceTypeOrganizationUserRole:
		return e.organizationUserRoleExecutor.Delete(ctx, *change)
	case planner.ResourceTypeOrganizationSystemAccountTeamMembership:
		return e.organizationSystemAccountTeamMembershipExecutor.Delete(ctx, *change)
	case planner.ResourceTypeOrganizationSystemAccountRole:
		return e.organizationSystemAccountRoleExecutor.Delete(ctx, *change)
	default:
		return fmt.Errorf("delete operation not yet implemented for %s", change.ResourceType)
	}
}

// Helper functions

// getResourceName is deprecated, use common.ExtractResourceName instead
// Kept for backward compatibility with existing code
func getResourceName(fields map[string]any) string {
	return common.ExtractResourceName(fields)
}

// actionToVerb is deprecated, use common utilities instead
// Kept for backward compatibility with existing code
func actionToVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "created"
	case planner.ActionUpdate:
		return "updated"
	case planner.ActionDelete:
		return "deleted"
	case planner.ActionExternalTool:
		return "executed"
	default:
		return string(action)
	}
}

// getParentAPIID resolves the parent API ID for child resources
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) getParentAPIID(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Add debug logging
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug(
		"getParentAPIID called",
		slog.String("change_id", change.ID),
		slog.String("resource_type", change.ResourceType),
		slog.String("resource_ref", change.ResourceRef),
		slog.Any("parent", change.Parent),
	)

	if change.Parent == nil {
		return "", fmt.Errorf("parent API reference required")
	}

	// Log parent details
	logger.Debug(
		"Parent details",
		slog.String("parent_ref", change.Parent.Ref),
		slog.String("parent_id", change.Parent.ID),
		slog.Bool("parent_id_empty", change.Parent.ID == ""),
		slog.Int("parent_id_length", len(change.Parent.ID)),
	)

	// Use the parent ID if it was already resolved
	if change.Parent.ID != "" {
		logger.Debug("Using resolved parent ID", slog.String("parent_id", change.Parent.ID))
		return change.Parent.ID, nil
	}

	// Check if parent was created in this execution
	logger.Debug("Checking dependencies", slog.Int("dep_count", len(change.DependsOn)))
	e.mu.Lock()
	var parentFromCreated string
	for _, dep := range change.DependsOn {
		if resourceID, ok := e.createdResources[dep]; ok {
			parentFromCreated = resourceID
			break
		}
	}
	e.mu.Unlock()
	if parentFromCreated != "" {
		logger.Debug(
			"Found parent in created resources",
			slog.String("resource_id", parentFromCreated),
		)
		return parentFromCreated, nil
	}

	// Otherwise look up by name
	logger.Debug("Falling back to API lookup by name", slog.String("api_ref", change.Parent.Ref))
	parentAPI, err := e.client.GetAPIByName(ctx, change.Parent.Ref)
	if err != nil {
		return "", fmt.Errorf("failed to get parent API: %w", err)
	}
	if parentAPI == nil {
		return "", fmt.Errorf("parent API not found: %s", change.Parent.Ref)
	}

	logger.Debug(
		"Found parent API by name",
		slog.String("api_name", parentAPI.Name),
		slog.String("api_id", parentAPI.ID),
	)

	return parentAPI.ID, nil
}
