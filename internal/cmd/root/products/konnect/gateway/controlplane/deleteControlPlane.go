package controlplane

import (
	"context"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type deleteControlPlaneCmd struct {
	*cobra.Command
}

func (c *deleteControlPlaneCmd) validate(_ cmd.Helper) error {
	return nil
}

func (c *deleteControlPlaneCmd) run(helper cmd.Helper) error {
	id := helper.GetArgs()[0]

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	ctx := context.Background()

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	res, err := sdk.GetControlPlaneAPI().DeleteControlPlane(ctx, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to delete Control Plane", e, helper.GetCmd(), attrs...)
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
	printer.Print(res)
	return nil
}

func (c *deleteControlPlaneCmd) preRunE(_ *cobra.Command, _ []string) error {
	return nil
}

func (c *deleteControlPlaneCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}
	return c.run(helper)
}

func newDeleteControlPlaneCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *deleteControlPlaneCmd {
	rv := deleteControlPlaneCmd{
		Command: baseCmd,
	}

	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	baseCmd.RunE = rv.runE
	baseCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if parentPreRun != nil {
			if e := parentPreRun(cmd, args); e != nil {
				return e
			}
		}
		return rv.preRunE(cmd, args)
	}

	return &rv
}
