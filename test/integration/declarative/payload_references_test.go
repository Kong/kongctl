//go:build integration

package declarative_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

func TestPayloadReferencesApplySavedPlanAndReplan(t *testing.T) {
	for _, saved := range []bool{false, true} {
		t.Run(fmt.Sprintf("saved=%t", saved), func(t *testing.T) {
			ctx := context.WithValue(t.Context(), log.LoggerKey, slog.Default())
			remote := &payloadReferenceHTTPClient{}
			s := sdk.New(sdk.WithClient(remote), sdk.WithServerURL("https://konnect.example.test"))
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI: s.AIGateways, AIGatewayPoliciesAPI: s.AIGatewayPolicies,
				ControlPlaneAPI: s.ControlPlanes, GatewayServiceAPI: s.Services,
			})
			file := filepath.Join(t.TempDir(), "config.yaml")
			config := `
control_planes:
  - ref: remote-cp
    _external:
      selector:
        matchFields:
          name: production
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    deployment_type: hybrid
    policies:
      - ref: policy
        name: policy
        display_name: Policy
        type: opentelemetry
        enabled: true
        global: true
        config:
          literal: "__REF__:ordinary#id"
          headers:
            X-Control-Plane-Id: !ref gateway
            other.gateway: !lookup {resource_type: ai_gateway, name: remote}
            inline.service: !lookup
              resource_type: gateway_service
              name: billing
              parent: !lookup {resource_type: control_plane, name: production}
            declared.service: !lookup
              resource_type: gateway_service
              name: billing
              parent_ref: remote-cp
          nested:
            - "0": !ref gateway#id
              display: !ref gateway#display_name
`
			require.NoError(t, os.WriteFile(file, []byte(config), 0o600))
			generate := func() *planner.Plan {
				rs, err := loader.New().LoadFromSources([]loader.Source{{Path: file, Type: loader.SourceTypeFile}}, false)
				require.NoError(t, err)
				plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs,
					planner.Options{Mode: planner.PlanModeApply})
				require.NoError(t, err)
				return plan
			}
			plan := generate()
			require.Len(t, plan.Changes, 2)
			if saved {
				data, err := json.Marshal(plan)
				require.NoError(t, err)
				plan = new(planner.Plan)
				require.NoError(t, json.Unmarshal(data, plan))
			}
			result := executor.New(client, nil, false).Execute(ctx, plan)
			require.Empty(t, result.Errors)
			require.Equal(t, 2, result.SuccessCount)
			require.Equal(t, []string{"POST gateway", "POST policy"}, remote.mutations)
			require.Empty(t, generate().Changes)

			// An unrelated policy update must retain resolved payload references.
			config = strings.Replace(config, "display_name: Policy", "display_name: Updated Policy", 1)
			require.NoError(t, os.WriteFile(file, []byte(config), 0o600))
			update := generate()
			require.Len(t, update.Changes, 1)
			require.Equal(t, planner.ActionUpdate, update.Changes[0].Action)
			result = executor.New(client, nil, false).Execute(ctx, update)
			require.Empty(t, result.Errors)
			require.Empty(t, generate().Changes)
		})
	}
}

// Exercise real SDK serialization while keeping all requests in-process.
type payloadReferenceHTTPClient struct {
	mu        sync.Mutex
	gateway   map[string]any
	policy    map[string]any
	mutations []string
}

func (c *payloadReferenceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	isPolicy := strings.Contains(req.URL.Path, "/policies")
	var response any
	status := http.StatusOK
	if req.Method == http.MethodGet {
		switch {
		case strings.HasSuffix(req.URL.Path, "/ai-gateways"):
			items := []any{map[string]any{"id": "remote-id", "name": "remote", "display_name": "Remote"}}
			if c.gateway != nil {
				items = append(items, c.gateway)
			}
			response = map[string]any{"data": items}
		case strings.HasSuffix(req.URL.Path, "/control-planes"):
			response = map[string]any{"data": []any{map[string]any{"id": "cp-id", "name": "production"}}}
		case strings.HasSuffix(req.URL.Path, "/services"):
			if !strings.Contains(req.URL.Path, "/control-planes/cp-id/") {
				return nil, fmt.Errorf("service lookup used incorrect parent scope")
			}
			response = map[string]any{"data": []any{map[string]any{"id": "service-id", "name": "billing"}}}
		case strings.HasSuffix(req.URL.Path, "/policies"):
			items := []any{}
			if c.policy != nil {
				items = append(items, c.policy)
			}
			response = map[string]any{"data": items}
		case isPolicy:
			response = c.policy
		default:
			response = c.gateway
		}
	} else {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		checked := bytes.ReplaceAll(data, []byte("__REF__:ordinary#id"), []byte("literal"))
		if bytes.Contains(checked, []byte("__REF__:")) || bytes.Contains(checked, []byte("__EXTERNAL__:")) {
			return nil, fmt.Errorf("unresolved expression reached the HTTP client")
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		kind, id := "gateway", "gateway-id"
		if isPolicy {
			kind, id = "policy", "policy-id"
			if raw, ok := payload["config"]; ok {
				config := raw.(map[string]any)
				if config["literal"] != "__REF__:ordinary#id" {
					return nil, fmt.Errorf("ordinary literal was changed")
				}
				headers := config["headers"].(map[string]any)
				if headers["X-Control-Plane-Id"] != "gateway-id" || headers["other.gateway"] != "remote-id" {
					return nil, fmt.Errorf("incorrect resolved outgoing headers")
				}
				if headers["inline.service"] != "service-id" || headers["declared.service"] != "service-id" {
					return nil, fmt.Errorf("incorrect remote child lookup result")
				}
				item := config["nested"].([]any)[0].(map[string]any)
				if item["0"] != "gateway-id" || item["display"] != "Gateway" {
					return nil, fmt.Errorf("incorrect resolved outgoing nested values")
				}
			}
		}
		payload["id"] = id
		if isPolicy {
			if c.policy == nil {
				c.policy = make(map[string]any)
			}
			for key, value := range payload {
				c.policy[key] = value
			}
			response = c.policy
		} else {
			c.gateway = payload
			response = c.gateway
		}
		c.mutations = append(c.mutations, req.Method+" "+kind)
		if req.Method == http.MethodPost {
			status = http.StatusCreated
		}
	}
	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(bytes.NewReader(data)), Request: req,
	}, nil
}
