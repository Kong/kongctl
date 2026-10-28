package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/state"
)

const opaqueVaultReference = "\x00redacted-vault-reference"

var publicVaultReferencePattern = regexp.MustCompile(`\{vault://([^/}]+)|\$\{vault[.\[]([^/\].}]+)`)

func publicChildReferences(kind string, payload any) []string {
	var result []string
	var visit func(any, string)
	visit = func(value any, key string) {
		switch typed := value.(type) {
		case map[string]any:
			for k, v := range typed {
				visit(v, k)
			}
		case []any:
			for _, v := range typed {
				visit(v, key)
			}
		case string:
			switch kind {
			case ResourceTypeAIGatewayVault:
				for _, match := range publicVaultReferencePattern.FindAllStringSubmatch(typed, -1) {
					for _, name := range match[1:] {
						if name != "" {
							result = append(result, name)
						}
					}
				}
			case ResourceTypeAIGatewayCACertificate:
				if key == "ca_certificates" || key == "ca_certificate" {
					result = append(result, typed)
				}
			case ResourceTypeAIGatewaySNI:
				if key == "snis" || key == "sni" {
					result = append(result, typed)
				}
			}
		}
	}
	if object, ok := payload.(map[string]any); ok {
		visit(object[FieldConfig], FieldConfig)
	}
	return result
}

// Write-only credential values can contain vault references, but the API hides
// even the public reference on GET/list. Absence cannot prove non-use. Refuse
// direct vault deletion while such a retained user exists; a user deletion is
// proof, whereas an omitted credential in a full PUT preserves its old value.
func hasOpaqueVaultReferences(kind string, payload map[string]any) bool {
	config, _ := payload[FieldConfig].(map[string]any)
	auth, _ := config["auth"].(map[string]any)
	switch kind {
	case ResourceTypeAIGatewayProvider:
		authType, _ := auth[FieldType].(string)
		if authType == "basic" {
			for _, field := range []string{"headers", "params"} {
				if items, ok := auth[field].([]any); ok && len(items) > 0 {
					return true
				}
			}
			return false
		}
		return len(auth) > 0
	case ResourceTypeAIGatewayAuthStrategy:
		return payload[FieldType] == "openid-connect" && len(config) > 0
	case ResourceTypeAIGatewayMCPServer, ResourceTypeAIGatewayAgent:
		upstream, _ := config["upstream"].(map[string]any)
		proxy, _ := config["proxy"].(map[string]any)
		if upstream["auth"] != nil || proxy["auth"] != nil {
			return true
		}
		// Upstream-server tool authentication is also write-only.
		server, _ := config["server"].(map[string]any)
		toolsAuth, _ := server["tools_list_auth"].(map[string]any)
		return toolsAuth[FieldType] == "credentials"
	case ResourceTypeAIGatewayVault:
		return payload[FieldType] == "hcv" || payload[FieldType] == "conjur"
	}
	return false
}

func referenceUsersFromValues[T any](
	kind, target string,
	values []T,
) ([]observedReferenceUser, error) {
	users := make([]observedReferenceUser, 0, len(values))
	for _, value := range values {
		observation := any(value)
		switch child := observation.(type) {
		case state.AIGatewayProvider:
			observation = map[string]any{
				FieldID:     child.ID,
				FieldName:   child.Name,
				FieldType:   child.Type,
				FieldConfig: child.Config,
			}
		case state.AIGatewayAuthStrategy:
			observation = map[string]any{
				FieldID:     child.ID,
				FieldName:   child.Name,
				FieldType:   child.Type,
				FieldConfig: child.Config,
			}
		}
		data, err := json.Marshal(observation)
		if err != nil {
			return nil, fmt.Errorf("cannot inspect AI Gateway reference observation")
		}
		var payload map[string]any
		if json.Unmarshal(data, &payload) != nil {
			return nil, fmt.Errorf("cannot decode AI Gateway reference observation")
		}
		id, _ := payload[FieldID].(string)
		name, _ := payload[FieldName].(string)
		references := publicChildReferences(target, payload)
		if target == ResourceTypeAIGatewayVault && hasOpaqueVaultReferences(kind, payload) {
			references = append(references, opaqueVaultReference)
		}
		users = append(users, observedReferenceUser{kind, id, name, references})
	}
	return users, nil
}

