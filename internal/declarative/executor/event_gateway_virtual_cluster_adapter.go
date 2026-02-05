package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

// EventGatewayVirtualClusterAdapter implements ResourceOperations for Event Gateway Virtual Cluster resources
type EventGatewayVirtualClusterAdapter struct {
	client *state.Client
}

// NewEventGatewayVirtualClusterAdapter creates a new EventGatewayVirtualClusterAdapter
func NewEventGatewayVirtualClusterAdapter(client *state.Client) *EventGatewayVirtualClusterAdapter {
	return &EventGatewayVirtualClusterAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to CreateVirtualClusterRequest
func (a *EventGatewayVirtualClusterAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateVirtualClusterRequest,
) error {
	// Required fields
	name, ok := fields["name"].(string)
	if !ok {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	// Destination (required)
	destField, ok := fields["destination"]
	if !ok {
		return fmt.Errorf("destination is required")
	}
	destination, err := buildBackendClusterReference(destField, execCtx)
	if err != nil {
		return fmt.Errorf("failed to build destination: %w", err)
	}
	create.Destination = destination

	// Authentication (required)
	authField, ok := fields["authentication"]
	if !ok {
		return fmt.Errorf("authentication is required")
	}
	authentication, err := buildVirtualClusterAuthentication(authField)
	if err != nil {
		return fmt.Errorf("failed to build authentication: %w", err)
	}
	create.Authentication = authentication

	// ACL Mode (required)
	aclModeField, ok := fields["acl_mode"]
	if !ok {
		return fmt.Errorf("acl_mode is required")
	}
	aclMode, err := buildACLMode(aclModeField)
	if err != nil {
		return fmt.Errorf("failed to build acl_mode: %w", err)
	}
	create.ACLMode = aclMode

	// DNS Label (required)
	dnsLabel, ok := fields["dns_label"].(string)
	if !ok {
		return fmt.Errorf("dns_label is required")
	}
	create.DNSLabel = dnsLabel

	// Optional fields
	if desc, ok := fields["description"].(string); ok {
		create.Description = &desc
	}

	if nsField, ok := fields["namespace"]; ok {
		namespace, err := buildVirtualClusterNamespace(nsField)
		if err != nil {
			return fmt.Errorf("failed to build namespace: %w", err)
		}
		create.Namespace = namespace
	}

	if labelsMap := extractLabelsField(fields, "labels"); labelsMap != nil {
		create.Labels = labelsMap
	}

	return nil
}

// MapUpdateFields maps the fields to update into an UpdateVirtualClusterRequest
func (a *EventGatewayVirtualClusterAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.UpdateVirtualClusterRequest,
	_ map[string]string,
) error {
	// Required fields - always sent even if not changed
	if name, ok := fieldsToUpdate["name"].(string); ok {
		update.Name = name
	}
	if destField, ok := fieldsToUpdate["destination"]; ok {
		destination, err := buildBackendClusterReference(destField, execCtx)
		if err != nil {
			return fmt.Errorf("failed to build destination: %w", err)
		}
		update.Destination = destination
	}
	if aclModeField, ok := fieldsToUpdate["acl_mode"]; ok {
		aclMode, err := buildACLMode(aclModeField)
		if err != nil {
			return fmt.Errorf("failed to build acl_mode: %w", err)
		}
		update.ACLMode = aclMode
	}
	if dnsLabel, ok := fieldsToUpdate["dns_label"].(string); ok {
		update.DNSLabel = dnsLabel
	}

	// Authentication requires conversion from Scheme to SensitiveDataAwareScheme
	if authField, ok := fieldsToUpdate["authentication"]; ok {
		authentication, err := buildVirtualClusterAuthentication(authField)
		if err != nil {
			return fmt.Errorf("failed to build authentication: %w", err)
		}
		// Convert to SensitiveDataAwareScheme
		sensitiveAuth := make([]kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme, len(authentication))
		for i, auth := range authentication {
			converted, err := convertToVirtualClusterSensitiveDataAwareAuth(auth)
			if err != nil {
				return fmt.Errorf("failed to convert authentication[%d]: %w", i, err)
			}
			sensitiveAuth[i] = converted
		}
		update.Authentication = sensitiveAuth
	}

	// Optional fields
	if description, ok := fieldsToUpdate["description"]; ok {
		if desc, ok := description.(string); ok {
			update.Description = &desc
		} else if description == nil {
			// Handle nil description (clear it)
			emptyStr := ""
			update.Description = &emptyStr
		}
	}

	if nsField, ok := fieldsToUpdate["namespace"]; ok {
		namespace, err := buildVirtualClusterNamespace(nsField)
		if err != nil {
			return fmt.Errorf("failed to build namespace: %w", err)
		}
		update.Namespace = namespace
	}

	if labels, ok := fieldsToUpdate["labels"].(map[string]string); ok {
		update.Labels = labels
	}

	return nil
}

// Create creates a new virtual cluster
func (a *EventGatewayVirtualClusterAdapter) Create(
	ctx context.Context,
	req kkComps.CreateVirtualClusterRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayVirtualCluster(ctx, gatewayID, req, namespace)
}

// Update updates an existing virtual cluster
func (a *EventGatewayVirtualClusterAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateVirtualClusterRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayVirtualCluster(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a virtual cluster
func (a *EventGatewayVirtualClusterAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayVirtualCluster(ctx, gatewayID, id)
}

// GetByID gets a virtual cluster by ID
func (a *EventGatewayVirtualClusterAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	cluster, err := a.client.GetEventGatewayVirtualCluster(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, nil
	}

	return &EventGatewayVirtualClusterResourceInfo{virtualCluster: cluster}, nil
}

// GetByName is not supported for virtual clusters (they are looked up by name within a gateway)
func (a *EventGatewayVirtualClusterAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway virtual clusters")
}

// ResourceType returns the resource type string
func (a *EventGatewayVirtualClusterAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayVirtualCluster
}

// RequiredFields returns the list of required fields for this resource
func (a *EventGatewayVirtualClusterAdapter) RequiredFields() []string {
	return []string{"name", "destination", "authentication", "acl_mode", "dns_label"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayVirtualClusterAdapter) SupportsUpdate() bool {
	return true
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context
func (a *EventGatewayVirtualClusterAdapter) getEventGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	// Priority 1: Check References (for new parent)
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		return gatewayRef.ID, nil
	}

	// Priority 2: Check Parent field (for existing parent)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("event gateway ID required for virtual cluster operations")
}

// EventGatewayVirtualClusterResourceInfo wraps an Event Gateway Virtual Cluster to implement ResourceInfo
type EventGatewayVirtualClusterResourceInfo struct {
	virtualCluster *state.EventGatewayVirtualCluster
}

func (e *EventGatewayVirtualClusterResourceInfo) GetID() string {
	return e.virtualCluster.ID
}

func (e *EventGatewayVirtualClusterResourceInfo) GetName() string {
	return e.virtualCluster.Name
}

func (e *EventGatewayVirtualClusterResourceInfo) GetLabels() map[string]string {
	return e.virtualCluster.Labels
}

func (e *EventGatewayVirtualClusterResourceInfo) GetNormalizedLabels() map[string]string {
	return e.virtualCluster.NormalizedLabels
}

// buildBackendClusterReference constructs BackendClusterReferenceModify from a map or SDK type
func buildBackendClusterReference(field any, execCtx *ExecutionContext) (kkComps.BackendClusterReferenceModify, error) {
	// If it's already the SDK type, return it directly
	if bcRef, ok := field.(kkComps.BackendClusterReferenceModify); ok {
		if bcRef.BackendClusterReferenceByID != nil && util.IsValidUUID(bcRef.BackendClusterReferenceByID.ID) {
			return bcRef, nil
		}

		idFromReference, err := getBackendClusterIDFromExecutionContext(execCtx)
		if err != nil {
			return kkComps.BackendClusterReferenceModify{},
				fmt.Errorf("failed to get backend cluster ID from execution context: %w", err)
		}

		// Use the ID from execution context if not provided in the field
		bcRef.BackendClusterReferenceByID = &kkComps.BackendClusterReferenceByID{ID: idFromReference}
		return bcRef, nil
	}

	// Otherwise, build from map structure
	refMap, ok := field.(map[string]any)
	if !ok {
		return kkComps.BackendClusterReferenceModify{},
			fmt.Errorf("destination must be an object, got %T", field)
	}

	// Check for id or name
	if id, ok := refMap["id"].(string); ok {
		if util.IsValidUUID(id) {
			return kkComps.BackendClusterReferenceModify{
				Type: kkComps.BackendClusterReferenceModifyTypeBackendClusterReferenceByID,
				BackendClusterReferenceByID: &kkComps.BackendClusterReferenceByID{
					ID: id,
				},
			}, nil
		}

		idFromReference, err := getBackendClusterIDFromExecutionContext(execCtx)
		if err != nil {
			return kkComps.BackendClusterReferenceModify{},
				fmt.Errorf("failed to get backend cluster ID from execution context: %w", err)
		}

		return kkComps.BackendClusterReferenceModify{
			Type: kkComps.BackendClusterReferenceModifyTypeBackendClusterReferenceByID,
			BackendClusterReferenceByID: &kkComps.BackendClusterReferenceByID{
				ID: idFromReference,
			},
		}, nil
	}

	if name, ok := refMap["name"].(string); ok {
		return kkComps.BackendClusterReferenceModify{
			Type: kkComps.BackendClusterReferenceModifyTypeBackendClusterReferenceByName,
			BackendClusterReferenceByName: &kkComps.BackendClusterReferenceByName{
				Name: name,
			},
		}, nil
	}

	return kkComps.BackendClusterReferenceModify{},
		fmt.Errorf("destination must have either 'id' or 'name' field")
}

// getBackendClusterIDFromExecutionContext extracts the backend cluster ID from ExecutionContext parameter
func getBackendClusterIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for virtual cluster operations")
	}

	change := *execCtx.PlannedChange
	if backendClusterRef, ok := change.References["event_gateway_backend_cluster_id"]; ok && backendClusterRef.ID != "" {
		return backendClusterRef.ID, nil
	}
	// Check fields as fallback
	if backendClusterID, ok := change.Fields["event_gateway_backend_cluster_id"].(string); ok {
		return backendClusterID, nil
	}

	return "", fmt.Errorf("backend cluster ID is required for virtual cluster operations")
}

// buildVirtualClusterAuthentication constructs VirtualClusterAuthenticationScheme slice from a slice or SDK type
func buildVirtualClusterAuthentication(field any) ([]kkComps.VirtualClusterAuthenticationScheme, error) {
	// If it's already the SDK type, return it directly
	if auth, ok := field.([]kkComps.VirtualClusterAuthenticationScheme); ok {
		return auth, nil
	}

	// Otherwise, build from slice of maps
	authSlice, ok := field.([]any)
	if !ok {
		return nil, fmt.Errorf("authentication must be an array, got %T", field)
	}

	result := make([]kkComps.VirtualClusterAuthenticationScheme, 0, len(authSlice))
	for i, authItem := range authSlice {
		authMap, ok := authItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("authentication[%d] must be an object, got %T", i, authItem)
		}

		authType, ok := authMap["type"].(string)
		if !ok {
			return nil, fmt.Errorf("authentication[%d].type is required and must be a string", i)
		}

		switch authType {
		case "anonymous":
			result = append(result, kkComps.CreateVirtualClusterAuthenticationSchemeAnonymous(
				kkComps.VirtualClusterAuthenticationAnonymous{},
			))

		case "sasl_plain":
			mediation, ok := authMap["mediation"].(string)
			if !ok {
				return nil, fmt.Errorf("authentication[%d].mediation is required for sasl_plain", i)
			}

			saslPlain := kkComps.VirtualClusterAuthenticationSaslPlain{
				Mediation: kkComps.VirtualClusterAuthenticationSaslPlainMediation(mediation),
			}

			// Parse principals if provided (required for "terminate" mediation)
			if principalsField, ok := authMap["principals"]; ok {
				principalsSlice, ok := principalsField.([]any)
				if !ok {
					return nil, fmt.Errorf("authentication[%d].principals must be an array", i)
				}

				principals := make([]kkComps.VirtualClusterAuthenticationPrincipal, 0, len(principalsSlice))
				for j, principalItem := range principalsSlice {
					principalMap, ok := principalItem.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("authentication[%d].principals[%d] must be an object", i, j)
					}

					username, ok := principalMap["username"].(string)
					if !ok {
						return nil, fmt.Errorf("authentication[%d].principals[%d].username is required", i, j)
					}

					password, ok := principalMap["password"].(string)
					if !ok {
						return nil, fmt.Errorf("authentication[%d].principals[%d].password is required", i, j)
					}

					principals = append(principals, kkComps.VirtualClusterAuthenticationPrincipal{
						Username: username,
						Password: password,
					})
				}
				saslPlain.Principals = principals
			}

			result = append(result, kkComps.CreateVirtualClusterAuthenticationSchemeSaslPlain(saslPlain))

		case "sasl_scram":
			algorithm, ok := authMap["algorithm"].(string)
			if !ok {
				return nil, fmt.Errorf("authentication[%d].algorithm is required for sasl_scram", i)
			}
			result = append(result, kkComps.CreateVirtualClusterAuthenticationSchemeSaslScram(
				kkComps.VirtualClusterAuthenticationSaslScram{
					Algorithm: kkComps.VirtualClusterAuthenticationSaslScramAlgorithm(algorithm),
				},
			))

		case "oauth_bearer":
			mediation, ok := authMap["mediation"].(string)
			if !ok {
				return nil, fmt.Errorf("authentication[%d].mediation is required for oauth_bearer", i)
			}

			oauthBearer := kkComps.VirtualClusterAuthenticationOauthBearer{
				Mediation: kkComps.VirtualClusterAuthenticationOauthBearerMediation(mediation),
			}

			// Parse optional claims_mapping
			if claimsMappingField, ok := authMap["claims_mapping"]; ok {
				claimsMappingMap, ok := claimsMappingField.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("authentication[%d].claims_mapping must be an object", i)
				}
				claimsMapping := &kkComps.VirtualClusterAuthenticationClaimsMapping{}
				if sub, ok := claimsMappingMap["sub"].(string); ok {
					claimsMapping.Sub = &sub
				}
				if scope, ok := claimsMappingMap["scope"].(string); ok {
					claimsMapping.Scope = &scope
				}
				oauthBearer.ClaimsMapping = claimsMapping
			}

			// Parse optional jwks
			if jwksField, ok := authMap["jwks"]; ok {
				jwksMap, ok := jwksField.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("authentication[%d].jwks must be an object", i)
				}
				endpoint, ok := jwksMap["endpoint"].(string)
				if !ok {
					return nil, fmt.Errorf("authentication[%d].jwks.endpoint is required", i)
				}
				jwks := &kkComps.VirtualClusterAuthenticationJWKS{
					Endpoint: endpoint,
				}
				if timeout, ok := jwksMap["timeout"].(string); ok {
					jwks.Timeout = &timeout
				}
				if cacheExpiration, ok := jwksMap["cache_expiration"].(string); ok {
					jwks.CacheExpiration = &cacheExpiration
				}
				oauthBearer.Jwks = jwks
			}

			// Parse optional validate
			if validateField, ok := authMap["validate"]; ok {
				validateMap, ok := validateField.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("authentication[%d].validate must be an object", i)
				}
				validate := &kkComps.VirtualClusterAuthenticationValidate{}
				if audiencesField, ok := validateMap["audiences"]; ok {
					audiencesSlice, ok := audiencesField.([]any)
					if !ok {
						return nil, fmt.Errorf("authentication[%d].validate.audiences must be an array", i)
					}
					audiences := make([]kkComps.VirtualClusterAuthenticationAudience, 0, len(audiencesSlice))
					for j, audienceItem := range audiencesSlice {
						audienceMap, ok := audienceItem.(map[string]any)
						if !ok {
							return nil, fmt.Errorf("authentication[%d].validate.audiences[%d] must be an object", i, j)
						}
						name, ok := audienceMap["name"].(string)
						if !ok {
							return nil, fmt.Errorf("authentication[%d].validate.audiences[%d].name is required", i, j)
						}
						audiences = append(audiences, kkComps.VirtualClusterAuthenticationAudience{
							Name: name,
						})
					}
					validate.Audiences = audiences
				}
				if issuer, ok := validateMap["issuer"].(string); ok {
					validate.Issuer = &issuer
				}
				oauthBearer.Validate = validate
			}

			result = append(result, kkComps.CreateVirtualClusterAuthenticationSchemeOauthBearer(oauthBearer))
		default:
			return nil, fmt.Errorf("unsupported authentication type: %s", authType)
		}
	}

	return result, nil
}

