package accesstoken

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func init() {
	tableview.RegisterChildLoader("system-account", "access-tokens", loadSystemAccountAccessTokens)
}

func loadSystemAccountAccessTokens(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	accountID, err := accountIDFromParent(parent)
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

	tokens, err := runListAccessTokens(accountID, sdk.GetSystemAccountAccessTokenAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildAccessTokenChildView(tokens), nil
}

func accountIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("system account parent is nil")
	}

	switch sa := parent.(type) {
	case *kkComps.SystemAccount:
		if id := sa.GetID(); id != nil {
			return *id, nil
		}
		return "", fmt.Errorf("system account has no ID")
	case kkComps.SystemAccount:
		if id := sa.GetID(); id != nil {
			return *id, nil
		}
		return "", fmt.Errorf("system account has no ID")
	default:
		return "", fmt.Errorf("unexpected parent type %T for access tokens", parent)
	}
}
