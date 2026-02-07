package portal

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func init() {
	tableview.RegisterChildLoader("portal", "pages", loadPortalPages)
	tableview.RegisterChildLoader("portal", "snippets", loadPortalSnippets)
	tableview.RegisterChildLoader("portal", "teams", loadPortalTeams)
	tableview.RegisterChildLoader("portal-page", "content", loadPortalPageContent)
	tableview.RegisterChildLoader("portal-snippet", "content", loadPortalSnippetContent)
}

func loadPortalPages(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	portalID, err := portalIDFromParent(parent)
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

	pageAPI := sdk.GetPortalPageAPI()
	if pageAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("portal pages client is not available")
	}

	pages, err := fetchPortalPageSummaries(helper, pageAPI, portalID)
	if err != nil {
		return tableview.ChildView{}, err
	}

	flattened := flattenPortalPages(pages)
	cache := newPortalPageDetailCache()

	rows := make([]table.Row, 0, len(flattened))
	for _, page := range flattened {
		record := portalPageSummaryToRecord(page)
		rows = append(rows, table.Row{record.ID, record.Title})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(flattened) {
			return ""
		}
		page := flattened[index]
		if detailRecord, ok := cache.Get(page.GetID()); ok {
			return portalPageInfoDetail(page, &detailRecord)
		}
		return portalPageInfoDetail(page, nil)
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "TITLE"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Pages",
		ParentType:     "portal-page",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(flattened) {
				return nil
			}
			page := flattened[index]
			return &portalPageContentContext{
				portalID: portalID,
				pageID:   strings.TrimSpace(page.GetID()),
				cache:    cache,
			}
		},
	}, nil
}

func loadPortalSnippets(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	portalID, err := portalIDFromParent(parent)
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

	snippetAPI := sdk.GetPortalSnippetAPI()
	if snippetAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("portal snippets client is not available")
	}

	summaries, err := fetchPortalSnippetSummaries(helper, snippetAPI, portalID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	rows := make([]table.Row, 0, len(summaries))
	for _, snippet := range summaries {
		record := portalSnippetSummaryToRecord(snippet)
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	cache := newPortalSnippetDetailCache()

	detail := func(index int) string {
		if index < 0 || index >= len(summaries) {
			return ""
		}
		snippet := summaries[index]
		if detailRecord, ok := cache.Get(snippet.GetID()); ok {
			return portalSnippetInfoDetail(snippet, &detailRecord)
		}
		return portalSnippetInfoDetail(snippet, nil)
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Snippets",
		ParentType:     "portal-snippet",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(summaries) {
				return nil
			}
			snippet := summaries[index]
			return &portalSnippetContentContext{
				portalID:  portalID,
				snippetID: strings.TrimSpace(snippet.GetID()),
				cache:     cache,
			}
		},
	}, nil
}

func loadPortalTeams(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	portalID, err := portalIDFromParent(parent)
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

	teamAPI := sdk.GetPortalTeamAPI()
	if teamAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("portal teams client is not available")
	}

	teams, err := fetchPortalTeams(helper, teamAPI, portalID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	rows := make([]table.Row, 0, len(teams))
	for _, team := range teams {
		record := portalTeamSummaryToRecord(team)
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(teams) {
			return ""
		}
		return portalTeamDetailView(teams[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Teams",
		ParentType:     "portal-team",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(teams) {
				return nil
			}
			return &teams[index]
		},
	}, nil
}

func portalIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("portal parent is nil")
	}

	switch p := parent.(type) {
	case *kkComps.Portal:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("portal identifier is missing")
		}
		return id, nil
	case *kkComps.PortalResponse:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("portal identifier is missing")
		}
		return id, nil
	case *kkComps.ListPortalsResponsePortal:
		id := strings.TrimSpace(p.GetID())
		if id == "" {
			return "", fmt.Errorf("portal identifier is missing")
		}
		return id, nil
	case kkComps.ListPortalsResponsePortal:
		id := strings.TrimSpace(p.GetID())
		if id == "" {
			return "", fmt.Errorf("portal identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}
