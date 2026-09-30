package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestResolveAIGatewayAuthStrategyReferences(t *testing.T) {
	for _, kind := range []string{ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer} {
		for _, action := range []ActionType{ActionCreate, ActionUpdate} {
			for _, selector := range []string{FieldID, FieldName} {
				for _, createStrategy := range []bool{false, true} {
					name := kind + "/" + string(action) + "/" + selector + "/existing"
					if createStrategy {
						name = kind + "/" + string(action) + "/" + selector + "/new"
					}
					t.Run(name, func(t *testing.T) {
						rs := &resources.ResourceSet{
							AIGatewayAuthStrategies: []resources.AIGatewayAuthStrategyResource{{
								BaseResource: resources.BaseResource{Ref: "auth-ref"}, Name: "auth-name",
							}},
						}
						changes := []PlannedChange{{
							ID: "user", ResourceType: kind, Action: action,
							Fields: authStrategyAccessFields([]any{"__REF__:auth-ref#" + selector}),
						}}
						wantID := "auth-name"
						if createStrategy {
							changes = append(changes, PlannedChange{
								ID: "auth", ResourceType: ResourceTypeAIGatewayAuthStrategy,
								Action: ActionCreate, ResourceRef: "auth-ref",
							})
							wantID = resources.UnknownReferenceID
						}
						resolved, err := NewReferenceResolver(nil, rs).ResolveReferences(t.Context(), changes)
						require.NoError(t, err)
						require.Empty(t, resolved.Errors)
						field := FieldAccess + "." + FieldAuthStrategies + ".0"
						require.Equal(t, ResolvedReference{
							Ref: "__REF__:auth-ref#name", ID: wantID,
						}, resolved.ChangeReferences["user"][field])
					})
				}
			}
		}
	}
}
