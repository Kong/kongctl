package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
)

func TestErrUnresolvedRef(t *testing.T) {
	err := errUnresolvedRef(planner.FieldDefaultApplicationStrategyID, "__REF__:default-strategy#id")

	assert.EqualError(
		t,
		err,
		"unresolved reference for default_application_auth_strategy_id: __REF__:default-strategy#id",
	)
}
