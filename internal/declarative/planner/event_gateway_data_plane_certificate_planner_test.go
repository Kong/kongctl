package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
)

func TestShouldUpdateDataPlaneCertificate_NoChanges(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	name := "test-cert"
	description := "Test certificate"
	certificate := "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----"

	current := state.EventGatewayDataPlaneCertificate{
		EventGatewayDataPlaneCertificate: kkComps.EventGatewayDataPlaneCertificate{
			ID:          "cert-123",
			Certificate: certificate,
			Name:        &name,
			Description: &description,
		},
	}

	desired := resources.EventGatewayDataPlaneCertificateResource{
		CreateEventGatewayDataPlaneCertificateRequest: kkComps.CreateEventGatewayDataPlaneCertificateRequest{
			Certificate: certificate,
			Name:        &name,
			Description: &description,
		},
		Ref: "test-cert-ref",
	}

	needsUpdate, updateFields := p.shouldUpdateDataPlaneCertificate(current, desired)
	assert.False(t, needsUpdate)
	assert.Empty(t, updateFields)
}

func TestShouldUpdateDataPlaneCertificate_MultipleChanges(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	currentName := "old-name"
	newName := "new-name"
	currentDesc := "Old description"
	newDesc := "New description"
	currentCert := "-----BEGIN CERTIFICATE-----\nOLD...\n-----END CERTIFICATE-----"
	newCert := "-----BEGIN CERTIFICATE-----\nNEW...\n-----END CERTIFICATE-----"

	current := state.EventGatewayDataPlaneCertificate{
		EventGatewayDataPlaneCertificate: kkComps.EventGatewayDataPlaneCertificate{
			ID:          "cert-123",
			Certificate: currentCert,
			Name:        &currentName,
			Description: &currentDesc,
		},
	}

	desired := resources.EventGatewayDataPlaneCertificateResource{
		CreateEventGatewayDataPlaneCertificateRequest: kkComps.CreateEventGatewayDataPlaneCertificateRequest{
			Certificate: newCert,
			Name:        &newName,
			Description: &newDesc,
		},
		Ref: "test-cert-ref",
	}

	needsUpdate, updateFields := p.shouldUpdateDataPlaneCertificate(current, desired)
	assert.True(t, needsUpdate)
	assert.Equal(t, newCert, updateFields["certificate"])
	assert.Equal(t, newName, updateFields["name"])
	assert.Equal(t, newDesc, updateFields["description"])
}
