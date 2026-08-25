package aigateway

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
)

type aiGatewayConfigStoreSecretRecord struct {
	Key              string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func (h aiGatewayConfigStoresHandler) runSecrets(store string, args []string) error {
	helper := cmd.BuildHelper(h.cmd, append([]string{store, "secrets"}, args...))
	if len(args) > 1 {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"too many arguments. Listing AI Gateway Config Store secrets requires a store and optional secret key",
		)}
	}
	store = strings.TrimSpace(store)
	if store == "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf("an AI Gateway Config Store ID or name is required")}
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}
	gatewayID, gatewayName := getAIGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"only one of --%s or --%s can be provided",
			aiGatewayIDFlagName,
			aiGatewayNameFlagName,
		)}
	}
	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"an AI Gateway identifier is required. Provide --%s or --%s",
			aiGatewayIDFlagName,
			aiGatewayNameFlagName,
		)}
	}
	if gatewayID == "" {
		gatewayID, err = resolveAIGatewayIDByName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}
	api := sdk.GetAIGatewayConfigStoresAPI()
	if api == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Config Stores client is not available",
			Err: fmt.Errorf("AI Gateway Config Stores client not configured"),
		}
	}
	if len(args) == 1 {
		return h.getSingleSecret(helper, api, gatewayID, store, args[0], outType, printer)
	}
	return h.listSecrets(helper, api, gatewayID, store, outType, printer, cfg)
}

func (h aiGatewayConfigStoresHandler) listSecrets(
	helper cmd.Helper,
	api helpers.AIGatewayConfigStoresAPI,
	gatewayID string,
	store string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	secrets, err := fetchAIGatewayConfigStoreSecrets(helper, api, gatewayID, store, cfg)
	if err != nil {
		return err
	}
	records := make([]aiGatewayConfigStoreSecretRecord, 0, len(secrets))
	rows := make([]table.Row, 0, len(secrets))
	for _, secret := range secrets {
		record := aiGatewayConfigStoreSecretToRecord(secret)
		records = append(records, record)
		rows = append(rows, table.Row{record.Key, record.LocalCreatedTime, record.LocalUpdatedTime})
	}
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		secrets,
		"",
		tableview.WithCustomTable([]string{"KEY", "CREATED", "UPDATED"}, rows),
		tableview.WithRootLabel("secrets"),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(secrets) {
				return ""
			}
			return aiGatewayConfigStoreSecretDetailView(secrets[index])
		}),
	)
}

func (h aiGatewayConfigStoresHandler) getSingleSecret(
	helper cmd.Helper,
	api helpers.AIGatewayConfigStoresAPI,
	gatewayID string,
	store string,
	key string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf("a Config Store secret key is required")}
	}
	res, err := api.GetAiGatewayConfigStoreSecret(
		helper.GetContext(),
		kkOps.GetAiGatewayConfigStoreSecretRequest{
			GatewayID:           gatewayID,
			ConfigStoreIDOrName: store,
			Key:                 key,
		},
	)
	if err != nil {
		return cmd.PrepareExecutionError(
			"Failed to get AI Gateway Config Store secret",
			err,
			helper.GetCmd(),
			cmd.TryConvertErrorToAttrs(err)...,
		)
	}
	secret := res.GetAIGatewayConfigStoreSecret()
	if secret == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Config Store secret response was empty",
			Err: fmt.Errorf("no Config Store secret returned for key %s", key),
		}
	}
	record := aiGatewayConfigStoreSecretToRecord(*secret)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		secret,
		"",
		tableview.WithRootLabel("secrets"),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index == 0 {
				return aiGatewayConfigStoreSecretDetailView(*secret)
			}
			return ""
		}),
	)
}

func fetchAIGatewayConfigStoreSecrets(
	helper cmd.Helper,
	api helpers.AIGatewayConfigStoresAPI,
	gatewayID string,
	store string,
	cfg config.Hook,
) ([]kkComps.AIGatewayConfigStoreSecret, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var secrets []kkComps.AIGatewayConfigStoreSecret
	for {
		res, err := api.ListAiGatewayConfigStoreSecrets(
			helper.GetContext(),
			kkOps.ListAiGatewayConfigStoreSecretsRequest{
				GatewayID:           gatewayID,
				ConfigStoreIDOrName: store,
				PageSize:            &requestPageSize,
				PageAfter:           pageAfter,
			},
		)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway Config Store secrets",
				err,
				helper.GetCmd(),
				cmd.TryConvertErrorToAttrs(err)...,
			)
		}
		body := res.GetListAIGatewayConfigStoreSecretsResponse()
		if body == nil {
			break
		}
		secrets = append(secrets, body.Data...)
		next := pagination.ExtractPageAfterCursor(body.Meta.Page.Next)
		if next == "" {
			break
		}
		pageAfter = &next
	}
	return secrets, nil
}

func aiGatewayConfigStoreSecretToRecord(
	secret kkComps.AIGatewayConfigStoreSecret,
) aiGatewayConfigStoreSecretRecord {
	record := aiGatewayConfigStoreSecretRecord{
		Key:              valueOrMissing(secret.Key),
		LocalCreatedTime: aiGatewayMissingValue,
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if !secret.CreatedAt.IsZero() {
		record.LocalCreatedTime = secret.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if !secret.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = secret.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayConfigStoreSecretDetailView(secret kkComps.AIGatewayConfigStoreSecret) string {
	record := aiGatewayConfigStoreSecretToRecord(secret)
	return fmt.Sprintf(
		"key: %s\ncreated_at: %s\nupdated_at: %s",
		record.Key,
		record.LocalCreatedTime,
		record.LocalUpdatedTime,
	)
}
