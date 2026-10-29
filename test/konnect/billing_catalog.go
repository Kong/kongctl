package konnect

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

const fixtureStatusField = "status"

// BillingCatalogServer is a loopback API fixture used with the pinned SDK. It
// exposes request bodies so tests verify actual wire contracts and no-op plans.
type BillingCatalogServer struct {
	mu            sync.Mutex
	SDK           *helpers.KonnectSDK
	Objects       map[helpers.BillingKind][]helpers.BillingObject
	Writes        []BillingCatalogWrite
	FailureStatus int
	counter       int
}
type BillingCatalogWrite struct {
	Kind   helpers.BillingKind
	Method string
	ID     string
	Fields map[string]any
}

func NewBillingCatalogServer(t *testing.T) *BillingCatalogServer {
	t.Helper()
	b := &BillingCatalogServer{Objects: map[helpers.BillingKind][]helpers.BillingObject{}}
	server := httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(server.Close)
	b.SDK = &helpers.KonnectSDK{
		SDK:        sdk.New(sdk.WithServerURL(server.URL), sdk.WithClient(server.Client())),
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "test-token",
	}
	return b
}

func (b *BillingCatalogServer) Snapshot(kind helpers.BillingKind) ([]helpers.BillingObject, []BillingCatalogWrite) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, _ := json.Marshal(b.Objects[kind])
	var objects []helpers.BillingObject
	_ = json.Unmarshal(data, &objects)
	return objects, append([]BillingCatalogWrite(nil), b.Writes...)
}

func (b *BillingCatalogServer) Mutate(kind helpers.BillingKind, id string, mutate func(helpers.BillingObject)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, v := range b.Objects[kind] {
		if v.ID() == id {
			mutate(v)
			return
		}
	}
}

func (b *BillingCatalogServer) serve(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	send := func(status int, v any) {
		w.WriteHeader(status)
		if status != 204 {
			if err := json.NewEncoder(w).Encode(v); err != nil {
				panic(err)
			}
		}
	}
	if b.FailureStatus != 0 {
		send(b.FailureStatus, map[string]any{fixtureStatusField: b.FailureStatus, "title": "fixture failure"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v3/openmeter/")
	if r.Method == http.MethodGet && (path == "app-catalog" || strings.HasPrefix(path, "app-catalog/")) {
		item := map[string]any{
			"type":            "sandbox",
			"name":            "Sandbox",
			"description":     "Testing",
			"capabilities":    []any{},
			"install_methods": []any{},
		}
		if path == "app-catalog" {
			send(200, map[string]any{"data": []any{item}, "meta": map[string]any{"page": map[string]any{"total": 1}}})
		} else {
			send(200, item)
		}
		return
	}
	var kind helpers.BillingKind
	var suffix string
	for prefix, k := range map[string]helpers.BillingKind{
		"tax-codes":  helpers.BillingTaxCode,
		"currencies": helpers.BillingCurrency,
		"apps":       helpers.BillingApp,
		"profiles":   helpers.BillingProfile,
		"plans":      helpers.BillingPlan,
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			kind = k
			suffix = strings.TrimPrefix(path, prefix)
			break
		}
	}
	install := path == "app-catalog/install"
	if install {
		kind = helpers.BillingApp
		suffix = ""
	}
	if kind == "" {
		send(404, map[string]any{fixtureStatusField: 404})
		return
	}
	if kind == helpers.BillingCurrency {
		suffix = strings.TrimPrefix(suffix, "/custom")
	}
	id := strings.TrimPrefix(suffix, "/")
	action := ""
	if before, after, ok := strings.Cut(id, "/"); ok {
		id = before
		action = after
	}
	if action == "cost-bases" {
		if r.Method == http.MethodGet {
			send(200, map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"total": 0}}})
			return
		}
		if r.Method == http.MethodPost {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				send(400, nil)
				return
			}
			b.Writes = append(b.Writes, BillingCatalogWrite{kind, r.Method, id, payload})
			payload["id"] = "01G65Z755AFWAKHE12NY0C9999"
			payload["created_at"] = "2026-10-28T00:00:00Z"
			send(201, payload)
			return
		}
	}
	if r.Method == http.MethodGet && id == "" {
		values := b.Objects[kind]
		if values == nil {
			values = []helpers.BillingObject{}
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("page[size]"))
		if size < 1 {
			size = 100
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page[number]"))
		if page < 1 {
			page = 1
		}
		start := min((page-1)*size, len(values))
		end := min(start+size, len(values))
		send(
			200,
			map[string]any{
				"data": values[start:end],
				"meta": map[string]any{"page": map[string]any{"total": len(values), "number": page, "size": size}},
			},
		)
		return
	}
	var payload map[string]any
	if r.Method != http.MethodGet && r.Method != http.MethodDelete && action == "" {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			send(400, map[string]any{fixtureStatusField: 400})
			return
		}
	}
	if r.Method == http.MethodPost && id == "" {
		b.counter++
		id = fmt.Sprintf("01G65Z755AFWAKHE12NY0C%04d", b.counter)
		value := helpers.BillingObject(maps.Clone(payload))
		value["id"] = id
		value["created_at"] = "2026-10-28T00:00:00Z"
		value["updated_at"] = "2026-10-28T00:00:00Z"
		// Scoped billing lifecycle classification; other kinds follow the default path.
		//exhaustive:ignore
		switch kind {
		case helpers.BillingCurrency:
			value["type"] = "custom"
		case helpers.BillingPlan:
			value[fixtureStatusField] = "draft"
			value["version"] = 1
			value["settlement_mode"] = "credit_then_invoice"
		case helpers.BillingApp:
			delete(value, "api_key")
			delete(value, "create_billing_profile")
			value[fixtureStatusField] = "ready"
			value["definition"] = map[string]any{
				"type":            value["type"],
				"name":            "Sandbox",
				"description":     "Test",
				"capabilities":    []any{},
				"install_methods": []any{},
			}
			value["default_for_capability_types"] = []any{}
			switch value["type"] {
			case "stripe":
				value["account_id"] = "acct_test"
				value["livemode"] = false
				value["masked_api_key"] = "***"
			case "external_invoicing":
				value["enable_draft_sync_hook"] = false
				value["enable_issuing_sync_hook"] = false
			}
		}
		b.Objects[kind] = append(b.Objects[kind], value)
		b.Writes = append(b.Writes, BillingCatalogWrite{kind, r.Method, id, payload})
		send(201, value)
		return
	}
	for i, value := range b.Objects[kind] {
		if value.ID() != id {
			continue
		}
		switch r.Method {
		case http.MethodGet:
			send(200, value)
		case http.MethodDelete:
			b.Objects[kind] = append(b.Objects[kind][:i], b.Objects[kind][i+1:]...)
			b.Writes = append(b.Writes, BillingCatalogWrite{kind, r.Method, id, nil})
			send(204, nil)
		case http.MethodPut, http.MethodPatch:
			maps.Copy(value, payload)
			b.Writes = append(b.Writes, BillingCatalogWrite{kind, r.Method, id, payload})
			send(200, value)
		case http.MethodPost:
			switch action {
			case "publish":
				value[fixtureStatusField] = "active"
			case "archive":
				value[fixtureStatusField] = "archived"
			default:
				send(400, map[string]any{fixtureStatusField: 400})
				return
			}
			b.Writes = append(b.Writes, BillingCatalogWrite{kind, r.Method, id, map[string]any{"action": action}})
			send(200, value)
		default:
			send(405, map[string]any{fixtureStatusField: 405})
		}
		return
	}
	send(404, map[string]any{fixtureStatusField: 404, "title": "not found"})
}
