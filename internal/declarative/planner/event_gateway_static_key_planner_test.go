package planner

import (
	"log/slog"
	"os"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
)

// newTestStaticKeyPlanner returns a minimal Planner suitable for unit tests.
func newTestStaticKeyPlanner() *Planner {
	return &Planner{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// ---------------------------------------------------------------------------
// doesStaticKeyNeedChange
// ---------------------------------------------------------------------------

func TestDoesStaticKeyNeedChange_NoChanges(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	desc := "a description"
	val := "secret"
	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:          "key-123",
			Name:        "my-key",
			Description: &desc,
			Labels:      map[string]string{"env": "prod"},
			Value:       &val,
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:        "my-key",
			Value:       "secret",
			Description: &desc,
			Labels:      map[string]string{"env": "prod"},
		},
		Ref: "my-key-ref",
	}

	assert.False(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_DescriptionChanged(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	oldDesc := "old description"
	newDesc := "new description"
	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:          "key-123",
			Name:        "my-key",
			Description: &oldDesc,
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:        "my-key",
			Value:       "secret",
			Description: &newDesc,
		},
		Ref: "my-key-ref",
	}

	assert.True(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_LabelsChanged(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:     "key-123",
			Name:   "my-key",
			Labels: map[string]string{"env": "staging"},
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:   "my-key",
			Value:  "secret",
			Labels: map[string]string{"env": "prod"},
		},
		Ref: "my-key-ref",
	}

	assert.True(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_ValueChanged(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	oldVal := "old-secret"
	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:    "key-123",
			Name:  "my-key",
			Value: &oldVal,
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:  "my-key",
			Value: "new-secret",
		},
		Ref: "my-key-ref",
	}

	assert.True(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_VaultRefChanged(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	oldRef := `${env["OLD_KEY"]}`
	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:    "key-123",
			Name:  "my-key",
			Value: &oldRef,
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:  "my-key",
			Value: `${env["NEW_KEY"]}`,
		},
		Ref: "my-key-ref",
	}

	assert.True(t, p.doesStaticKeyNeedChange(current, desired))
}
