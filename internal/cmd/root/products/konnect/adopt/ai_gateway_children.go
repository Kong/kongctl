package adopt

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/build"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

type aiChildObservation struct {
	childCandidate
	payload map[string]any
}

type aiChildFamily struct {
	name, alias, selector string
	list                  func(context.Context, string) ([]childCandidate, error)
	prepare               func(context.Context, string, childCandidate, string, bool) (preparedChildAdoption, error)
}

func aiChildAdapter[T, Request any](name, alias, selector string, kind resources.ResourceType,
	list func(context.Context, string) ([]T, error),
	get func(context.Context, string, string) (*T, error),
	observe func(T) (aiChildObservation, error),
	update func(context.Context, string, string, Request, string) (string, error),
) aiChildFamily {
	read := func(ctx context.Context, parent, id string) (aiChildObservation, error) {
		current, err := get(ctx, parent, id)
		if err != nil {
			return aiChildObservation{}, err
		}
		if current == nil {
			return aiChildObservation{}, fmt.Errorf("child %s no longer exists within selected gateway", id)
		}
		observed, err := observe(*current)
		if err != nil {
			return observed, err
		}
		if observed.id != id {
			return observed, fmt.Errorf("child ID changed during adoption")
		}
		return observed, nil
	}
	return aiChildFamily{
		name: name, alias: alias, selector: selector,
		list: func(ctx context.Context, parent string) ([]childCandidate, error) {
			values, err := list(ctx, parent)
			if err != nil {
				return nil, err
			}
			result := make([]childCandidate, 0, len(values))
			for _, value := range values {
				observed, err := observe(value)
				if err != nil {
					return nil, err
				}
				result = append(result, observed.childCandidate)
			}
			return result, nil
		},
		prepare: func(ctx context.Context, parent string, selected childCandidate,
			namespace string, overwrite bool,
		) (preparedChildAdoption, error) {
			observed, err := read(ctx, parent, selected.id)
			if err != nil {
				return nil, err
			}
			if observed.name != selected.name {
				return nil, fmt.Errorf("child name changed during adoption")
			}
			metadata, err := childPayloadMetadata(observed.payload)
			if err != nil {
				return nil, err
			}
			desired, err := adoptionManagedBy(metadata, namespace, overwrite, build.Version(ctx))
			if err != nil {
				return nil, err
			}
			if kind == resources.ResourceTypeAIGatewayVault {
				config, _ := observed.payload[planner.FieldConfig].(map[string]any)
				if observed.payload[planner.FieldType] == "hcv" && config["auth_method"] == "approle" {
					secretFile, _ := config["secret_id_file"].(string)
					if strings.TrimSpace(secretFile) == "" {
						return nil, fmt.Errorf("AppRole vault adoption cannot preserve redacted secret_id; backend metadata " +
							"PATCH support or secret_id_file is required")
					}
				}
			}
			payload := maps.Clone(observed.payload)
			payload[planner.FieldManagedBy] = desired
			var request Request
			if err := mapChildAdoptionRequest(payload, &request); err != nil {
				return nil, err
			}
			if err := prepareDeclarativeChild(
				ctx, kind, selected.id, selected.id, namespace, resources.IsChildProtected(desired), payload,
				adoptionParent{kind: resources.ResourceTypeAIGateway, id: parent},
			); err != nil {
				return nil, err
			}
			return func() (*adoptCommon.AdoptResult, error) {
				if result, dryRun := adoptCommon.PreviewAdoption(
					ctx, string(kind), selected.id, selected.name, namespace, map[string]string{"gateway_id": parent},
					"managed_by", metadata, desired,
				); dryRun {
					return finishDeclarativeChild(ctx, result)
				}
				fresh, err := read(ctx, parent, selected.id)
				if err != nil {
					return nil, err
				}
				if !reflect.DeepEqual(fresh, observed) {
					return nil, fmt.Errorf("child changed after adoption validation; retry adoption")
				}
				id, err := update(ctx, parent, selected.id, request, namespace)
				if err != nil {
					return nil, err
				}
				if id != selected.id {
					return nil, fmt.Errorf("adoption API did not retain the child ID")
				}
				updated, err := read(ctx, parent, selected.id)
				if err != nil {
					return nil, err
				}
				retained, err := childPayloadMetadata(updated.payload)
				if err != nil {
					return nil, err
				}
				if updated.name != selected.name || !maps.Equal(retained, desired) {
					return nil, fmt.Errorf("adoption API did not retain the child identity or ownership metadata")
				}
				return finishDeclarativeChild(ctx, &adoptCommon.AdoptResult{
					ResourceType: string(kind),
					ID:           selected.id,
					Name:         selected.name,
					Namespace:    namespace,
				})
			}, nil
		},
	}
}

