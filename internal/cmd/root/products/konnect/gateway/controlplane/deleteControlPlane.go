package controlplane

import (
	"context"

	"github.com/kong/kong-cli/internal/cmd"
	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/common"
	"github.com/kong/kong-cli/internal/konnect/auth"
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

	token, e := common.GetAccessToken(cfg, logger)
	if e != nil {
		return e
	}

	kkClient, err := auth.GetAuthenticatedClient(token)
	if err != nil {
		return err
	}

	res, err := kkClient.ControlPlanes.DeleteControlPlane(ctx, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to delete Control Plane", e, helper.GetCmd(), attrs...)
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	printer, err := cli.Format(outType, helper.GetStreams().Out)
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

func newDeleteControlPlaneCmd(baseCmd *cobra.Command) *deleteControlPlaneCmd {
	rv := deleteControlPlaneCmd{
		Command: baseCmd,
	}

	baseCmd.RunE = rv.runE
	baseCmd.PreRunE = rv.preRunE

	return &rv
}
