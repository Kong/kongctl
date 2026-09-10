package planner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObservationCacheList(t *testing.T) {
	t.Parallel()

	type observation struct {
		namespace string
		id        string
	}
	type response struct {
		values []observation
		err    error
	}
	type step struct {
		query []string
		want  []observation
		err   error
		calls int
	}

	all := []observation{{"b", "b-1"}, {"a", "a-1"}, {"a", "a-2"}}
	aOnly := all[1:]
	bOnly := all[:1]
	empty := []observation{}
	fetchErr := errors.New("observation failed")

	testCases := []struct {
		name         string
		uncached     bool
		fanout       bool
		responses    []response
		steps        []step
		wantRequests [][]string
	}{
		{
			name: "empty queries skip fetching",
			steps: []step{
				{query: nil, want: empty},
				{query: []string{}, want: empty},
				{query: []string{" ", ""}, want: empty},
			},
		},
		{
			name:      "equivalent queries reuse observations",
			responses: []response{{values: all}},
			steps: []step{
				{query: []string{"b", " a ", "b", ""}, want: all, calls: 1},
				{query: []string{"a", "b"}, want: all, calls: 1},
			},
			wantRequests: [][]string{{"a", "b"}},
		},
		{
			name:      "different namespaces fetch separately",
			responses: []response{{values: aOnly}, {values: bOnly}},
			steps: []step{
				{query: []string{"a"}, want: aOnly, calls: 1},
				{query: []string{"b"}, want: bOnly, calls: 2},
				{query: []string{"a"}, want: aOnly, calls: 2},
			},
			wantRequests: [][]string{{"a"}, {"b"}},
		},
		{
			name:      "nil success is cached",
			responses: []response{{values: nil}},
			steps: []step{
				{query: []string{"a"}, want: nil, calls: 1},
				{query: []string{"a"}, want: nil, calls: 1},
			},
			wantRequests: [][]string{{"a"}},
		},
		{
			name:      "empty success is cached",
			responses: []response{{values: empty}},
			steps: []step{
				{query: []string{"a"}, want: empty, calls: 1},
				{query: []string{"a"}, want: empty, calls: 1},
			},
			wantRequests: [][]string{{"a"}},
		},
		{
			name:      "failed observation is discarded and retried",
			responses: []response{{values: aOnly, err: fetchErr}, {values: aOnly}},
			steps: []step{
				{query: []string{"a"}, want: nil, err: fetchErr, calls: 1},
				{query: []string{"a"}, want: aOnly, calls: 2},
				{query: []string{"a"}, want: aOnly, calls: 2},
			},
			wantRequests: [][]string{{"a"}, {"a"}},
		},
		{
			name:      "wildcard observations are reused and filtered in order",
			responses: []response{{values: all}},
			steps: []step{
				{query: []string{"b", " * ", "a"}, want: all, calls: 1},
				{query: []string{"a"}, want: aOnly, calls: 1},
				{query: []string{"missing"}, want: empty, calls: 1},
				{query: []string{"b"}, want: bOnly, calls: 1},
				{query: []string{"*"}, want: all, calls: 1},
			},
			wantRequests: [][]string{{"*"}},
		},
		{
			name:      "empty wildcard observation is loaded",
			responses: []response{{values: empty}},
			steps: []step{
				{query: []string{"*"}, want: empty, calls: 1},
				{query: []string{"a"}, want: empty, calls: 1},
			},
			wantRequests: [][]string{{"*"}},
		},
		{
			name:      "nil wildcard observation filters to an empty slice",
			responses: []response{{values: nil}},
			steps: []step{
				{query: []string{"*"}, want: nil, calls: 1},
				{query: []string{"a"}, want: empty, calls: 1},
			},
			wantRequests: [][]string{{"*"}},
		},
		{
			name:      "fanout filters and reuses the wildcard fetch",
			fanout:    true,
			responses: []response{{values: all}, {values: all}},
			steps: []step{
				{query: []string{"a"}, want: aOnly, calls: 1},
				{query: []string{"b"}, want: bOnly, calls: 1},
				{query: []string{"a"}, want: aOnly, calls: 1},
				{query: []string{"*"}, want: all, calls: 2},
				{query: []string{"*"}, want: all, calls: 2},
			},
			wantRequests: [][]string{{"*"}, {"*"}},
		},
		{
			name:      "nil cache fetches each time",
			uncached:  true,
			responses: []response{{values: aOnly}, {values: aOnly}},
			steps: []step{
				{query: []string{"a"}, want: aOnly, calls: 1},
				{query: []string{"a"}, want: aOnly, calls: 2},
			},
			wantRequests: [][]string{{"a"}, {"a"}},
		},
		{
			name:      "nil cache fanout returns unfiltered observations",
			uncached:  true,
			fanout:    true,
			responses: []response{{values: all}, {values: all}},
			steps: []step{
				{query: []string{"a"}, want: all, calls: 1},
				{query: []string{"b"}, want: all, calls: 2},
				{query: []string{" "}, want: empty, calls: 2},
			},
			wantRequests: [][]string{{"*"}, {"*"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cache := newObservationCache[observation]()
			activeCache := &cache
			if tc.uncached {
				activeCache = nil
			}

			var requests [][]string
			fetch := func(_ context.Context, namespaces []string) ([]observation, error) {
				require.Less(t, len(requests), len(tc.responses), "unexpected fetch for %v", namespaces)
				result := tc.responses[len(requests)]
				requests = append(requests, namespaces)
				return result.values, result.err
			}

			for i, step := range tc.steps {
				got, err := activeCache.list(t.Context(), step.query, tc.fanout, fetch,
					func(value observation) string { return value.namespace })
				require.ErrorIs(t, err, step.err, "step %d", i)
				require.Equal(t, step.want, got, "step %d", i)
				require.Len(t, requests, step.calls, "step %d", i)
			}
			require.Equal(t, tc.wantRequests, requests)
		})
	}
}
