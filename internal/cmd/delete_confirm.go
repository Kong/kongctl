package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
)

type deleteContextKey string

const (
	deleteForceContextKey       deleteContextKey = "kongctl-delete-force"
	deleteAutoApproveContextKey deleteContextKey = "kongctl-delete-auto-approve"
)

// SetDeleteForce stores the --force flag value on the command context so that
// nested delete handlers can read it without rebinding flags or configs.
func SetDeleteForce(cmd *cobra.Command, force bool) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, deleteForceContextKey, force)
	cmd.SetContext(ctx)
}

// DeleteForceEnabled reports whether the current helper is operating in
// force mode (confirmation prompts should be skipped).
func DeleteForceEnabled(helper Helper) bool {
	if helper == nil || helper.GetCmd() == nil {
		return false
	}
	ctx := helper.GetCmd().Context()
	if ctx == nil {
		return false
	}
	force, _ := ctx.Value(deleteForceContextKey).(bool)
	return force
}

// SetDeleteAutoApprove stores the --yes/--approve flag state.
func SetDeleteAutoApprove(cmd *cobra.Command, approved bool) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, deleteAutoApproveContextKey, approved)
	cmd.SetContext(ctx)
}

// DeleteAutoApproveEnabled reports whether the user opted to skip confirmation prompts.
func DeleteAutoApproveEnabled(helper Helper) bool {
	if helper == nil || helper.GetCmd() == nil {
		return false
	}
	ctx := helper.GetCmd().Context()
	if ctx == nil {
		return false
	}
	approved, _ := ctx.Value(deleteAutoApproveContextKey).(bool)
	return approved
}

// ConfirmDelete prompts the user to confirm destructive delete operations
// unless the --force flag was provided.
func ConfirmDelete(helper Helper, description string, warnings ...string) error {
	if DeleteAutoApproveEnabled(helper) {
		return nil
	}

	streams := helper.GetStreams()
	fmt.Fprintf(streams.Out, "\nYou are about to delete %s\n", description)

	for _, warning := range warnings {
		if strings.TrimSpace(warning) != "" {
			fmt.Fprintln(streams.Out, warning)
		}
	}

	fmt.Fprint(streams.Out, "\nDo you want to continue? Type 'yes' to confirm: ")

	input := helper.GetStreams().In
	if f, ok := input.(*os.File); ok && f.Fd() == os.Stdin.Fd() {
		if tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0); err == nil {
			defer tty.Close()
			input = tty
		}
	}

	reader := bufio.NewReader(input)
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	ctx := helper.GetCmd().Context()
	if ctx == nil {
		ctx = context.Background()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		return PrepareExecutionErrorMsg(helper, "delete cancelled")
	case <-sigCh:
		return PrepareExecutionErrorMsg(helper, "delete cancelled")
	case err := <-errCh:
		_ = err
		return PrepareExecutionErrorMsg(helper, "delete cancelled")
	case line := <-lineCh:
		if strings.ToLower(strings.TrimSpace(line)) != "yes" {
			return PrepareExecutionErrorMsg(helper, "delete cancelled")
		}
		return nil
	}
}
