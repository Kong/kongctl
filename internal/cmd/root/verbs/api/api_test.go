package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyJQFilter(t *testing.T) {
	body := []byte(`{"foo":{"bar":1},"list":[1,2,3],"str":"value"}`)

	out, err := applyJQFilter(body, ".foo.bar")
	require.NoError(t, err)
	require.JSONEq(t, "1", string(out))

	out, err = applyJQFilter(body, ".list[1]")
	require.NoError(t, err)
	require.JSONEq(t, "2", string(out))

	out, err = applyJQFilter(body, ".str")
	require.NoError(t, err)
	require.JSONEq(t, `"value"`, string(out))

	_, err = applyJQFilter([]byte("not-json"), ".foo")
	require.Error(t, err)

	_, err = applyJQFilter(body, ".list[")
	require.Error(t, err)

	_, err = applyJQFilter(body, ".missing")
	require.Error(t, err)
}
