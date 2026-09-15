package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

// ControlPlane is a Konnect hosted Kong Mesh control plane.
//
// Version is the Konnect API line the control plane is reached on — "v0" or
// "v3" — not the control plane's own version, which only GET / reports. A
// control plane labelled v3 may still be running 2.14 behind the v1 line, so
// nothing may be inferred from this field beyond which prefix to use.
type ControlPlane struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	Features    []any             `json:"features"`
}

// controlPlanesResponse is the standard Konnect list envelope, which differs
// from the envelope a control plane's own API uses for resource lists.
type controlPlanesResponse struct {
	Data []ControlPlane `json:"data"`
	Meta struct {
		Page struct {
			Number int `json:"number"`
			Size   int `json:"size"`
			Total  int `json:"total"`
		} `json:"page"`
	} `json:"meta"`
}

// controlPlanePageSize is how many control planes are requested per call.
const controlPlanePageSize = 100

// ListControlPlanes returns every Kong Mesh control plane the authenticated
// identity can see.
//
// This addresses Konnect rather than a control plane, so it composes the
// Konnect base URL directly instead of going through the per control plane
// resolver — which would be circular, since that resolver may need this list.
func ListControlPlanes(helper cmd.Helper) ([]ControlPlane, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return nil, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return nil, err
	}

	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return nil, err
	}

	tokenSource, err := konnectcommon.GetAccessTokenSource(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("resolve Konnect access token: %w", err)
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := konnectcommon.ResolveAccessToken(ctx, cfg, tokenSource); err != nil {
		return nil, fmt.Errorf("resolve Konnect access token: %w", err)
	}

	var controlPlanes []ControlPlane
	client := httpclient.NewLoggingHTTPClient(logger)

	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("page[size]", fmt.Sprint(controlPlanePageSize))
		query.Set("page[number]", fmt.Sprint(page))
		path := meshcommon.ControlPlanesPath + "?" + query.Encode()

		result, err := apiutil.RequestWithTokenSource(
			ctx, client, http.MethodGet, strings.TrimRight(baseURL, "/"), path, tokenSource, nil, nil)
		if err != nil {
			return nil, err
		}

		logger.Debug("mesh control plane list call completed",
			"path", path, "status_code", result.StatusCode)

		if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
			return nil, buildAPIError(result.StatusCode, result.Body)
		}

		var payload controlPlanesResponse
		if err := json.Unmarshal(result.Body, &payload); err != nil {
			return nil, fmt.Errorf("failed to decode the mesh control plane list: %w", err)
		}

		controlPlanes = append(controlPlanes, payload.Data...)

		// An empty page ends the listing whatever the total says, so a
		// miscounted total cannot spin here.
		if len(payload.Data) == 0 || len(controlPlanes) >= payload.Meta.Page.Total {
			return controlPlanes, nil
		}
	}
}

// resolveControlPlaneIDByName finds the control plane an operator named.
//
// Konnect does not constrain control plane names to be unique, so an ambiguous
// name is reported rather than resolved arbitrarily: picking one would send
// writes to a control plane the operator did not choose.
func resolveControlPlaneIDByName(helper cmd.Helper, name string) (string, error) {
	controlPlanes, err := ListControlPlanes(helper)
	if err != nil {
		return "", err
	}

	var matches []ControlPlane
	for _, controlPlane := range controlPlanes {
		if controlPlane.Name == name {
			matches = append(matches, controlPlane)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0].ID, nil
	case 0:
		return "", fmt.Errorf(
			"no Kong Mesh control plane named %q; run 'get mesh control-planes' to list them", name)
	default:
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, match.ID)
		}
		return "", fmt.Errorf(
			"%d Kong Mesh control planes are named %q; select one with --%s: %s",
			len(matches), name, meshcommon.ControlPlaneIDFlagName, strings.Join(ids, ", "))
	}
}
