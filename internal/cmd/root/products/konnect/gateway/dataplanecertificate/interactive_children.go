package dataplanecertificate

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func init() {
	tableview.RegisterChildLoader(
		kkCommon.ViewParentControlPlane,
		kkCommon.ViewFieldDataPlaneCertificates,
		loadControlPlaneDataPlaneCertificates,
	)
}

func loadControlPlaneDataPlaneCertificates(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
	controlPlaneID, err := dataPlaneCertificateControlPlaneIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	certAPI := sdk.GetDataPlaneCertificateAPI()
	if certAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("data plane certificates client not configured")
	}

	res, err := certAPI.ListDpClientCertificates(helper.GetContext(), controlPlaneID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list data plane certificates",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	var certs []kkComps.DataPlaneClientCertificate
	if res != nil && res.GetListDataPlaneCertificatesResponse() != nil {
		certs = res.GetListDataPlaneCertificatesResponse().GetItems()
	}

	rows := make([]table.Row, 0, len(certs))
	for i := range certs {
		record := dataPlaneCertificateToRecord(&certs[i])
		rows = append(rows, table.Row{record.ID, record.Fingerprint})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(certs) {
			return ""
		}
		return dataPlaneCertificateDetailView(&certs[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "FINGERPRINT"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Data Plane Certificates",
		ParentType:     kkCommon.ViewParentDataPlaneCertificate,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(certs) {
				return nil
			}
			return &certs[index]
		},
	}, nil
}

func dataPlaneCertificateControlPlaneIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("control plane parent is nil")
	}

	switch cp := parent.(type) {
	case *kkComps.ControlPlane:
		id := strings.TrimSpace(cp.ID)
		if id == "" {
			return "", fmt.Errorf("control plane identifier is missing")
		}
		return id, nil
	case kkComps.ControlPlane:
		id := strings.TrimSpace(cp.ID)
		if id == "" {
			return "", fmt.Errorf("control plane identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}
