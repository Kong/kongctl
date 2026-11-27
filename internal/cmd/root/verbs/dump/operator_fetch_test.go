package dump

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kongctl/internal/operatorexport"
)

const (
	fakeCPID       = "11111111-0000-4000-8000-000000000001"
	fakeServiceID  = "22222222-0000-4000-8000-000000000002"
	fakeGroupID    = "33333333-0000-4000-8000-000000000003"
	fakeConsumerID = "44444444-0000-4000-8000-000000000004"
	fakeUpstreamID = "55555555-0000-4000-8000-000000000005"
)

// fakeKonnect serves canned core entity listings keyed by path suffix and
// fails the test on any path it does not know, so the test pins exactly which
// endpoints the fetch layer calls.
func fakeKonnect(t *testing.T, responses map[string]string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var requested []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.URL.Path)
		mu.Unlock()

		prefix := "/v2/control-planes/" + fakeCPID + "/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		rest = strings.TrimPrefix(rest, "core-entities/")
		// The SDK fills the unset workspace path segment with "default".
		rest = strings.TrimPrefix(rest, "default/")
		body, ok := responses[strings.Trim(rest, "/")]
		if !ok {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), requested...)
	}
}

func TestFetchOperatorSnapshotReadsEveryAdoptableEntity(t *testing.T) {
	empty := `{"data":[]}`
	responses := map[string]string{
		"services": `{"data":[{"id":"` + fakeServiceID + `","name":"svc","host":"example.com"}]}`,
		"routes": `{"data":[
			{"id":"r1","name":"json-route","paths":["/a"],"service":{"id":"` + fakeServiceID + `"}},
			{"id":"r2","name":"expr-route","expression":"http.path == \"/b\""}
		]}`,
		"plugins":         `{"data":[{"id":"p1","name":"cors","config":{"origins":null}}]}`,
		"consumers":       `{"data":[{"id":"` + fakeConsumerID + `","username":"alice"}]}`,
		"consumer_groups": `{"data":[{"id":"` + fakeGroupID + `","name":"gold"}]}`,
		"consumer_groups/" + fakeGroupID + "/consumers": `{"data":[{"id":"` +
			fakeConsumerID + `","username":"alice"}]}`,
		"upstreams": `{"data":[{"id":"` + fakeUpstreamID + `","name":"up"}]}`,
		"upstreams/" + fakeUpstreamID + "/targets": `{"data":[{"id":"t1","target":"10.0.0.1:80","weight":100}]}`,
		"acls":                   `{"data":[{"id":"a1","group":"admins","consumer":{"id":"` + fakeConsumerID + `"}}]}`,
		"basic-auths":            empty,
		"hmac-auths":             empty,
		"jwts":                   empty,
		"key-auths":              empty,
		"certificates":           empty,
		"ca_certificates":        empty,
		"snis":                   empty,
		"vaults":                 empty,
		"key-sets":               empty,
		"keys":                   empty,
		"dp-client-certificates": `{"items":[{"id":"d1","cert":"CERT"}],"page":{"total_count":2}}`,
	}
	server, requested := fakeKonnect(t, responses)

	sdk := kk.New(
		kk.WithServerURL(server.URL),
		kk.WithSecurity(kkComps.Security{PersonalAccessToken: new("kpat_test")}),
	)
	cp := operatorexport.ControlPlane{ID: fakeCPID, Name: "cp", ClusterType: "CLUSTER_TYPE_CONTROL_PLANE"}

	snap, warnings, err := fetchOperatorSnapshot(t.Context(), sdk, cp, 100)
	require.NoError(t, err)

	assert.Len(t, snap.Services, 1)
	require.Len(t, snap.Routes, 2)
	assert.NotNil(t, snap.Routes[0].RouteJSON, "JSON routes must decode as RouteJSON")
	assert.NotNil(t, snap.Routes[1].RouteExpression, "expression routes must decode as RouteExpression")
	assert.Len(t, snap.Plugins, 1)
	assert.Equal(t, map[string][]string{fakeGroupID: {fakeConsumerID}}, snap.ConsumerGroupMembers)
	assert.Len(t, snap.Targets, 1)
	assert.Len(t, snap.ACLs, 1)
	assert.Len(t, snap.DataPlaneCertificates, 1)

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "reported 2 data plane client certificates but returned 1")

	// The snapshot must build cleanly end to end.
	result, err := operatorexport.Build(snap, operatorexport.Options{
		Namespace: "kong", AuthRef: "auth", AdoptMode: operatorexport.AdoptModeMatch,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Manifests)

	for path := range responses {
		found := false
		for _, got := range requested() {
			if strings.HasSuffix(strings.Trim(got, "/"), path) {
				found = true
				break
			}
		}
		assert.True(t, found, "expected a request to %s", path)
	}
}
