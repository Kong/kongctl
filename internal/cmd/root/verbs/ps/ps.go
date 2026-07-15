package ps

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/columns"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/processes"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	defaultStopTimeout = 15 * time.Second
)

var (
	use = "ps"

	short = i18n.T("root.verbs.ps.short", "Manage detached processes")
	long  = normalizers.LongDesc(i18n.T("root.verbs.ps.long",
		`List and manage detached background processes tracked by the local process registry.`))
	example = normalizers.Examples(i18n.T("root.verbs.ps.examples",
		fmt.Sprintf(`
  # List detached kongctl processes
  %[1]s ps

  # Stop one detached process
  %[1]s ps stop 12345

  # Stop all detached processes
  %[1]s ps stop --all
`, meta.CLIName)))
)

type processListItem struct {
	PID       int              `json:"pid"                yaml:"pid"`
	Status    processes.Status `json:"status"             yaml:"status"`
	Kind      string           `json:"kind"               yaml:"kind"`
	Profile   string           `json:"profile,omitempty"  yaml:"profile,omitempty"`
	CreatedAt time.Time        `json:"created_at"         yaml:"created_at"`
	LogFile   string           `json:"log_file,omitempty" yaml:"log_file,omitempty"`
	Record    string           `json:"record_file"        yaml:"record_file"`
}

type processStopResult struct {
	PID     int    `json:"pid"              yaml:"pid"`
	Kind    string `json:"kind"             yaml:"kind"`
	Action  string `json:"action"           yaml:"action"`
	Success bool   `json:"success"          yaml:"success"`
	Detail  string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

type psCmd struct {
	stopAll     bool
	stopTimeout time.Duration
}

// NewPSCmd builds the ps verb.
func NewPSCmd() (*cobra.Command, error) {
	c := &psCmd{
		stopTimeout: defaultStopTimeout,
	}

	cmdObj := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		RunE:    c.runList,
	}

	stopCmd := &cobra.Command{
		Use:   "stop <pid>",
		Short: "Stop detached kongctl processes",
		Long:  "Stop one detached kongctl process by PID or all tracked processes with --all.",
		RunE:  c.runStop,
	}
	stopCmd.Flags().BoolVar(&c.stopAll, "all", false, "Stop all tracked detached processes.")
	stopCmd.Flags().DurationVar(&c.stopTimeout, "timeout", c.stopTimeout,
		"How long to wait for graceful process shutdown.")
	cmdObj.AddCommand(stopCmd)
	columns.AddFlags(cmdObj.PersistentFlags())

	return cmdObj, nil
}

func (c *psCmd) runList(cmdObj *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cmdObj, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the ps command does not accept positional arguments"),
		}
	}
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	selected, err := columns.Resolve(helper.GetCmd(), outType)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	records, err := processes.ListRecords()
	if err != nil {
		return cmd.PrepareExecutionError("failed to list detached processes", err, helper.GetCmd())
	}

	items := make([]processListItem, 0, len(records))
	for _, record := range records {
		state := processes.Inspect(record.Record)
		items = append(items, processListItem{
			PID:       record.PID,
			Status:    state.Status,
			Kind:      record.Kind,
			Profile:   record.Profile,
			CreatedAt: record.CreatedAt,
			LogFile:   record.LogFile,
			Record:    record.File,
		})
	}

	if outType == common.TEXT {
		if len(selected) > 0 {
			headers, rows, err := columns.Project(items, selected)
			if err != nil {
				return err
			}
			return columns.Render(helper.GetStreams().Out, headers, rows, 120)
		}
		return renderListText(helper.GetStreams().Out, items)
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(items)

	return nil
}

