package dump

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/operatorexport"
)

// maxOffsetPages bounds offset pagination so a misbehaving API cannot loop
// forever.
const maxOffsetPages = 10000

// collectOffsetPages drains an offset-paginated Konnect core entity listing.
func collectOffsetPages[T any](fetch func(offset *string) ([]T, *string, error)) ([]T, error) {
	var all []T
	var offset *string
	for range maxOffsetPages {
		data, next, err := fetch(offset)
		if err != nil {
			return nil, err
		}
		all = append(all, data...)
		if next == nil || strings.TrimSpace(*next) == "" {
			return all, nil
		}
		offset = next
	}
	return nil, fmt.Errorf("pagination exceeded safety limit of %d pages", maxOffsetPages)
}

// fetchRoutes lists routes and classifies each one from the raw response.
//
// The SDK decodes the Route union by trying RouteJSON first, and since every
// RouteJSON field is optional, expression routes decode as RouteJSON with
// their expression dropped. Exporting one of those would let the operator
// rewrite the route as a traditional route, so routes are classified here by
// whether the raw entity carries an expression.
func fetchRoutes(ctx context.Context, sdk *kk.SDK, cpID string, size *int64) ([]kkComps.Route, error) {
	return collectOffsetPages(func(offset *string) ([]kkComps.Route, *string, error) {
		res, err := sdk.Routes.ListRoute(ctx, kkOps.ListRouteRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		routes, err := classifyRoutes(res.RawResponse, res.Object.Data)
		if err != nil {
			return nil, nil, err
		}
		return routes, res.Object.Offset, nil
	})
}

func classifyRoutes(raw *http.Response, decoded []kkComps.Route) ([]kkComps.Route, error) {
	if raw == nil || raw.Body == nil {
		return nil, fmt.Errorf("route response body is unavailable, so expression routes cannot be identified")
	}
	body, err := io.ReadAll(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("read route response: %w", err)
	}
	raw.Body = io.NopCloser(bytes.NewReader(body))

	var page struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decode route response: %w", err)
	}
	if len(page.Data) != len(decoded) {
		return nil, fmt.Errorf("route response has %d entries but %d were decoded", len(page.Data), len(decoded))
	}

	routes := make([]kkComps.Route, len(decoded))
	for i, entry := range page.Data {
		expression, ok := entry["expression"]
		if !ok || string(expression) == "null" {
			routes[i] = decoded[i]
			continue
		}
		var route kkComps.RouteExpression
		encoded, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("encode expression route: %w", err)
		}
		if err := json.Unmarshal(encoded, &route); err != nil {
			return nil, fmt.Errorf("decode expression route: %w", err)
		}
		routes[i] = kkComps.CreateRouteRouteExpression(route)
	}
	return routes, nil
}

