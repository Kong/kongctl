package adopt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func childExportTestSetup(t *testing.T, dryRun bool, parentRef string) (adoptCommon.AdoptRunSetup, *adoptionExport) {
	t.Helper()
	setup, _ := childTestSetup(t, dryRun, cmdCommon.JSON)
	command := setup.Helper.GetCmd()
	addDeclarativeExportFlags(command)
	require.NoError(t, command.Flags().Set(declarativeOutputFlag, filepath.Join(t.TempDir(), "adopted.yaml")))
	if parentRef != "" {
		require.NoError(t, command.Flags().Set(parentRefFlag, parentRef))
	}
	export, err := openAdoptionExport(command, dryRun)
	require.NoError(t, err)
	command.SetContext(context.WithValue(setup.Helper.GetContext(), declarativeExportKey{}, export))
	t.Cleanup(export.close)
	return setup, export
}

func TestAdoptionDeclarativeExportLoadsEveryAIFamily(t *testing.T) {
	for index, descriptor := range aiChildFamilies(state.NewClient(state.ClientConfig{})) {
		t.Run(descriptor.name, func(t *testing.T) {
			setup, export := childExportTestSetup(t, false, "")
			current := aiAdoptionPayload(t, descriptor.name)
			client := aiAdoptionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					var request map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
					maps.Copy(current, request)
				} else {
					require.Equal(t, http.MethodGet, r.Method)
				}
				require.NoError(t, json.NewEncoder(w).Encode(current))
			})
			write, err := aiChildFamilies(client)[index].prepare(setup.Helper.GetContext(), adoptionGatewayID,
				childCandidate{adoptionChildID, "team-child"}, "team-a", false)
			require.NoError(t, err)
			_, err = write()
			require.NoError(t, err)
			content, err := os.ReadFile(export.path)
			require.NoError(t, err)
			require.Contains(t, string(content), "!ref ")
			require.NotContains(t, string(content), "managed_by:")
			require.NotContains(t, string(content), "_defaults:")
			loaded, err := loader.New().LoadFile(export.path)
			require.NoError(t, err)
			shape, err := adoptionExportShapeForName(descriptor.name)
			require.NoError(t, err)
			children := loaded.AllResourcesByType(shape)
			require.Len(t, children, 1)
			require.Equal(t, adoptionChildID, children[0].GetRef())
			require.Len(t, loaded.AIGateways, 1)
			require.True(t, loaded.AIGateways[0].IsExternal())
		})
	}
}

func adoptionExportShapeForName(name string) (resources.ResourceType, error) {
	for _, kind := range resources.RegisteredTypes() {
		shape, err := adoptionExportShape(kind)
		if err == nil && len(shape.collection) == 1 &&
			shape.collection[0] == "ai_gateway_"+strings.ReplaceAll(name, "-", "_") {
			return kind, nil
		}
	}
	if name == "model-providers" {
		return resources.ResourceTypeAIGatewayProvider, nil
	}
	return "", fmt.Errorf("unknown child family %s", name)
}

func TestAdoptionExportReplansAndComposesWithExistingParent(t *testing.T) {
	for _, parentRef := range []string{"", "shared-gateway"} {
		t.Run(parentRef, func(t *testing.T) {
			setup, export := childExportTestSetup(t, false, parentRef)
			current := aiAdoptionPayload(t, "models")
			delete(current, planner.FieldManagedBy)
			client := aiAdoptionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					var request map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
					maps.Copy(current, request)
				} else {
					require.Equal(t, http.MethodGet, r.Method)
				}
				if strings.Contains(r.URL.Path, "/models") {
					if strings.HasSuffix(r.URL.Path, "/"+adoptionChildID) || r.Method == http.MethodPut {
						require.NoError(t, json.NewEncoder(w).Encode(current))
					} else {
						require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
							"data": []any{current}, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
						}))
					}
				} else {
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"id": adoptionGatewayID, "name": "shared", "display_name": "Shared",
						"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
					}))
				}
			})
			write, err := aiChildFamilies(client)[0].prepare(setup.Helper.GetContext(), adoptionGatewayID,
				childCandidate{adoptionChildID, "team-child"}, "team-a", false)
			require.NoError(t, err)
			_, err = write()
			require.NoError(t, err)
			sources := []loader.Source{{Type: loader.SourceTypeFile, Path: export.path}}
			content, err := os.ReadFile(export.path)
			require.NoError(t, err)
			if parentRef != "" {
				require.NotContains(t, string(content), "ai_gateways:")
				require.Contains(t, string(content), "!ref shared-gateway#id")
				parentPath := filepath.Join(t.TempDir(), "parent.yaml")
				require.NoError(t,
					os.WriteFile(parentPath,
						[]byte("ai_gateways:\n- ref: shared-gateway\n  _external:\n    id: "+adoptionGatewayID+"\n"),
						0o600))
				sources = append(sources, loader.Source{Type: loader.SourceTypeFile, Path: parentPath})
			}
			loaded, err := loader.New().LoadFromSources(sources, false)
			require.NoError(t, err)
			plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), loaded,
				planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}

