package deck

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/kong/kongctl/internal/declarative/constants"
)

// Runner executes deck commands.
type Runner interface {
	Run(ctx context.Context, opts RunOptions) (*RunResult, error)
}

// RunOptions configures a deck invocation.
type RunOptions struct {
	Args                    []string
	Mode                    string
	KonnectToken            string
	KonnectControlPlaneName string
	KonnectAddress          string
}

// RunResult captures deck command output.
type RunResult struct {
	Stdout string
	Stderr string
}

// ExecRunner runs deck via os/exec.
type ExecRunner struct {
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewRunner returns a runner that executes deck commands.
func NewRunner() *ExecRunner {
	return &ExecRunner{
		execCommand: exec.CommandContext,
	}
}

// Run executes deck with the provided options.
func (r *ExecRunner) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if r == nil || r.execCommand == nil {
		return nil, ErrInvalidArgs{Reason: "deck runner not configured"}
	}

	args, err := buildArgs(opts)
	if err != nil {
		return nil, err
	}

	cmd := r.execCommand(ctx, "deck", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		result := &RunResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}
		if errors.Is(err, exec.ErrNotFound) {
			return result, ErrDeckNotFound{}
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return result, ErrDeckNotFound{}
		}
		return result, err
	}

	return &RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, nil
}

func buildArgs(opts RunOptions) ([]string, error) {
	if len(opts.Args) == 0 {
		return nil, ErrInvalidArgs{Reason: "args cannot be empty"}
	}

	args := append([]string{}, opts.Args...)

	for i, arg := range args {
		if arg == constants.DeckModePlaceholder {
			mode := strings.TrimSpace(opts.Mode)
			if mode == "" {
				return nil, ErrInvalidArgs{Reason: "mode placeholder requires apply or sync"}
			}
			if mode != "apply" && mode != "sync" {
				return nil, ErrInvalidArgs{Reason: "mode must be apply or sync"}
			}
			args[i] = mode
			continue
		}
		if strings.Contains(arg, constants.DeckModePlaceholder) {
			return nil, ErrInvalidArgs{Reason: "mode placeholder must be a standalone argument"}
		}
	}

	if len(args) > 0 && args[0] == "gateway" {
		if err := ensureKonnectContext(opts); err != nil {
			return nil, err
		}
		if err := ensureNoFlag(args, "--konnect-token"); err != nil {
			return nil, err
		}
		if err := ensureNoFlag(args, "--konnect-control-plane-name"); err != nil {
			return nil, err
		}
		if err := ensureNoFlag(args, "--konnect-addr"); err != nil {
			return nil, err
		}

		injected := []string{
			"--konnect-token", opts.KonnectToken,
			"--konnect-control-plane-name", opts.KonnectControlPlaneName,
			"--konnect-addr", opts.KonnectAddress,
		}
		insertAt := 2
		if insertAt > len(args) {
			insertAt = len(args)
		}
		args = append(args[:insertAt], append(injected, args[insertAt:]...)...)
	}

	return args, nil
}

func ensureKonnectContext(opts RunOptions) error {
	if strings.TrimSpace(opts.KonnectToken) == "" {
		return ErrInvalidArgs{Reason: "konnect token is required for gateway steps"}
	}
	if strings.TrimSpace(opts.KonnectControlPlaneName) == "" {
		return ErrInvalidArgs{Reason: "konnect control plane name is required for gateway steps"}
	}
	if strings.TrimSpace(opts.KonnectAddress) == "" {
		return ErrInvalidArgs{Reason: "konnect address is required for gateway steps"}
	}
	return nil
}

func ensureNoFlag(args []string, flag string) error {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return ErrConflictingFlag{Flag: flag}
		}
	}
	return nil
}