// buildACLMode constructs VirtualClusterACLMode from a string or SDK type
func buildACLMode(field any) (kkComps.VirtualClusterACLMode, error) {
	// If it's already the SDK type, return it directly
	if mode, ok := field.(kkComps.VirtualClusterACLMode); ok {
		return mode, nil
	}

	// Otherwise, convert from string
	modeStr, ok := field.(string)
	if !ok {
		return "", fmt.Errorf("acl_mode must be a string, got %T", field)
	}

	switch modeStr {
	case "enforce_on_gateway":
		return kkComps.VirtualClusterACLModeEnforceOnGateway, nil
	case "passthrough":
		return kkComps.VirtualClusterACLModePassthrough, nil
	default:
		return "", fmt.Errorf("invalid acl_mode: %s (must be 'enforce_on_gateway' or 'passthrough')", modeStr)
	}
}

// buildVirtualClusterNamespace constructs VirtualClusterNamespace from a map or SDK type
func buildVirtualClusterNamespace(field any) (*kkComps.VirtualClusterNamespace, error) {
	// If it's already the SDK type, return it directly
	if ns, ok := field.(*kkComps.VirtualClusterNamespace); ok {
		return ns, nil
	}

	// Otherwise, build from map structure
	nsMap, ok := field.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("namespace must be an object, got %T", field)
	}

	mode, ok := nsMap["mode"].(string)
	if !ok {
		return nil, fmt.Errorf("namespace.mode is required and must be a string")
	}

	namespace := &kkComps.VirtualClusterNamespace{
		Mode: kkComps.Mode(mode),
	}

	if prefix, ok := nsMap["prefix"].(string); ok {
		namespace.Prefix = prefix
	}

	return namespace, nil
}