func (c *psCmd) runStop(cmdObj *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cmdObj, args)
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	selected, err := columns.Resolve(helper.GetCmd(), outType)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	records, err := processes.ListRecords()
	if err != nil {
		return cmd.PrepareExecutionError("failed to load detached process records", err, helper.GetCmd())
	}

	targets, err := c.resolveTargets(helper.GetArgs(), records)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	results := make([]processStopResult, 0, len(targets))
	var stopErrors []error
	for _, target := range targets {
		result := stopDetachedProcess(target, c.stopTimeout)
		results = append(results, result)
		if !result.Success {
			stopErrors = append(stopErrors, fmt.Errorf("pid %d: %s", result.PID, result.Detail))
		}
	}

	if outType == common.TEXT {
		if len(selected) > 0 {
			headers, rows, err := columns.Project(results, selected)
			if err != nil {
				return err
			}
			if err := columns.Render(helper.GetStreams().Out, headers, rows, 120); err != nil {
				return err
			}
		} else if err := renderStopText(helper.GetStreams().Out, results); err != nil {
			return err
		}
	} else {
		printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(results)
	}

	if len(stopErrors) > 0 {
		return cmd.PrepareExecutionError(
			"one or more detached processes failed to stop",
			errors.Join(stopErrors...),
			helper.GetCmd(),
		)
	}

	return nil
}

func (c *psCmd) resolveTargets(args []string, records []processes.StoredRecord) ([]processes.StoredRecord, error) {
	if c.stopAll {
		if len(args) > 0 {
			return nil, fmt.Errorf("do not provide a PID when using --all")
		}
		return records, nil
	}

	if len(args) != 1 {
		return nil, fmt.Errorf("provide a process PID or use --all")
	}

	pid, err := strconv.Atoi(args[0])
	if err != nil || pid <= 0 {
		return nil, fmt.Errorf("invalid PID %q", args[0])
	}

	for _, record := range records {
		if record.PID == pid {
			return []processes.StoredRecord{record}, nil
		}
	}

	return nil, fmt.Errorf("no detached process record found for PID %d", pid)
}

func stopDetachedProcess(record processes.StoredRecord, timeout time.Duration) processStopResult {
	result := processStopResult{
		PID:  record.PID,
		Kind: record.Kind,
	}

	state := processes.Inspect(record.Record)
	switch state.Status {
	case processes.StatusRunning:
		if err := processes.Terminate(record.PID, timeout); err != nil {
			result.Action = "stop"
			result.Detail = err.Error()
			return result
		}
		if err := processes.RemoveRecordByPath(record.File); err != nil {
			result.Action = "stop"
			result.Detail = fmt.Sprintf("process stopped but failed to remove record: %v", err)
			return result
		}
		result.Action = "stopped"
		result.Success = true
		result.Detail = "sent SIGTERM and removed process record"
		return result
	case processes.StatusExited, processes.StatusStale:
		if err := processes.RemoveRecordByPath(record.File); err != nil {
			result.Action = "prune"
			result.Detail = fmt.Sprintf("failed to remove stale record: %v", err)
			return result
		}
		result.Action = "pruned"
		result.Success = true
		result.Detail = "removed stale process record"
		return result
	case processes.StatusUnknown:
		result.Action = "inspect"
		if state.CheckError != "" {
			result.Detail = state.CheckError
			return result
		}
		result.Detail = "unable to determine process state"
		return result
	default:
		result.Action = "inspect"
		if state.CheckError != "" {
			result.Detail = state.CheckError
			return result
		}
		result.Detail = "unable to determine process state"
		return result
	}
}

func renderListText(out io.Writer, items []processListItem) error {
	if out == nil {
		return nil
	}
	if len(items) == 0 {
		_, err := fmt.Fprintln(out, "No detached kongctl processes found.")
		return err
	}

	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{strconv.Itoa(item.PID), string(item.Status), displayOrDash(item.Kind), displayOrDash(item.Profile)}
	}
	return columns.Render(out, []string{"PID", "STATUS", "KIND", "PROFILE"}, rows, 120)
}

func renderStopText(out io.Writer, results []processStopResult) error {
	if out == nil {
		return nil
	}
	if len(results) == 0 {
		_, err := fmt.Fprintln(out, "No detached kongctl processes matched.")
		return err
	}

	rows := make([][]string, len(results))
	for i, result := range results {
		rows[i] = []string{strconv.Itoa(result.PID), result.Action, strconv.FormatBool(result.Success)}
	}
	return columns.Render(out, []string{"PID", "ACTION", "SUCCESS"}, rows, 120)
}

func displayOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
