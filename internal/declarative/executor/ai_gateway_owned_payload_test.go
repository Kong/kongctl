package executor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingOwnedChildMarshaler struct {
	cause error
}

func (m failingOwnedChildMarshaler) MarshalJSON() ([]byte, error) {
	return nil, m.cause
}

func TestOwnedChildPayloadErrorsDoNotExposeCredentials(t *testing.T) {
	const credential = "synthetic-credential-must-not-appear"
	cause := errors.New("invalid credential: " + credential)
	for _, tc := range []struct {
		name, message string
		request       any
	}{
		{
			name: "custom marshaler error", message: "cannot encode AI Gateway child payload",
			request: failingOwnedChildMarshaler{cause: cause},
		},
		{
			name: "non-object JSON", message: "cannot decode AI Gateway child payload",
			request: credential,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := ownedChildPayload(tc.request)
			require.Nil(t, payload)
			require.EqualError(t, err, tc.message)
			require.NotContains(t, err.Error(), credential)
			require.NotErrorIs(t, err, cause)
		})
	}
}