// convertToVirtualClusterSensitiveDataAwareAuth converts VirtualClusterAuthenticationScheme
// to VirtualClusterAuthenticationSensitiveDataAwareScheme for update operations
func convertToVirtualClusterSensitiveDataAwareAuth(
	auth kkComps.VirtualClusterAuthenticationScheme,
) (kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme, error) {
	switch auth.Type {
	case kkComps.VirtualClusterAuthenticationSchemeTypeAnonymous:
		return kkComps.CreateVirtualClusterAuthenticationSensitiveDataAwareSchemeAnonymous(
			kkComps.VirtualClusterAuthenticationAnonymous{},
		), nil

	case kkComps.VirtualClusterAuthenticationSchemeTypeSaslPlain:
		if auth.VirtualClusterAuthenticationSaslPlain == nil {
			return kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme{},
				fmt.Errorf("SASL Plain authentication data is missing")
		}

		saslPlain := kkComps.VirtualClusterAuthenticationSaslPlainSensitiveDataAware{
			Mediation: kkComps.Mediation(auth.VirtualClusterAuthenticationSaslPlain.Mediation),
		}

		// Convert principals if present
		if len(auth.VirtualClusterAuthenticationSaslPlain.Principals) > 0 {
			principals := make([]kkComps.VirtualClusterAuthenticationPrincipalSensitiveDataAware, 0,
				len(auth.VirtualClusterAuthenticationSaslPlain.Principals))
			for _, principal := range auth.VirtualClusterAuthenticationSaslPlain.Principals {
				principals = append(principals, kkComps.VirtualClusterAuthenticationPrincipalSensitiveDataAware{
					Username: principal.Username,
					Password: &principal.Password,
				})
			}
			saslPlain.Principals = principals
		}

		return kkComps.CreateVirtualClusterAuthenticationSensitiveDataAwareSchemeSaslPlain(saslPlain), nil

	case kkComps.VirtualClusterAuthenticationSchemeTypeSaslScram:
		if auth.VirtualClusterAuthenticationSaslScram == nil {
			return kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme{},
				fmt.Errorf("SASL SCRAM authentication data is missing")
		}
		return kkComps.CreateVirtualClusterAuthenticationSensitiveDataAwareSchemeSaslScram(
			kkComps.VirtualClusterAuthenticationSaslScram{
				Algorithm: auth.VirtualClusterAuthenticationSaslScram.Algorithm,
			},
		), nil

	case kkComps.VirtualClusterAuthenticationSchemeTypeOauthBearer:
		if auth.VirtualClusterAuthenticationOauthBearer == nil {
			return kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme{},
				fmt.Errorf("OAuth Bearer authentication data is missing")
		}
		return kkComps.CreateVirtualClusterAuthenticationSensitiveDataAwareSchemeOauthBearer(
			*auth.VirtualClusterAuthenticationOauthBearer,
		), nil

	default:
		return kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme{},
			fmt.Errorf("unsupported authentication type: %s", auth.Type)
	}
}
