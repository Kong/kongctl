package dump

import (
	"context"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
)

var controlPlaneChildCollectors = buildControlPlaneChildCollectors()

func buildControlPlaneChildCollectors() []childCollector[declresources.ControlPlaneResource] {
	collectors := []childCollector[declresources.ControlPlaneResource]{
		childCollection(
			"failed to load data plane certificates",
			func(ctx context.Context, d *childDumpContext) ([]declresources.ControlPlaneDataPlaneCertificateResource, error) {
				return buildDataPlaneCertificates(ctx, d.client, d.parentID)
			},
			func(cp *declresources.ControlPlaneResource) *[]declresources.ControlPlaneDataPlaneCertificateResource {
				return &cp.DataPlaneCertificates
			},
		),
	}
	if err := validateChildCollectors("control plane child dump", collectors); err != nil {
		panic(err)
	}
	return collectors
}