func TestAdoptionExportDryRunIsProposedAndOmitsSecrets(t *testing.T) {
	setup, export := childExportTestSetup(t, true, "existing-gateway")
	payload := map[string]any{
		"type": "openai", "name": "provider", "display_name": "Provider",
		"config": map[string]any{"auth": map[string]any{"type": "basic", "headers": []any{
			map[string]any{"name": "Authorization", "value": "must-never-appear"},
		}}},
	}
	parent := adoptionParent{kind: resources.ResourceTypeAIGateway, id: adoptionGatewayID}
	require.NoError(t, prepareDeclarativeChild(setup.Helper.GetContext(), resources.ResourceTypeAIGatewayProvider,
		adoptionChildID, adoptionChildID, "team-a", true, payload, parent))
	result := &adoptCommon.AdoptResult{ID: adoptionChildID, Name: "provider", Namespace: "team-a"}
	_, err := finishDeclarativeChild(setup.Helper.GetContext(), result)
	require.NoError(t, err)
	content, err := os.ReadFile(export.path)
	require.NoError(t, err)
	require.Contains(t, string(content), "# DRY RUN:")
	require.Contains(t, string(content), "Authorization")
	require.Contains(t, string(content), "protected: true")
	require.NotContains(t, string(content), "must-never-appear")
	require.NotContains(t, string(content), "ai_gateways:")
	config := payload["config"].(map[string]any)
	auth := config["auth"].(map[string]any)
	header := auth["headers"].([]any)[0].(map[string]any)
	require.Equal(t, "must-never-appear", header["value"])
}

func TestAdoptionExportValidatesDestinationAndParentRef(t *testing.T) {
	for _, flags := range [][]string{
		{"--parent-ref", "parent"},
		{"--parent-ref", "bad#id", "--declarative-output", "unused"},
		{"--declarative-output", ""},
		{"--declarative-output", "-"},
	} {
		command := &cobra.Command{}
		addDeclarativeExportFlags(command)
		require.NoError(t, command.ParseFlags(flags))
		_, err := openAdoptionExport(command, false)
		require.Error(t, err)
	}
	directory := t.TempDir()
	existing := filepath.Join(directory, "existing.yaml")
	require.NoError(t, os.WriteFile(existing, []byte("keep"), 0o600))
	for _, path := range []string{existing, filepath.Join(directory, "missing", "output.yaml")} {
		command := &cobra.Command{}
		addDeclarativeExportFlags(command)
		require.NoError(t, command.Flags().Set(declarativeOutputFlag, path))
		_, err := openAdoptionExport(command, false)
		require.Error(t, err)
	}
	content, err := os.ReadFile(existing)
	require.NoError(t, err)
	require.Equal(t, "keep", string(content))
}

func TestAdoptionExportPartialFailureContainsOnlySuccesses(t *testing.T) {
	setup, export := childExportTestSetup(t, false, "")
	second := "6078f559-063b-4f10-9098-9e6553b21d9b"
	values := []childCandidate{{adoptionChildID, "first"}, {second, "second"}}
	writes := 0
	err := runChildAdoptions(setup, values, func(value childCandidate) (preparedChildAdoption, error) {
		payload := map[string]any{"name": value.name, "display_name": value.name}
		if err := prepareDeclarativeChild(
			setup.Helper.GetContext(), resources.ResourceTypeAIGatewayConsumerGroup, value.id, value.id, "team-a",
			false, payload, adoptionParent{kind: resources.ResourceTypeAIGateway, id: adoptionGatewayID},
		); err != nil {
			return nil, err
		}
		return func() (*adoptCommon.AdoptResult, error) {
			if value.id == second {
				return nil, errors.New("API failure")
			}
			writes++
			return finishDeclarativeChild(setup.Helper.GetContext(),
				&adoptCommon.AdoptResult{
					ID:        value.id,
					Name:      value.name,
					Namespace: "team-a",
				})
		}, nil
	})
	require.ErrorContains(t, err, "API failure")
	require.Equal(t, 1, writes)
	content, err := os.ReadFile(export.path)
	require.NoError(t, err)
	require.Contains(t, string(content), "first")
	require.NotContains(t, string(content), "second")
}

