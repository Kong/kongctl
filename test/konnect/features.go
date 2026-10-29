package konnect

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

const FeatureID = "01G65Z755AFWAKHE12NY0CQ9FH"

// FeatureServer exercises the SDK's real request serialization against a local API.
type FeatureServer struct {
	mu            sync.Mutex
	Server        *httptest.Server
	SDK           *helpers.KonnectSDK
	Features      []kkComps.Feature
	Writes        []map[string]any
	FailureStatus int
}

func NewFeatureServer(t *testing.T, features ...kkComps.Feature) *FeatureServer {
	t.Helper()
	m := &FeatureServer{Features: features}
	m.Server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.Server.Close)
	m.SDK = &helpers.KonnectSDK{
		SDK:        kkSDK.New(kkSDK.WithServerURL(m.Server.URL), kkSDK.WithClient(m.Server.Client())),
		BaseURL:    m.Server.URL,
		HTTPClient: m.Server.Client(),
		Token:      "test-token",
	}
	return m
}

func (m *FeatureServer) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	send := func(value any) {
		if err := json.NewEncoder(w).Encode(value); err != nil {
			panic(err)
		}
	}
	if m.FailureStatus != 0 {
		w.WriteHeader(m.FailureStatus)
		send(
			map[string]any{
				fixtureStatusField: m.FailureStatus,
				"title":            "feature fixture failure",
				"detail":           "feature fixture failure",
			},
		)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v3/openmeter/plans" {
		send(map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"total": 0}}})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v3/openmeter/features")
	if path == r.URL.Path {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet && path == "" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page[number]"))
		size, _ := strconv.Atoi(r.URL.Query().Get("page[size]"))
		if page < 1 {
			page = 1
		}
		if size < 1 {
			size = 100
		}
		start := min((page-1)*size, len(m.Features))
		end := min(start+size, len(m.Features))
		send(
			kkComps.FeaturePagePaginatedResponse{
				Data: m.Features[start:end],
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(m.Features))}},
			},
		)
		return
	}
	var payload map[string]any
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodDelete {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m.Writes = append(m.Writes, payload)
	}
	if r.Method == http.MethodPost && path == "" {
		data, _ := json.Marshal(payload)
		var feature kkComps.Feature
		if err := json.Unmarshal(data, &feature); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		feature.ID = FeatureID
		m.Features = append(m.Features, feature)
		w.WriteHeader(http.StatusCreated)
		send(feature)
		return
	}
	id := strings.TrimPrefix(strings.TrimSuffix(path, "/cost/query"), "/")
	index := slices.IndexFunc(m.Features, func(v kkComps.Feature) bool { return v.ID == id })
	if index < 0 {
		w.WriteHeader(http.StatusNotFound)
		send(map[string]any{fixtureStatusField: 404, "title": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		send(m.Features[index])
	case http.MethodPatch:
		for key := range payload {
			if key != "unit_cost" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if value, present := payload["unit_cost"]; present {
			data, _ := json.Marshal(value)
			m.Features[index].UnitCost = nil
			if err := json.Unmarshal(data, &m.Features[index].UnitCost); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		send(m.Features[index])
	case http.MethodDelete:
		m.Features = slices.Delete(m.Features, index, index+1)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		send(map[string]any{"data": []any{}, "meta": map[string]any{}})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *FeatureServer) Snapshot() ([]kkComps.Feature, []map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	values := slices.Clone(m.Features)
	writes := make([]map[string]any, len(m.Writes))
	for i, w := range m.Writes {
		writes[i] = maps.Clone(w)
	}
	return values, writes
}

func (m *FeatureServer) Fail(status int) { m.mu.Lock(); defer m.mu.Unlock(); m.FailureStatus = status }

func (m *FeatureServer) SetLabels(id string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.Features {
		if m.Features[i].ID == id {
			m.Features[i].Labels = labels
			return
		}
	}
	panic(fmt.Sprintf("missing fixture feature %s", id))
}
