package decerrors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAlreadyExistsError(t *testing.T) {
	t.Run("matches unique validation payload", func(t *testing.T) {
		err := errors.New(
			`{"status":400,"invalid_parameters":[{"field":"name","reason":"name: \"My Simple Portal\" ` +
				`already exists and must be unique.","rule":"unique","source":"body"}]}`,
		)
		assert.True(t, IsAlreadyExistsError(err, 400))
	})

	t.Run("matches conflict status", func(t *testing.T) {
		err := errors.New("conflict")
		assert.True(t, IsAlreadyExistsError(err, 409))
	})

	t.Run("ignores unrelated validation errors", func(t *testing.T) {
		err := errors.New(
			`{"status":400,"detail":"Invalid Parameters","invalid_parameters":[{"field":"name","reason":"is required"}]}`,
		)
		assert.False(t, IsAlreadyExistsError(err, 400))
	})
}

func TestExtractStatusCodeFromError(t *testing.T) {
	err := errors.New(`request failed: {"status":400,"detail":"Invalid Parameters"}`)
	assert.Equal(t, 400, ExtractStatusCodeFromError(err))
}