// fetchOperatorSnapshot reads every entity of a gateway control plane that the
// Kong Operator can adopt. It also returns warnings about incomplete reads.
func fetchOperatorSnapshot(
	ctx context.Context,
	sdk *kk.SDK,
	cp operatorexport.ControlPlane,
	pageSize int64,
) (operatorexport.Snapshot, []string, error) {
	snap := operatorexport.Snapshot{ControlPlane: cp}
	cpID := cp.ID
	size := new(pageSize)

	var err error
	if snap.Services, err = helpers.GetAllGatewayServices(ctx, pageSize, cpID, sdk); err != nil {
		return snap, nil, fmt.Errorf("failed to list services: %w", err)
	}
	if snap.Routes, err = fetchRoutes(ctx, sdk, cpID, size); err != nil {
		return snap, nil, fmt.Errorf("failed to list routes: %w", err)
	}
	if snap.Plugins, err = helpers.GetAllGatewayPlugins(ctx, pageSize, cpID, sdk); err != nil {
		return snap, nil, fmt.Errorf("failed to list plugins: %w", err)
	}
	if snap.Consumers, err = helpers.GetAllGatewayConsumers(ctx, pageSize, cpID, sdk); err != nil {
		return snap, nil, fmt.Errorf("failed to list consumers: %w", err)
	}
	if snap.ConsumerGroups, err = helpers.GetAllGatewayConsumerGroups(ctx, pageSize, cpID, sdk); err != nil {
		return snap, nil, fmt.Errorf("failed to list consumer groups: %w", err)
	}
	snap.ConsumerGroupMembers = map[string][]string{}
	for _, group := range snap.ConsumerGroups {
		groupID := derefString(group.ID)
		members, err := helpers.GetAllGatewayConsumerGroupConsumers(ctx, pageSize, cpID, groupID, sdk)
		if err != nil {
			return snap, nil, fmt.Errorf("failed to list consumers of consumer group %s: %w", group.Name, err)
		}
		for _, member := range members {
			snap.ConsumerGroupMembers[groupID] = append(snap.ConsumerGroupMembers[groupID], derefString(member.ID))
		}
	}
	if snap.Upstreams, err = helpers.GetAllGatewayUpstreams(ctx, pageSize, cpID, sdk); err != nil {
		return snap, nil, fmt.Errorf("failed to list upstreams: %w", err)
	}
	for _, upstream := range snap.Upstreams {
		targets, err := helpers.GetAllGatewayTargetsForUpstream(ctx, pageSize, cpID, derefString(upstream.ID), sdk)
		if err != nil {
			return snap, nil, fmt.Errorf("failed to list targets of upstream %s: %w", upstream.Name, err)
		}
		snap.Targets = append(snap.Targets, targets...)
	}

	if snap.ACLs, err = collectOffsetPages(func(offset *string) ([]kkComps.ACL, *string, error) {
		res, err := sdk.ACLs.ListACL(ctx, kkOps.ListACLRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list ACL credentials: %w", err)
	}
	if snap.BasicAuths, err = collectOffsetPages(func(offset *string) ([]kkComps.BasicAuth, *string, error) {
		res, err := sdk.BasicAuthCredentials.ListBasicAuth(ctx,
			kkOps.ListBasicAuthRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list basic-auth credentials: %w", err)
	}
	if snap.HMACAuths, err = collectOffsetPages(func(offset *string) ([]kkComps.HMACAuth, *string, error) {
		res, err := sdk.HMACAuthCredentials.ListHmacAuth(ctx,
			kkOps.ListHmacAuthRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list hmac-auth credentials: %w", err)
	}
	if snap.JWTs, err = collectOffsetPages(func(offset *string) ([]kkComps.Jwt, *string, error) {
		res, err := sdk.JWTs.ListJwt(ctx, kkOps.ListJwtRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list JWT credentials: %w", err)
	}
	if snap.KeyAuths, err = collectOffsetPages(func(offset *string) ([]kkComps.KeyAuth, *string, error) {
		res, err := sdk.APIKeys.ListKeyAuth(ctx,
			kkOps.ListKeyAuthRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list key-auth credentials: %w", err)
	}

	if snap.Certificates, err = collectOffsetPages(func(offset *string) ([]kkComps.Certificate, *string, error) {
		res, err := sdk.Certificates.ListCertificate(ctx,
			kkOps.ListCertificateRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list certificates: %w", err)
	}
	if snap.CACertificates, err = collectOffsetPages(func(offset *string) ([]kkComps.CACertificate, *string, error) {
		res, err := sdk.CACertificates.ListCaCertificate(ctx,
			kkOps.ListCaCertificateRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list CA certificates: %w", err)
	}
	if snap.SNIs, err = collectOffsetPages(func(offset *string) ([]kkComps.Sni, *string, error) {
		res, err := sdk.SNIs.ListSni(ctx, kkOps.ListSniRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list SNIs: %w", err)
	}
	if snap.Vaults, err = collectOffsetPages(func(offset *string) ([]kkComps.Vault, *string, error) {
		res, err := sdk.Vaults.ListVault(ctx, kkOps.ListVaultRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list vaults: %w", err)
	}
	if snap.KeySets, err = collectOffsetPages(func(offset *string) ([]kkComps.KeySet, *string, error) {
		res, err := sdk.KeySets.ListKeySet(ctx,
			kkOps.ListKeySetRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list key sets: %w", err)
	}
	if snap.Keys, err = collectOffsetPages(func(offset *string) ([]kkComps.Key, *string, error) {
		res, err := sdk.Keys.ListKey(ctx, kkOps.ListKeyRequest{ControlPlaneID: cpID, Size: size, Offset: offset})
		if err != nil || res.Object == nil {
			return nil, nil, err
		}
		return res.Object.Data, res.Object.Offset, nil
	}); err != nil {
		return snap, nil, fmt.Errorf("failed to list keys: %w", err)
	}

	dpCerts, err := sdk.DPCertificates.ListDpClientCertificates(ctx, cpID)
	if err != nil {
		return snap, nil, fmt.Errorf("failed to list data plane client certificates: %w", err)
	}
	var warnings []string
	if list := dpCerts.ListDataPlaneCertificatesResponse; list != nil {
		snap.DataPlaneCertificates = list.Items
		// This endpoint is not paginated; report rather than hide a short read.
		if total := list.GetPage().GetTotalCount(); total != nil && *total > int64(len(list.Items)) {
			warnings = append(warnings, fmt.Sprintf(
				"Konnect reported %d data plane client certificates but returned %d; the rest were not exported",
				*total, len(list.Items)))
		}
	}

	return snap, warnings, nil
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
