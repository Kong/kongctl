package helpers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/kong/kongctl/internal/util"
)

type IdentityOrgChartImportAPI interface {
	ImportOrgChart(context.Context, string, kkComps.ImportPrincipalsRequest, ...kkOps.Option) (
		*kkOps.ImportOrgChartResponse, error)
}

type IdentityOrgChartImportUsers struct {
	Created   *int64 `json:"created" yaml:"created"`
	Updated   *int64 `json:"updated" yaml:"updated"`
	Unchanged *int64 `json:"unchanged" yaml:"unchanged"`
	Inactive  *int64 `json:"inactive" yaml:"inactive"`
	Adopted   *int64 `json:"adopted,omitempty" yaml:"adopted,omitempty"`
}

type IdentityOrgChartImportTeams struct {
	Created *int64 `json:"created" yaml:"created"`
	Deleted *int64 `json:"deleted" yaml:"deleted"`
}

type IdentityOrgChartImportMemberships struct {
	Added   *int64 `json:"added" yaml:"added"`
	Removed *int64 `json:"removed" yaml:"removed"`
}

type IdentityOrgChartImportCounts struct {
	Users       *IdentityOrgChartImportUsers       `json:"users" yaml:"users"`
	Teams       *IdentityOrgChartImportTeams       `json:"teams" yaml:"teams"`
	Memberships *IdentityOrgChartImportMemberships `json:"memberships" yaml:"memberships"`
}

type IdentityOrgChartImportWarning struct {
	Row  *int64 `json:"row,omitempty" yaml:"row,omitempty"`
	Type string `json:"type" yaml:"type"`
}

type IdentityOrgChartImportError struct {
	Row   *int64 `json:"row,omitempty" yaml:"row,omitempty"`
	Field string `json:"field" yaml:"field"`
}

// Public summaries contain counts and value-free notices. The API can carry employee
// names/emails in arbitrary diagnostic fields and stays exclusively private.
type IdentityOrgChartImportSummary struct {
	DirectoryID  string                          `json:"directory_id" yaml:"directory_id"`
	DryRun       bool                            `json:"dry_run" yaml:"dry_run"`
	Status       string                          `json:"status" yaml:"status"`
	Rows         int                             `json:"rows" yaml:"rows"`
	Counts       IdentityOrgChartImportCounts    `json:"counts" yaml:"counts"`
	WarningCount int                             `json:"warning_count" yaml:"warning_count"`
	ErrorCount   int                             `json:"error_count" yaml:"error_count"`
	Warnings     []IdentityOrgChartImportWarning `json:"warnings" yaml:"warnings"`
	Errors       []IdentityOrgChartImportError   `json:"errors" yaml:"errors"`
}

func publicIdentityImportRow(value *int64, rows int) *int64 {
	if value == nil || *value < 1 || *value > int64(rows) {
		return nil
	}
	return value
}

func publicIdentityImportDiagnostics(warnings, failures []json.RawMessage, rows int) (
	[]IdentityOrgChartImportWarning, []IdentityOrgChartImportError,
) {
	publicWarnings := make([]IdentityOrgChartImportWarning, 0, len(warnings))
	publicErrors := make([]IdentityOrgChartImportError, 0, len(failures))
	for _, raw := range warnings {
		var value IdentityOrgChartImportWarning
		if json.Unmarshal(raw, &value) != nil {
			value = IdentityOrgChartImportWarning{}
		}
		switch value.Type {
		case "principal_adopted", "email_changed", "quota_shortfall", "inactive_not_reactivated":
		default:
			value.Type = "other"
		}
		value.Row = publicIdentityImportRow(value.Row, rows)
		publicWarnings = append(publicWarnings, value)
	}
	for _, raw := range failures {
		var value IdentityOrgChartImportError
		if json.Unmarshal(raw, &value) != nil {
			value = IdentityOrgChartImportError{}
		}
		switch value.Field {
		case "external_id", "email", "given_name", "family_name", "manager_external_id", "cost_center":
		default:
			value.Field = "other"
		}
		value.Row = publicIdentityImportRow(value.Row, rows)
		publicErrors = append(publicErrors, value)
	}
	return publicWarnings, publicErrors
}

// Syntax and the observed row bound are checked locally; canonical columns,
// matching and hierarchy validation remain the native whole-file operation.
func ValidateIdentityOrgChartCSV(contents []byte) (int, error) {
	if !utf8.Valid(contents) {
		return 0, fmt.Errorf("org-chart CSV must be UTF-8")
	}
	reader := csv.NewReader(bytes.NewReader(contents))
	rows := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("invalid CSV syntax near record %d; input values withheld", rows+1)
		}
		rows++
		if rows > 50001 {
			return 0, fmt.Errorf("org-chart CSV supports at most 50000 data rows")
		}
	}
	if rows < 2 {
		return 0, fmt.Errorf("org-chart CSV requires a header and at least one data row")
	}
	return rows - 1, nil
}

