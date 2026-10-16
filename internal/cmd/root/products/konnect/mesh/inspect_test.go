package mesh

import "testing"

func TestOverviewStatus(t *testing.T) {
	tests := map[string]struct {
		item map[string]any
		want string
	}{
		"dataplane with an open subscription is online": {
			item: map[string]any{"dataplaneInsight": map[string]any{
				"subscriptions": []any{map[string]any{
					"connectTime": "2026-09-11T10:16:08Z", "disconnectTime": nil,
				}},
			}},
			want: "Online",
		},
		"dataplane whose last subscription closed is offline": {
			item: map[string]any{"dataplaneInsight": map[string]any{
				"subscriptions": []any{map[string]any{
					"connectTime": "2026-09-11T10:16:08Z", "disconnectTime": "2026-09-11T11:00:00Z",
				}},
			}},
			want: "Offline",
		},
		"an earlier open subscription still counts as online": {
			item: map[string]any{"dataplaneInsight": map[string]any{
				"subscriptions": []any{
					map[string]any{"connectTime": "1", "disconnectTime": nil},
					map[string]any{"connectTime": "2", "disconnectTime": "3"},
				},
			}},
			want: "Online",
		},
		"zones report through zoneInsight": {
			item: map[string]any{"zoneInsight": map[string]any{
				"subscriptions": []any{map[string]any{
					"connectTime": "1", "disconnectTime": "2",
				}},
			}},
			want: "Offline",
		},
		"no subscriptions at all reports offline": {
			item: map[string]any{"dataplaneInsight": map[string]any{"subscriptions": []any{}}},
			want: "Offline",
		},
		"no insight reports nothing rather than guessing": {
			item: map[string]any{"name": "dp-1"},
			want: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := overviewStatus(tc.item); got != tc.want {
				t.Fatalf("overviewStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJoinOrigins(t *testing.T) {
	tests := map[string]struct {
		origins any
		want    string
	}{
		"names are preferred": {
			origins: []any{
				map[string]any{"name": "allow-all", "kri": "kri_mtp_default___allow-all_"},
				map[string]any{"name": "deny-legacy"},
			},
			want: "allow-all, deny-legacy",
		},
		"a KRI stands in when there is no name": {
			origins: []any{map[string]any{"kri": "kri_mtp_default___allow-all_"}},
			want:    "kri_mtp_default___allow-all_",
		},
		"entries carrying neither are skipped": {
			origins: []any{map[string]any{"mesh": "default"}, map[string]any{"name": "keep"}},
			want:    "keep",
		},
		"a non-list is not an origin list": {
			origins: "allow-all",
			want:    "",
		},
		"no origins renders empty": {
			origins: []any{},
			want:    "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := joinOrigins(tc.origins); got != tc.want {
				t.Fatalf("joinOrigins() = %q, want %q", got, tc.want)
			}
		})
	}
}