func TestAdoptionExportFileFailureStillReportsAdoptedResource(t *testing.T) {
	setup, export := childExportTestSetup(t, false, "")
	candidate := childCandidate{adoptionChildID, "group"}
	err := runChildAdoptions(setup,
		[]childCandidate{candidate},
		func(value childCandidate) (preparedChildAdoption,
			error,
		) {
			require.NoError(t, prepareDeclarativeChild(setup.Helper.GetContext(), resources.ResourceTypeAIGatewayConsumerGroup,
				value.id, value.id, "team-a", false, map[string]any{"name": "group", "display_name": "Group"},
				adoptionParent{kind: resources.ResourceTypeAIGateway, id: adoptionGatewayID}))
			return func() (*adoptCommon.AdoptResult, error) {
				require.NoError(t, export.file.Close())
				return finishDeclarativeChild(setup.Helper.GetContext(),
					&adoptCommon.AdoptResult{
						ID:        value.id,
						Name:      value.name,
						Namespace: "team-a",
					})
			}, nil
		})
	require.ErrorContains(t, err, "was adopted but declarative output")
	require.Len(t, export.completed, 1)
}

func TestAdoptionExportFlagsAreChildOnly(t *testing.T) {
	root, err := NewAIGatewayCmd(verbs.Adopt, nil, nil, nil)
	require.NoError(t, err)
	require.Nil(t, root.Flags().Lookup(declarativeOutputFlag))
	for _, child := range root.Commands() {
		require.NotNil(t, child.Flags().Lookup(declarativeOutputFlag))
		require.NotNil(t, child.Flags().Lookup(parentRefFlag))
	}
}

func TestAdoptionExportKeepsLastSnapshotOnLaterFileFailure(t *testing.T) {
	setup, export := childExportTestSetup(t, false, "")
	ctx := setup.Helper.GetContext()
	parent := adoptionParent{kind: resources.ResourceTypeAIGateway, id: adoptionGatewayID}
	add := func(id, name string) error {
		return prepareDeclarativeChild(ctx, resources.ResourceTypeAIGatewayConsumerGroup,
			id, id, "team-a", false, map[string]any{"name": name, "display_name": name}, parent)
	}
	require.NoError(t, add(adoptionChildID, "first"))
	_,
		err := finishDeclarativeChild(ctx,
		&adoptCommon.AdoptResult{
			ID:        adoptionChildID,
			Name:      "first",
			Namespace: "team-a",
		})
	require.NoError(t, err)
	before, err := os.ReadFile(export.path)
	require.NoError(t, err)
	second := "6078f559-063b-4f10-9098-9e6553b21d9b"
	require.NoError(t, add(second, "second"))
	require.NoError(t, export.file.Close())
	result, err := finishDeclarativeChild(ctx, &adoptCommon.AdoptResult{ID: second, Name: "second", Namespace: "team-a"})
	require.ErrorContains(t, err, "was adopted but declarative output")
	require.Equal(t, second, result.ID)
	after, err := os.ReadFile(export.path)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func TestAdoptionExportDoesNotReplaceChangedDestination(t *testing.T) {
	setup, export := childExportTestSetup(t, false, "")
	ctx := setup.Helper.GetContext()
	require.NoError(t, prepareDeclarativeChild(ctx, resources.ResourceTypeAIGatewayConsumerGroup,
		adoptionChildID, adoptionChildID, "team-a", false, map[string]any{"name": "group", "display_name": "Group"},
		adoptionParent{kind: resources.ResourceTypeAIGateway, id: adoptionGatewayID}))
	require.NoError(t, os.Remove(export.path))
	require.NoError(t, os.WriteFile(export.path, []byte("user-owned replacement"), 0o600))
	_,
		err := finishDeclarativeChild(ctx,
		&adoptCommon.AdoptResult{
			ID:        adoptionChildID,
			Name:      "group",
			Namespace: "team-a",
		})
	require.ErrorContains(t, err, "destination changed during adoption")
	export.close()
	content, err := os.ReadFile(export.path)
	require.NoError(t, err)
	require.Equal(t, "user-owned replacement", string(content))
}