func validateIdentityOrgChartImportCounts(value *IdentityOrgChartImportCounts) error {
	if value == nil || value.Users == nil || value.Teams == nil || value.Memberships == nil {
		return fmt.Errorf("import report has missing counts; inspect the private report")
	}
	for _, count := range []*int64{
		value.Users.Created, value.Users.Updated, value.Users.Unchanged,
		value.Users.Inactive, value.Teams.Created, value.Teams.Deleted, value.Memberships.Added, value.Memberships.Removed,
	} {
		if count == nil || *count < 0 {
			return fmt.Errorf("import report has missing or negative counts; inspect the private report")
		}
	}
	if value.Users.Adopted != nil && *value.Users.Adopted < 0 {
		return fmt.Errorf("import report has negative adoption count; inspect the private report")
	}
	return nil
}

func identityOrgChartImportFailureBody(err error) []byte {
	var generic *sdkerrors.SDKError
	if errors.As(err, &generic) {
		return []byte(generic.Body)
	}
	var validation *sdkerrors.ResponseValidationError
	if errors.As(err, &validation) {
		return []byte(validation.Body)
	}
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		if body := []byte(cause.Error()); json.Valid(body) {
			return body
		}
	}
	body, _ := json.Marshal(map[string]string{"request_error": err.Error()})
	return body
}

func ImportAndCaptureIdentityOrgChart(ctx context.Context, api IdentityOrgChartImportAPI,
	directoryID string, contents []byte, dryRun bool, outputPath string,
) (*IdentityOrgChartImportSummary, error) {
	if api == nil {
		return nil, fmt.Errorf("org-chart import API is not configured")
	}
	if !util.IsValidUUID(directoryID) {
		return nil, fmt.Errorf("directory ID must be a UUID")
	}
	contents = bytes.Clone(contents)
	rows, err := ValidateIdentityOrgChartCSV(contents)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(contents)
	metadata := secretoutput.Metadata{
		DirectoryID: directoryID, DryRun: &dryRun, RequestSHA256: hex.EncodeToString(digest[:]),
	}
	output, err := secretoutput.ReserveReport(outputPath, metadata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	response, err := api.ImportOrgChart(httpclient.WithoutRetries(ctx), directoryID, kkComps.ImportPrincipalsRequest{
		File: kkComps.File{FileName: "org-chart.csv", Content: contents}, DryRun: &dryRun,
	}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		// Error text may include arbitrary server/transport diagnostics. Persist
		// it deliberately in the private file, never in a returned unwrap target.
		body := identityOrgChartImportFailureBody(err)
		if captureErr := output.CaptureReport(metadata, body); captureErr != nil {
			return nil, fmt.Errorf("import outcome unconfirmed; private error capture failed: %w", captureErr)
		}
		return nil, fmt.Errorf("%w; outcome unconfirmed; inspect the private report and remote state before retrying",
			identitySecretAPIError("import organization chart", err))
	}
	if response == nil || response.StatusCode != http.StatusOK || response.RawResponse == nil ||
		response.RawResponse.Body == nil {
		return nil, fmt.Errorf("import response is missing; outcome unconfirmed; inspect state before retrying")
	}
	body, err := io.ReadAll(response.RawResponse.Body)
	response.RawResponse.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("import response could not be read; outcome unconfirmed; inspect state before retrying")
	}
	if err := output.CaptureReport(metadata, body); err != nil {
		return nil, fmt.Errorf("import report capture failed; inspect the reserved file before retrying: %w", err)
	}
	var report struct {
		Counts   *IdentityOrgChartImportCounts `json:"counts"`
		Warnings []json.RawMessage             `json:"warnings"`
		Errors   []json.RawMessage             `json:"errors"`
	}
	if json.Unmarshal(body, &report) != nil || report.Warnings == nil || report.Errors == nil {
		return nil, fmt.Errorf("import report is incomplete; inspect the private report before retrying")
	}
	if err := validateIdentityOrgChartImportCounts(report.Counts); err != nil {
		return nil, err
	}
	status := "completed"
	if dryRun {
		status = "validated"
	}
	summary := &IdentityOrgChartImportSummary{
		DirectoryID: directoryID, DryRun: dryRun, Rows: rows,
		Counts: *report.Counts, Status: status, WarningCount: len(report.Warnings), ErrorCount: len(report.Errors),
	}
	summary.Warnings, summary.Errors = publicIdentityImportDiagnostics(report.Warnings, report.Errors, rows)
	if summary.ErrorCount != 0 {
		summary.Status = "reported_errors"
		return summary, fmt.Errorf("import reported %d row errors; inspect the private report", summary.ErrorCount)
	}
	return summary, nil
}
