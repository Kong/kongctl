package api

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func init() {
	tableview.RegisterChildLoader("api", "documents", loadAPIDocuments)
}

func loadAPIDocuments(ctx context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	api, ok := parent.(*kkComps.APIResponseSchema)
	if !ok {
		return tableview.ChildView{}, fmt.Errorf("unexpected parent type %T", parent)
	}
	if api == nil || api.ID == "" {
		return tableview.ChildView{}, fmt.Errorf("api identifier is missing")
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

	apiDocAPI := sdk.GetAPIDocumentAPI()
	if apiDocAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("api documents client is not available")
	}

	docs, err := fetchDocumentSummaries(helper, apiDocAPI, api.ID)
	if err != nil {
		return tableview.ChildView{}, err
	}

	flattened := flattenDocuments(docs)

	rows := make([]table.Row, 0, len(flattened))
	for _, doc := range flattened {
		record := documentSummaryToRecord(doc)
		rows = append(rows, table.Row{record.ID, record.Title})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(flattened) {
			return ""
		}
		return documentSummaryDetailView(&flattened[index])
	}

	return tableview.ChildView{
		Headers:        []string{"DOCUMENT", "TITLE"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Documents",
	}, nil
}
