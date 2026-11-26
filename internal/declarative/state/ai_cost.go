package state

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	o "github.com/Kong/sdk-konnect-go/models/operations"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util/pagination"
)

// AICostAPI wraps the generated SDK's discriminated request and response models.
type AICostAPI struct{ sdk *sdk.SDK }

// NewAICostAPI uses the same authenticated SDK as the other Konnect APIs.
func NewAICostAPI(client helpers.SDKAPI) *AICostAPI {
	konnect, ok := client.(*helpers.KonnectSDK)
	if !ok || konnect.SDK == nil {
		return nil
	}
	return &AICostAPI{sdk: konnect.SDK}
}

type AICostObject map[string]any

// GetID returns the object's ID. An attribution plugin has none of its own, so
// its AI Gateway's ID identifies it.
func (v AICostObject) GetID() string {
	if id, _ := v["id"].(string); id != "" {
		return id
	}
	id, _ := v["gateway_id"].(string)
	return id
}

func (v AICostObject) GetName() string { name, _ := v["name"].(string); return name }
func (v AICostObject) GetManagedBy() map[string]string {
	result := map[string]string{}
	values, _ := v["managed_by"].(map[string]any)
	for k, v := range values {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

func costObject[T any](value *T) (AICostObject, error) {
	if value == nil {
		return nil, fmt.Errorf("missing AI Cost Management response")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("invalid AI Cost Management response")
	}
	var result AICostObject
	if err := json.Unmarshal(data, &result); err != nil || result == nil {
		return nil, fmt.Errorf("invalid AI Cost Management response")
	}
	return result, nil
}

func costPages[T any](size int64, nextPage func(*int64, *string) ([]T, c.CursorMeta, error)) ([]AICostObject, error) {
	if size < 1 || size > 100 {
		return nil, fmt.Errorf("AI Cost Management page size must be between 1 and 100")
	}
	values := []AICostObject{}
	var cursor *string
	var tracker pagination.CursorTracker
	for {
		data, meta, err := nextPage(&size, cursor)
		if err != nil {
			return nil, err
		}
		for _, value := range data {
			decoded, err := costObject(&value)
			if err != nil {
				return nil, err
			}
			values = append(values, decoded)
		}
		next, err := tracker.Next(meta.Page.Next)
		if err != nil {
			return nil, err
		}
		if next == "" {
			return values, nil
		}
		cursor = &next
	}
}

func (a *AICostAPI) List(ctx context.Context, kind resources.ResourceType, size int64) ([]AICostObject, error) {
	if a == nil {
		return nil, fmt.Errorf("AI Cost Management SDK unavailable")
	}
	//exhaustive:ignore
	switch kind {
	case resources.ResourceTypeAICostBudget:
		return costPages(size, func(size *int64, after *string) ([]c.AICostBudget, c.CursorMeta, error) {
			return costPage(
				a.sdk.Budgets.ListCostBudgets(ctx, o.ListCostBudgetsRequest{PageSize: size, PageAfter: after}),
			)
		})
	case resources.ResourceTypeAICostChannel:
		return costPages(size, func(size *int64, after *string) ([]c.AICostNotificationChannel, c.CursorMeta, error) {
			return costPage(
				a.sdk.NotificationChannels.ListCostNotificationChannels(
					ctx,
					o.ListCostNotificationChannelsRequest{PageSize: size, PageAfter: after},
				),
			)
		})
	case resources.ResourceTypeAICostSchedule:
		return costPages(size, func(size *int64, after *string) ([]c.AICostExportSchedule, c.CursorMeta, error) {
			return costPage(
				a.sdk.Exports.ListCostExportSchedules(
					ctx,
					o.ListCostExportSchedulesRequest{PageSize: size, PageAfter: after},
				),
			)
		})
	case resources.ResourceTypeAICostPlugin:
		return costPages(size, func(size *int64, after *string) ([]c.AICostAttributionPlugin, c.CursorMeta, error) {
			return costPage(
				a.sdk.AttributionPlugins.ListCostAttributionPlugins(
					ctx,
					o.ListCostAttributionPluginsRequest{PageSize: size, PageAfter: after},
				),
			)
		})
	}
	return nil, fmt.Errorf("unsupported AI Cost Management collection %q", kind)
}

type costPageBody[T any] interface {
	GetData() []T
	GetMeta() c.CursorMeta
}

type costPageResponse[T any, B costPageBody[T]] interface {
	GetObject() B
}

func costPage[T any, B costPageBody[T], R costPageResponse[T, B]](response R, err error) ([]T, c.CursorMeta, error) {
	if err != nil {
		return nil, c.CursorMeta{}, err
	}
	page := response.GetObject()
	meta := page.GetMeta()
	data := page.GetData()
	if meta.Page.Size < 1 || data == nil {
		return nil, c.CursorMeta{}, fmt.Errorf("incomplete AI Cost Management page")
	}
	return data, meta, nil
}

func (a *AICostAPI) Get(ctx context.Context, kind resources.ResourceType, id string) (AICostObject, error) {
	if a == nil {
		return nil, fmt.Errorf("AI Cost Management SDK unavailable")
	}
	var value AICostObject
	var err error
	//exhaustive:ignore
	switch kind {
	case resources.ResourceTypeAICostBudget:
		response, readErr := a.sdk.Budgets.GetCostBudget(ctx, id)
		value, err = costResult(response.GetAICostBudget(), readErr)
	case resources.ResourceTypeAICostChannel:
		response, readErr := a.sdk.NotificationChannels.GetCostNotificationChannel(ctx, id)
		value, err = costResult(response.GetAICostNotificationChannel(), readErr)
	case resources.ResourceTypeAICostSchedule:
		response, readErr := a.sdk.Exports.GetCostExportSchedule(ctx, id)
		value, err = costResult(response.GetAICostExportSchedule(), readErr)
	case resources.ResourceTypeAICostRule:
		response, readErr := a.sdk.Budgets.GetCostNotificationRule(ctx, id)
		value, err = costResult(response.GetAICostNotificationRule(), readErr)
	case resources.ResourceTypeAICostPlugin:
		response, readErr := a.sdk.AttributionPlugins.GetCostAttributionPlugin(ctx, id)
		value, err = costResult(response.GetAICostAttributionPlugin(), readErr)
	default:
		return nil, fmt.Errorf("unsupported AI Cost Management resource %q", kind)
	}
	if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
		return nil, nil
	}
	return value, err
}

func costResult[T any](value *T, err error) (AICostObject, error) {
	if err != nil {
		return nil, err
	}
	return costObject(value)
}

func costRequest[T any](fields map[string]any) (T, error) {
	var request T
	data, err := json.Marshal(fields)
	if err != nil {
		return request, fmt.Errorf("invalid AI Cost Management payload")
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return request, fmt.Errorf("invalid AI Cost Management request fields")
	}
	return request, nil
}

func (a *AICostAPI) Write(
	ctx context.Context,
	kind resources.ResourceType,
	id string,
	fields map[string]any,
) (AICostObject, error) {
	if a == nil {
		return nil, fmt.Errorf("AI Cost Management SDK unavailable")
	}
	//exhaustive:ignore
	switch kind {
	case resources.ResourceTypeAICostBudget:
		if id == "" {
			request, err := costRequest[o.CreateCostBudgetRequestBody](fields)
			if err != nil {
				return nil, err
			}
			response, err := a.sdk.Budgets.CreateCostBudget(ctx, request)
			return costResult(response.GetAICostBudget(), err)
		}
		request, err := costRequest[o.UpsertCostBudgetRequestBody](fields)
		if err != nil {
			return nil, err
		}
		response, err := a.sdk.Budgets.UpsertCostBudget(ctx, id, request)
		return costResult(response.GetAICostBudget(), err)
	case resources.ResourceTypeAICostChannel:
		if id == "" {
			request, err := costRequest[o.CreateCostNotificationChannelRequestBody](fields)
			if err != nil {
				return nil, err
			}
			response, err := a.sdk.NotificationChannels.CreateCostNotificationChannel(ctx, request)
			return costResult(response.GetAICostNotificationChannel(), err)
		}
		request, err := costRequest[o.UpsertCostNotificationChannelRequestBody](fields)
		if err != nil {
			return nil, err
		}
		response, err := a.sdk.NotificationChannels.UpsertCostNotificationChannel(ctx, id, request)
		return costResult(response.GetAICostNotificationChannel(), err)
	case resources.ResourceTypeAICostSchedule:
		if id == "" {
			request, err := costRequest[c.AICostExportScheduleInput](fields)
			if err != nil {
				return nil, err
			}
			response, err := a.sdk.Exports.CreateCostExportSchedule(ctx, request)
			return costResult(response.GetAICostExportSchedule(), err)
		}
		request, err := costRequest[c.AICostExportScheduleInput](fields)
		if err != nil {
			return nil, err
		}
		response, err := a.sdk.Exports.UpsertCostExportSchedule(ctx, id, request)
		return costResult(response.GetAICostExportSchedule(), err)
	case resources.ResourceTypeAICostRule:
		request, err := costRequest[o.UpsertCostNotificationRuleRequestBody](fields)
		if err != nil {
			return nil, err
		}
		response, err := a.sdk.Budgets.UpsertCostNotificationRule(ctx, id, request)
		return costResult(response.GetAICostNotificationRule(), err)
	case resources.ResourceTypeAICostPlugin:
		request, err := costRequest[c.AICostAttributionPluginConfiguration](fields)
		if err != nil {
			return nil, err
		}
		response, err := a.sdk.AttributionPlugins.UpsertCostAttributionPlugin(ctx, id, request)
		return costResult(response.GetAICostAttributionPlugin(), err)
	}
	return nil, fmt.Errorf("unsupported AI Cost Management resource %q", kind)
}

func (a *AICostAPI) Delete(ctx context.Context, kind resources.ResourceType, id string) error {
	if a == nil {
		return fmt.Errorf("AI Cost Management SDK unavailable")
	}
	var err error
	//exhaustive:ignore
	switch kind {
	case resources.ResourceTypeAICostBudget:
		_, err = a.sdk.Budgets.DeleteCostBudget(ctx, id)
	case resources.ResourceTypeAICostChannel:
		_, err = a.sdk.NotificationChannels.DeleteCostNotificationChannel(ctx, id)
	case resources.ResourceTypeAICostSchedule:
		_, err = a.sdk.Exports.DeleteCostExportSchedule(ctx, id)
	case resources.ResourceTypeAICostRule:
		_, err = a.sdk.Budgets.DeleteCostNotificationRule(ctx, id)
	case resources.ResourceTypeAICostPlugin:
		_, err = a.sdk.AttributionPlugins.DeleteCostAttributionPlugin(ctx, id)
	default:
		return fmt.Errorf("unsupported AI Cost Management resource %q", kind)
	}
	if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
		return nil
	}
	return err
}

