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

const MeterID = "01G65Z755AFWAKHE12NY0CQ9FH"

// MeterServer exercises the SDK's real request serialization against a local API.
type MeterServer struct {
	mu            sync.Mutex
	Server        *httptest.Server
	SDK           *helpers.KonnectSDK
	Meters        []kkComps.Meter
	Writes        []map[string]any
	FailureStatus int
}

func NewMeterServer(t *testing.T, meters ...kkComps.Meter) *MeterServer {
	t.Helper()
	m := &MeterServer{Meters: meters}
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

func (m *MeterServer) serve(w http.ResponseWriter, r *http.Request) {
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
				"title":            "meter fixture failure",
				"detail":           "meter fixture failure",
			},
		)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v3/openmeter/meters")
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
		start := min((page-1)*size, len(m.Meters))
		end := min(start+size, len(m.Meters))
		send(
			kkComps.MeterPagePaginatedResponse{
				Data: m.Meters[start:end],
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(m.Meters))}},
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
		var meter kkComps.Meter
		if err := json.Unmarshal(data, &meter); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		meter.ID = MeterID
		m.Meters = append(m.Meters, meter)
		w.WriteHeader(http.StatusCreated)
		send(meter)
		return
	}
	id := strings.TrimPrefix(strings.TrimSuffix(path, "/query"), "/")
	index := slices.IndexFunc(m.Meters, func(v kkComps.Meter) bool { return v.ID == id })
	if index < 0 {
		w.WriteHeader(http.StatusNotFound)
		send(map[string]any{fixtureStatusField: 404, "title": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		send(m.Meters[index])
	case http.MethodPut:
		data, _ := json.Marshal(payload)
		if err := json.Unmarshal(data, &m.Meters[index]); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if value, ok := payload["dimensions"]; ok {
			data, _ := json.Marshal(value)
			m.Meters[index].Dimensions = nil
			if err := json.Unmarshal(data, &m.Meters[index].Dimensions); err != nil {
				panic(err)
			}
		}
		if value, ok := payload["labels"]; ok {
			data, _ := json.Marshal(value)
			m.Meters[index].Labels = nil
			if err := json.Unmarshal(data, &m.Meters[index].Labels); err != nil {
				panic(err)
			}
		}
		send(m.Meters[index])
	case http.MethodDelete:
		m.Meters = slices.Delete(m.Meters, index, index+1)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		send(map[string]any{"data": []any{}, "meta": map[string]any{}})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *MeterServer) Snapshot() ([]kkComps.Meter, []map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	values := slices.Clone(m.Meters)
	writes := make([]map[string]any, len(m.Writes))
	for i, w := range m.Writes {
		writes[i] = maps.Clone(w)
	}
	return values, writes
}

func (m *MeterServer) Fail(status int) { m.mu.Lock(); defer m.mu.Unlock(); m.FailureStatus = status }

func (m *MeterServer) SetLabels(id string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.Meters {
		if m.Meters[i].ID == id {
			m.Meters[i].Labels = labels
			return
		}
	}
	panic(fmt.Sprintf("missing fixture meter %s", id))
}
