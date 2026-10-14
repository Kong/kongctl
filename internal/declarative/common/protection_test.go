package common

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/require"
)

func TestGetProtectionStatus(t *testing.T) {
	for _, tc := range []struct {
		name      string
		labels    map[string]string
		protected bool
	}{
		{name: "nil labels"},
		{name: "managed without protection", labels: map[string]string{labels.NamespaceKey: "default"}},
		{name: "protected", labels: map[string]string{labels.ProtectedKey: labels.TrueValue}, protected: true},
		{name: "explicitly unprotected", labels: map[string]string{labels.ProtectedKey: labels.FalseValue}},
		{name: "empty protection", labels: map[string]string{labels.ProtectedKey: ""}},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.protected, GetProtectionStatus(tc.labels)) })
	}
}
