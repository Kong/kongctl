package portal

import (
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestPortalTeamDetailView_UsesAPIFieldLabels(t *testing.T) {
	id := "team-id"
	name := "Developers"
	description := "Portal developer team"
	createdAt := time.Date(2026, time.April, 25, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	detail := portalTeamDetailView(kkComps.PortalTeamResponse{
		ID:          &id,
		Name:        &name,
		Description: &description,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
	})

	for _, expected := range []string{
		"name: Developers",
		"id: team-id",
		"description: Portal developer team",
		"created_at:",
		"updated_at:",
	} {
		require.Contains(t, detail, expected)
	}

	for _, oldLabel := range []string{"Name:", "ID:", "Description:", "Created:", "Updated:"} {
		require.NotContains(t, detail, oldLabel)
	}
}

func TestPortalDeveloperDetailView_UsesAPIFieldLabels(t *testing.T) {
	now := time.Date(2026, time.April, 25, 12, 0, 0, 0, time.UTC)

	detail := portalDeveloperDetailView(kkComps.PortalDeveloper{
		ID:        "developer-id",
		Email:     "developer@example.com",
		FullName:  "Portal Developer",
		Status:    kkComps.DeveloperStatusApproved,
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	})

	for _, expected := range []string{
		"email: developer@example.com",
		"id: developer-id",
		"full_name: Portal Developer",
		"status: approved",
		"created_at:",
		"updated_at:",
	} {
		require.Contains(t, detail, expected)
	}

	for _, oldLabel := range []string{"Email:", "ID:", "Full Name:", "Status:", "Created:", "Updated:"} {
		require.NotContains(t, detail, oldLabel)
	}
}

func TestPortalTeamDeveloperDetailView_UsesAPIFieldLabels(t *testing.T) {
	id := "developer-id"
	email := "developer@example.com"
	fullName := "Portal Developer"
	active := true
	createdAt := time.Date(2026, time.April, 25, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	detail := portalTeamDeveloperDetailView(kkComps.BasicDeveloper{
		ID:        &id,
		Email:     &email,
		FullName:  &fullName,
		Active:    &active,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	})

	for _, expected := range []string{
		"email: developer@example.com",
		"id: developer-id",
		"full_name: Portal Developer",
		"active: true",
		"created_at:",
		"updated_at:",
	} {
		require.Contains(t, detail, expected)
	}

	for _, oldLabel := range []string{"Email:", "ID:", "Full Name:", "Active:", "Created:", "Updated:"} {
		require.NotContains(t, detail, oldLabel)
	}
}