func (p *Planner) observeAdditionalAIGatewayReferences(
	ctx context.Context,
	gatewayID, target string,
) ([]observedReferenceUser, error) {
	var users []observedReferenceUser
	{
		values, err := p.listAIGatewayProviders(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayProvider, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayAuthStrategies(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayAuthStrategy, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayPolicies(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayPolicy, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayCustomPolicies(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayCustomPolicy, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayAgents(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayAgent, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayModels(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayModel, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayMCPServers(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayMCPServer, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayConsumers(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayConsumer, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayConsumerGroups(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(
			ResourceTypeAIGatewayConsumerGroup,
			target,
			values,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayVaults(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayVault, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayCertificates(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewayCertificate, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewayCACertificates(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(
			ResourceTypeAIGatewayCACertificate,
			target,
			values,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	{
		values, err := p.listAIGatewaySNIs(ctx, gatewayID)
		if err != nil {
			return nil, err
		}
		observed, err := referenceUsersFromValues(ResourceTypeAIGatewaySNI, target, values)
		if err != nil {
			return nil, err
		}
		users = append(users, observed...)
	}
	return users, nil
}

func (p *Planner) resolveAdditionalAIGatewayReferenceDeletes(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, target string,
	plan *Plan,
) error {
	aliases := map[string]string{}
	if target == ResourceTypeAIGatewaySNI &&
		planHasAIGatewayChildDelete(plan, target, gatewayRef, gatewayID) {
		snis, err := p.listAIGatewaySNIs(ctx, gatewayID)
		if err != nil {
			return err
		}
		for _, sni := range snis {
			hostname, _ := sni.Hostname.(string)
			aliases[sni.ID] = hostname
		}
	}
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.ContainsFunc(values, func(value string) bool {
			return value != "" &&
				(value == name || value == deletion.ResourceID || value == aliases[deletion.ResourceID] ||
					value == opaqueVaultReference)
		})
	}
	opaqueUsers := map[string]bool{}
	return resolveObservedReferenceDeletes(
		ctx,
		namespace,
		gatewayRef,
		plan,
		observedReferenceDeletePolicy{
			gatewayID: gatewayID, targetType: target,
			usesTarget: func(kind string) bool { return strings.HasPrefix(kind, ResourceTypeAIGateway+"_") },
			observe: func(ctx context.Context) ([]observedReferenceUser, error) {
				users, err := p.observeAdditionalAIGatewayReferences(ctx, gatewayID, target)
				for _, user := range users {
					if slices.Contains(user.references, opaqueVaultReference) {
						opaqueUsers[user.resourceType+"/"+user.id] = true
					}
				}
				return users, err
			},
			observedReferences: references,
			plannedReferences: func(deletion, change PlannedChange) bool {
				return references(deletion, publicChildReferences(target, change.Fields))
			},
			updateDetaches: func(deletion, change PlannedChange) bool {
				// Metadata-only PUTs preserve fresh public fields and unselected
				// credentials. Only an explicit configuration change proves detach.
				if _, changed := change.ChangedFields[FieldConfig]; !changed {
					return false
				}
				if _, present := change.Fields[FieldConfig]; !present {
					return false
				}
				if target == ResourceTypeAIGatewayVault &&
					opaqueUsers[change.ResourceType+"/"+change.ResourceID] {
					return false
				}
				return !references(deletion, publicChildReferences(target, change.Fields))
			},
			conflict: func(deletion PlannedChange, user observedReferenceUser, _ bool) error {
				if target == ResourceTypeAIGatewayVault &&
					slices.Contains(user.references, opaqueVaultReference) {
					return fmt.Errorf(
						"cannot delete AI Gateway vault %q: retained %s %q has redacted credentials "+
							"that may reference it; the API does not expose those references",

						deletion.Fields[FieldName],
						user.resourceType,
						user.name,
					)
				}
				return fmt.Errorf(
					"cannot delete %s %q while %s %q still references it",
					target,
					deletion.Fields[FieldName],
					user.resourceType,
					user.name,
				)
			},
		},
	)
}

func planHasAIGatewayChildDelete(plan *Plan, kind, gatewayRef, gatewayID string) bool {
	return slices.ContainsFunc(plan.Changes, func(change PlannedChange) bool {
		return change.ResourceType == kind && change.Action == ActionDelete &&
			aiGatewayChangeMatchesGateway(change, gatewayRef, gatewayID)
	})
}

func (p *Planner) resolveAIGatewayVaultDeletes(
	ctx context.Context,
	ns, ref, id string,
	plan *Plan,
) error {
	return p.resolveAdditionalAIGatewayReferenceDeletes(
		ctx,
		ns,
		ref,
		id,
		ResourceTypeAIGatewayVault,
		plan,
	)
}

func (p *Planner) resolveAIGatewayCACertificateDeletes(
	ctx context.Context,
	ns, ref, id string,
	plan *Plan,
) error {
	return p.resolveAdditionalAIGatewayReferenceDeletes(
		ctx,
		ns,
		ref,
		id,
		ResourceTypeAIGatewayCACertificate,
		plan,
	)
}

func (p *Planner) resolveAIGatewaySNIDeletes(
	ctx context.Context,
	ns, ref, id string,
	plan *Plan,
) error {
	return p.resolveAdditionalAIGatewayReferenceDeletes(
		ctx,
		ns,
		ref,
		id,
		ResourceTypeAIGatewaySNI,
		plan,
	)
}
