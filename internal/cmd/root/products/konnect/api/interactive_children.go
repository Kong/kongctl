package api

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/util"
)

func init() {
	tableview.RegisterChildLoader("api", "documents", loadAPIDocuments)
	tableview.RegisterChildLoader("api", "versions", loadAPIVersions)
	tableview.RegisterChildLoader("api", "publications", loadAPIPublications)
	tableview.RegisterChildLoader("api", "implementations", loadAPIImplementations)
	tableview.RegisterChildLoader("api-document", "content", loadAPIDocumentContent)
	tableview.RegisterChildLoader("api-version", "spec", loadAPIVersionSpec)
}

func loadAPIDocuments(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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
	detailCache := newAPIDocumentDetailCache()

	rows := make([]table.Row, 0, len(flattened))
	for _, doc := range flattened {
		record := documentSummaryToRecord(doc)
		rows = append(rows, table.Row{record.ID, record.Title})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(flattened) {
			return ""
		}
		summary := &flattened[index]
		if detail, ok := detailCache.Get(summary.ID); ok {
			return documentSummaryDetailView(summary, &detail)
		}
		return documentSummaryDetailView(summary, nil)
	}

	return tableview.ChildView{
		Headers:        []string{"DOCUMENT", "TITLE"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Documents",
		ParentType:     "api-document",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(flattened) {
				return nil
			}
			doc := flattened[index]
			return &apiDocumentContentContext{
				apiID: api.ID,
				docID: strings.TrimSpace(doc.ID),
				cache: detailCache,
			}
		},
	}, nil
}

func loadAPIVersions(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	versionAPI := sdk.GetAPIVersionAPI()
	if versionAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("api versions client is not available")
	}

	summaries, err := fetchVersionSummaries(helper, versionAPI, api.ID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	detailCache := newAPIVersionDetailCache()
	rows := make([]table.Row, 0, len(summaries))
	for i := range summaries {
		record := versionSummaryToRecord(summaries[i])
		rows = append(rows, table.Row{util.AbbreviateUUID(record.ID), record.Version})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(summaries) {
			return ""
		}
		summary := &summaries[index]
		if detail, ok := detailCache.Get(summary.GetID()); ok {
			return versionSummaryDetailView(summary, &detail)
		}
		return versionSummaryDetailView(summary, nil)
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "VERSION"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Versions",
		ParentType:     "api-version",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(summaries) {
				return nil
			}
			summary := summaries[index]
			return &apiVersionContentContext{
				apiID:   api.ID,
				version: strings.TrimSpace(summary.GetID()),
				cache:   detailCache,
			}
		},
	}, nil
}

func loadAPIPublications(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	publicationAPI := sdk.GetAPIPublicationAPI()
	if publicationAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("api publications client is not available")
	}

	publications, err := fetchPublications(helper, publicationAPI, api.ID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	rows := make([]table.Row, 0, len(publications))
	for i := range publications {
		record := publicationToRecord(publications[i])
		upperVisibility := strings.ToUpper(record.Visibility)
		rows = append(rows, table.Row{util.AbbreviateUUID(record.PortalID), upperVisibility})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(publications) {
			return ""
		}
		return publicationDetailView(&publications[index])
	}

	return tableview.ChildView{
		Headers:        []string{"PORTAL", "VISIBILITY"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Publications",
		ParentType:     "api-publication",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(publications) {
				return nil
			}
			return &publications[index]
		},
	}, nil
}

func loadAPIImplementations(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	implementationAPI := sdk.GetAPIImplementationAPI()
	if implementationAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("api implementations client is not available")
	}

	implementations, err := fetchImplementations(helper, implementationAPI, api.ID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	rows := make([]table.Row, len(implementations))
	for i := range implementations {
		record := implementationToRecord(implementations[i])
		rows[i] = table.Row{record.ImplementationID, record.ServiceID}
	}

	detail := func(index int) string {
		if index < 0 || index >= len(implementations) {
			return ""
		}
		return implementationDetailView(&implementations[index])
	}

	return tableview.ChildView{
		Headers:        []string{"IMPLEMENTATION", "SERVICE"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Implementations",
		ParentType:     "api-implementation",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(implementations) {
				return nil
			}
			return &implementations[index]
		},
	}, nil
}