func childPayloadMetadata(payload map[string]any) (map[string]string, error) {
	data, err := json.Marshal(payload[planner.FieldManagedBy])
	if err != nil {
		return nil, fmt.Errorf("cannot encode child ownership metadata")
	}
	var metadata map[string]string
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("invalid child ownership metadata")
	}
	return metadata, nil
}

// Verify that SDK mapping retains every observed writable field. Never silently
// drop public configuration while performing an ownership-only full PUT.
func mapChildAdoptionRequest(payload map[string]any, destination any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("cannot encode child adoption request")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("SDK cannot represent child adoption request")
	}
	mapped, err := json.Marshal(destination)
	if err != nil {
		return fmt.Errorf("cannot verify child adoption request")
	}
	var original, retained any
	if err := json.Unmarshal(data, &original); err != nil {
		return fmt.Errorf("cannot inspect child adoption request")
	}
	if err := json.Unmarshal(mapped, &retained); err != nil {
		return fmt.Errorf("cannot inspect mapped child adoption request")
	}
	if !adoptionPayloadRetained(original, retained) {
		return fmt.Errorf("SDK cannot preserve all public fields during child adoption")
	}
	return nil
}

func adoptionPayloadRetained(original, retained any) bool {
	switch value := original.(type) {
	case map[string]any:
		mapped, ok := retained.(map[string]any)
		if !ok {
			return len(value) == 0 && retained == nil
		}
		for key, field := range value {
			actual, exists := mapped[key]
			if !exists && field == nil {
				continue
			}
			if !adoptionPayloadRetained(field, actual) {
				return false
			}
		}
		return true
	case []any:
		mapped, ok := retained.([]any)
		if !ok {
			return len(value) == 0 && retained == nil
		}
		if len(value) != len(mapped) {
			return false
		}
		for i := range value {
			if !adoptionPayloadRetained(value[i], mapped[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(original, retained)
	}
}

func aiChildSnapshot(id string, payload map[string]any, err error) (aiChildObservation, error) {
	if err != nil {
		return aiChildObservation{}, err
	}
	delete(payload, "konnect_managed")
	name, _ := payload[planner.FieldName].(string)
	return aiChildObservation{childCandidate{id: id, name: name}, payload}, nil
}

func aiChildFamilies(client *state.Client) []aiChildFamily {
	return []aiChildFamily{
		aiChildAdapter("models", "model", "model", resources.ResourceTypeAIGatewayModel,
			client.ListAIGatewayModels, client.GetAIGatewayModel,
			func(value state.AIGatewayModel) (aiChildObservation, error) {
				payload, err := resources.AIGatewayModelMutablePayloadMap(value.AIGatewayModel)
				return aiChildSnapshot(resources.AIGatewayModelID(value.AIGatewayModel), payload, err)
			}, client.UpdateAIGatewayModel),
		aiChildAdapter("model-providers", "model-provider", "model-provider", resources.ResourceTypeAIGatewayProvider,
			client.ListAIGatewayProviders, client.GetAIGatewayProvider,
			func(value state.AIGatewayProvider) (aiChildObservation, error) {
				writable := resources.AIGatewayProviderResource{
					Name:        value.Name,
					Type:        value.Type,
					DisplayName: value.DisplayName,
					Labels:      value.Labels,
					ManagedBy:   value.ManagedBy,
					Config:      value.Config,
					Datastores:  value.Datastores,
					TokenVault:  value.TokenVault,
				}
				payload, err := writable.MutablePayloadMap()
				return aiChildSnapshot(value.ID, payload, err)
			}, client.UpdateAIGatewayProvider),
		aiChildAdapter("auth-strategies", "auth-strategy", "auth-strategy", resources.ResourceTypeAIGatewayAuthStrategy,
			client.ListAIGatewayAuthStrategies, client.GetAIGatewayAuthStrategy,
			func(value state.AIGatewayAuthStrategy) (aiChildObservation, error) {
				writable := resources.AIGatewayAuthStrategyResource{
					Name:        value.Name,
					Type:        value.Type,
					DisplayName: value.DisplayName,
					Labels:      value.Labels,
					ManagedBy:   value.ManagedBy,
					Config:      value.Config,
					Datastores:  value.Datastores,
					TokenVault:  value.TokenVault,
				}
				payload, err := writable.MutablePayloadMap()
				return aiChildSnapshot(value.ID, payload, err)
			}, client.UpdateAIGatewayAuthStrategy),
		aiChildAdapter("policies", "policy", "policy", resources.ResourceTypeAIGatewayPolicy,
			client.ListAIGatewayPolicies, client.GetAIGatewayPolicy,
			func(value state.AIGatewayPolicy) (aiChildObservation, error) {
				payload, err := resources.AIGatewayPolicyMutablePayloadMap(value.AIGatewayPolicy)
				return aiChildSnapshot(resources.AIGatewayPolicyID(value.AIGatewayPolicy), payload, err)
			}, client.UpdateAIGatewayPolicy),
		aiChildAdapter("custom-policies", "custom-policy", "custom-policy", resources.ResourceTypeAIGatewayCustomPolicy,
			client.ListAIGatewayCustomPolicies, client.GetAIGatewayCustomPolicy,
			func(value state.AIGatewayCustomPolicy) (aiChildObservation, error) {
				payload, err := resources.AIGatewayCustomPolicyMutablePayloadMap(value.AIGatewayCustomPolicy)
				return aiChildSnapshot(resources.AIGatewayCustomPolicyID(value.AIGatewayCustomPolicy), payload, err)
			}, client.UpdateAIGatewayCustomPolicy),
		aiChildAdapter("agents", "agent", "agent", resources.ResourceTypeAIGatewayAgent,
			client.ListAIGatewayAgents, client.GetAIGatewayAgent,
			func(value state.AIGatewayAgent) (aiChildObservation, error) {
				payload, err := resources.AIGatewayAgentMutablePayloadMap(value.AIGatewayAgent)
				return aiChildSnapshot(resources.AIGatewayAgentID(value.AIGatewayAgent), payload, err)
			}, client.UpdateAIGatewayAgent),
		aiChildAdapter("mcp-servers", "mcp-server", "mcp-server", resources.ResourceTypeAIGatewayMCPServer,
			client.ListAIGatewayMCPServers, client.GetAIGatewayMCPServer,
			func(value state.AIGatewayMCPServer) (aiChildObservation, error) {
				payload, err := resources.AIGatewayMCPServerMutablePayloadMap(value.AIGatewayMCPServer)
				return aiChildSnapshot(resources.AIGatewayMCPServerID(value.AIGatewayMCPServer), payload, err)
			}, client.UpdateAIGatewayMCPServer),
		aiChildAdapter("vaults", "vault", "vault", resources.ResourceTypeAIGatewayVault,
			client.ListAIGatewayVaults, client.GetAIGatewayVault,
			func(value state.AIGatewayVault) (aiChildObservation, error) {
				payload, err := resources.AIGatewayVaultMutablePayloadMap(value.AIGatewayVault)
				return aiChildSnapshot(resources.AIGatewayVaultID(value.AIGatewayVault), payload, err)
			}, client.UpdateAIGatewayVault),
		aiChildAdapter("consumers", "consumer", "consumer", resources.ResourceTypeAIGatewayConsumer,
			client.ListAIGatewayConsumers, client.GetAIGatewayConsumer,
			func(value state.AIGatewayConsumer) (aiChildObservation, error) {
				payload, err := resources.AIGatewayConsumerMutablePayloadMap(value.AIGatewayConsumer)
				return aiChildSnapshot(resources.AIGatewayConsumerID(value.AIGatewayConsumer), payload, err)
			}, client.UpdateAIGatewayConsumer),
		aiChildAdapter("consumer-groups", "consumer-group", "consumer-group", resources.ResourceTypeAIGatewayConsumerGroup,
			client.ListAIGatewayConsumerGroups, client.GetAIGatewayConsumerGroup,
			func(value state.AIGatewayConsumerGroup) (aiChildObservation, error) {
				payload, err := resources.AIGatewayConsumerGroupMutablePayloadMap(value.AIGatewayConsumerGroup)
				return aiChildSnapshot(resources.AIGatewayConsumerGroupID(value.AIGatewayConsumerGroup), payload, err)
			}, client.UpdateAIGatewayConsumerGroup),
		aiChildAdapter("ca-certificates", "ca-certificate", "ca-certificate", resources.ResourceTypeAIGatewayCACertificate,
			client.ListAIGatewayCACertificates, client.GetAIGatewayCACertificate,
			func(value state.AIGatewayCACertificate) (aiChildObservation, error) {
				writable := resources.AIGatewayCACertificateResource{
					Name:      value.Name,
					Cert:      value.Cert,
					Labels:    value.Labels,
					ManagedBy: value.ManagedBy,
				}
				return aiChildSnapshot(value.ID, writable.PayloadMap(), nil)
			}, client.UpdateAIGatewayCACertificate),
		aiChildAdapter("snis", "sni", "sni", resources.ResourceTypeAIGatewaySNI,
			client.ListAIGatewaySNIs, client.GetAIGatewaySNI,
			func(value state.AIGatewaySNI) (aiChildObservation, error) {
				return aiChildSnapshot(value.ID,
					map[string]any{
						planner.FieldName:        value.Name,
						planner.FieldDisplayName: value.DisplayName,
						planner.FieldHostname:    value.Hostname,
						planner.FieldCertificate: value.Certificate,
						planner.FieldLabels:      value.Labels,
						planner.FieldManagedBy:   value.ManagedBy,
					},
					nil)
			}, client.UpdateAIGatewaySNI),
	}
}

func newAIGatewayChildCommands(verb verbs.VerbValue,
	addFlags func(verbs.VerbValue, *cobra.Command), preRun func(*cobra.Command, []string) error,
) []*cobra.Command {
	var commands []*cobra.Command
	// Metadata and typed callbacks share one inventory; APIs are supplied per run.
	for index, family := range aiChildFamilies(state.NewClient(state.ClientConfig{})) {
		command := &cobra.Command{
			Use: family.name + " [name|id]", Aliases: []string{family.alias},
			Short: "Adopt existing AI Gateway " + family.name + " into a namespace", PreRunE: preRun,
			Long: "Adopt only selected children using their independent managed_by ownership. The " +
				"gateway and descendants are not adopted. Public fields, credentials, and " +
				"protection are preserved. Bulk operations validate all selected children before " +
				"writing, but are not atomic.",
		}
		command.Flags().String("gateway-id", "", "Parent AI Gateway UUID")
		command.Flags().String("gateway-name", "", "Exact parent AI Gateway name or display name")
		command.MarkFlagsOneRequired("gateway-id", "gateway-name")
		command.MarkFlagsMutuallyExclusive("gateway-id", "gateway-name")
		command.Flags().String(family.selector+"-id", "", "Exact child UUID")
		command.Flags().String(family.selector+"-name", "", "Exact child name (including UUID-shaped names)")
		addChildSelectionFlags(command)
		addDeclarativeExportFlags(command)
		command.Args = func(cmd *cobra.Command, args []string) error {
			selection, err := readChildSelection(cmd, args, family.selector+"-id", family.selector+"-name")
			if err == nil && selection.idOnly && !util.IsValidUUID(selection.identifier) {
				return fmt.Errorf("child ID must be a UUID")
			}
			return err
		}
		if addFlags != nil {
			addFlags(verb, command)
		}
		command.Example = "  kongctl adopt ai-gateway " + family.name +
			" existing-child --gateway-name shared-gateway --namespace team-a\n" +
			"  kongctl adopt ai-gateway " + family.name + " --all --gateway-name shared-gateway --namespace team-a\n" +
			"  kongctl adopt ai-gateway " + family.name +
			" --name-pattern 'team-*' --gateway-name shared-gateway --namespace team-a"
		command.Example += "\n  kongctl adopt ai-gateway " + family.name +
			" --name-pattern 'legacy-*' --gateway-name shared-gateway --namespace team-a" +
			" --parent-ref shared-gateway --declarative-output adopted.yaml"

		command.RunE = func(cmd *cobra.Command, args []string) error {
			setup, cleanup, err := setupChildAdoptRun(cmd, args)
			if err != nil {
				return err
			}
			defer cleanup()
			selection, err := readChildSelection(cmd, args, family.selector+"-id", family.selector+"-name")
			if err != nil {
				return err
			}
			identifier, _ := cmd.Flags().GetString("gateway-id")
			if identifier != "" && !util.IsValidUUID(identifier) {
				return fmt.Errorf("gateway-id must be a UUID")
			}
			if identifier == "" {
				identifier, _ = cmd.Flags().GetString("gateway-name")
			}
			if strings.TrimSpace(identifier) == "" {
				return fmt.Errorf("gateway selector cannot be empty")
			}
			gateway,
				err := resolveAIGateway(setup.Helper,
				setup.SDK.GetAIGatewayAPI(),
				setup.Cfg,
				identifier,
				cmd.Flags().Changed("gateway-name"))
			if err != nil {
				return err
			}
			if gateway.ID == "" {
				return fmt.Errorf("selected AI Gateway has no ID")
			}
			client := state.NewClient(state.ClientConfig{
				AIGatewayProvidersAPI:      setup.SDK.GetAIGatewayProvidersAPI(),
				AIGatewayAuthStrategiesAPI: setup.SDK.GetAIGatewayAuthStrategiesAPI(),
				AIGatewayCustomPoliciesAPI: setup.SDK.GetAIGatewayCustomPoliciesAPI(),
				AIGatewayPoliciesAPI:       setup.SDK.GetAIGatewayPoliciesAPI(),
				AIGatewayAgentsAPI:         setup.SDK.GetAIGatewayAgentsAPI(),
				AIGatewayConsumersAPI:      setup.SDK.GetAIGatewayConsumersAPI(),
				AIGatewayConsumerGroupsAPI: setup.SDK.GetAIGatewayConsumerGroupsAPI(),
				AIGatewayModelAPI:          setup.SDK.GetAIGatewayModelAPI(),
				AIGatewayMCPServersAPI:     setup.SDK.GetAIGatewayMCPServersAPI(),
				AIGatewayVaultsAPI:         setup.SDK.GetAIGatewayVaultsAPI(),
				AIGatewayCACertificatesAPI: setup.SDK.GetAIGatewayCACertificatesAPI(),
				AIGatewaySNIsAPI:           setup.SDK.GetAIGatewaySNIsAPI(),
			})
			adapter := aiChildFamilies(client)[index]
			ctx := adoptCommon.EnsureContext(setup.Helper.GetContext())
			values, err := adapter.list(ctx, gateway.ID)
			if err != nil {
				return err
			}
			selected, err := selectChildren(values, selection)
			if err != nil {
				return err
			}
			return runChildAdoptions(setup, selected, func(value childCandidate) (preparedChildAdoption, error) {
				return adapter.prepare(ctx, gateway.ID, value, setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
			})
		}
		commands = append(commands, command)
	}
	return commands
}
