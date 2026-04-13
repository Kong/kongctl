package dcrprovider

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
)

func TestNewDCRProviderCmdAliases(t *testing.T) {
	cmd, err := NewDCRProviderCmd(verbs.Get, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, alias := range []string{"dcr-providers", "dcr-provider", "dcrp", "dcrps", "DCRP", "DCRPS"} {
		if !slices.Contains(cmd.Aliases, alias) {
			t.Fatalf("expected alias %q in %v", alias, cmd.Aliases)
		}
	}
}

func TestNormalizeDCRProviderPreservesFalseActive(t *testing.T) {
	provider, err := normalizeDCRProvider(map[string]any{
		"id":            "d67a4203-b1e8-4631-a626-5fe7c55efe88",
		"name":          "test-okta-dcr-provider",
		"display_name":  "Test Okta DCR Provider",
		"provider_type": "okta",
		"issuer":        "https://example.com",
		"dcr_config": map[string]any{
			"client_id": "client-123",
		},
		"labels": map[string]string{
			"env": "test",
		},
		"active":     false,
		"created_at": "2026-03-13T17:15:08.497Z",
		"updated_at": "2026-03-13T17:15:08.497Z",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if provider.Active == nil || *provider.Active {
		t.Fatalf("expected active=false, got %v", provider.Active)
	}
	if provider.CreatedAt == nil {
		t.Fatal("expected created_at to be parsed")
	}
	if provider.CreatedAt.Format(time.RFC3339Nano) != "2026-03-13T17:15:08.497Z" {
		t.Fatalf("expected created_at to round trip, got %s", provider.CreatedAt.Format(time.RFC3339Nano))
	}
}

func TestDCRProviderDetailView(t *testing.T) {
	active := false
	provider := dcrProvider{
		ID:           "d67a4203-b1e8-4631-a626-5fe7c55efe88",
		Name:         "test-okta-dcr-provider",
		DisplayName:  "Test Okta DCR Provider",
		ProviderType: "okta",
		Issuer:       "https://example.com",
		DCRConfig: map[string]any{
			"token_endpoint": "https://example.com/token",
			"client_id":      "client-123",
		},
		Labels: map[string]string{
			"env": "test",
		},
		Active: &active,
	}

	detail := dcrProviderDetailView(provider)
	for _, expected := range []string{
		"provider_type: okta",
		"issuer: https://example.com",
		"active: false",
		"dcr_config: client_id, token_endpoint",
		"labels: env=test",
	} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("expected detail to contain %q, got:\n%s", expected, detail)
		}
	}
}
