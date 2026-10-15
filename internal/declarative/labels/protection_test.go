package labels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsProtectedResource(t *testing.T) {
	for _, tc := range []struct {
		name      string
		labels    map[string]string
		protected bool
	}{
		{name: "nil labels"},
		{name: "managed without protection", labels: map[string]string{NamespaceKey: "default"}},
		{name: "protected", labels: map[string]string{ProtectedKey: TrueValue}, protected: true},
		{name: "explicitly unprotected", labels: map[string]string{ProtectedKey: FalseValue}},
		{name: "empty protection", labels: map[string]string{ProtectedKey: ""}},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.protected, IsProtectedResource(tc.labels)) })
	}
}