func AICostRuleUsesChannel(rule AICostObject, channelID string) bool {
	channels, _ := rule["channels"].([]any)
	for _, channel := range channels {
		ref, _ := channel.(map[string]any)
		if ref["id"] == channelID {
			return true
		}
	}
	return false
}

func (a *AICostAPI) ReadReport(ctx context.Context, name, id string, size int64) (any, error) {
	if a == nil {
		return nil, fmt.Errorf("AI Cost Management SDK unavailable")
	}
	switch name {
	case "budget-balances":
		if id != "" {
			response, err := a.sdk.Budgets.GetCostBudgetBalance(ctx, o.GetCostBudgetBalanceRequest{BudgetID: id})
			return costResult(response.GetObject(), err)
		}
		var evaluatedAt *time.Time
		return costPages(size, func(size *int64, after *string) ([]o.ListCostBudgetBalancesData, c.CursorMeta, error) {
			response, err := a.sdk.Budgets.ListCostBudgetBalances(
				ctx,
				o.ListCostBudgetBalancesRequest{PageSize: size, PageAfter: after, EvaluatedAt: evaluatedAt},
			)
			if err != nil {
				return nil, c.CursorMeta{}, err
			}
			body := response.GetObject()
			if body == nil {
				return nil, c.CursorMeta{}, fmt.Errorf("missing budget balances response")
			}
			evaluatedAt = &body.Meta.EvaluatedAt
			return body.Data, c.CursorMeta{
				Page: c.CursorMetaPage{Next: body.Meta.Page.Next, Size: body.Meta.Page.Size},
			}, nil
		})
	case "notification-events":
		if id != "" {
			response, err := a.sdk.Budgets.GetCostNotificationEvent(ctx, id, nil)
			return costResult(response.GetOneOf(), err)
		}
		return costPages(
			size,
			func(size *int64, after *string) ([]c.AICostNotificationThresholdEvent, c.CursorMeta, error) {
				return costPage(
					a.sdk.Budgets.ListCostNotificationEvents(
						ctx,
						o.ListCostNotificationEventsRequest{PageSize: size, PageAfter: after},
					),
				)
			},
		)
	case "notification-firings":
		return costPages(
			size,
			func(size *int64, after *string) ([]o.ListCostNotificationRuleFiringsData, c.CursorMeta, error) {
				return costPage(
					a.sdk.Budgets.ListCostNotificationRuleFirings(
						ctx,
						o.ListCostNotificationRuleFiringsRequest{BudgetID: id, PageSize: size, PageAfter: after},
					),
				)
			},
		)
	case "exports":
		if id != "" {
			response, err := a.sdk.Exports.GetCostExport(ctx, id)
			return costResult(response.GetAICostExport(), err)
		}
		return costPages(size, func(size *int64, after *string) ([]c.AICostExport, c.CursorMeta, error) {
			return costPage(
				a.sdk.Exports.ListCostExports(ctx, o.ListCostExportsRequest{PageSize: size, PageAfter: after}),
			)
		})
	case "usage-events":
		return costPages(size, func(size *int64, after *string) ([]c.AICostUsageEvent, c.CursorMeta, error) {
			return costPage(
				a.sdk.UsageEvents.ListCostUsageEvents(
					ctx,
					o.ListCostUsageEventsRequest{PageSize: size, PageAfter: after},
				),
			)
		})
	case "forecast":
		response, err := a.sdk.UsageEvents.ForecastCostUsageEvents(ctx, nil, nil)
		return costResult(response.GetObject(), err)
	}
	return nil, fmt.Errorf("unsupported AI Cost Management report %q", name)
}
