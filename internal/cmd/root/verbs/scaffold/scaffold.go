package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/openapi2mcp"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

const (
	Verb                     = verbs.Scaffold
	outputFlagUnsupportedMsg = "flags -o/--" + cmdcommon.OutputFlagName + " are not supported for the scaffold command"

	fromOpenAPIFlagName = "from-openapi"
	aiGatewayFlagName   = "ai-gateway"
	nameFlagName        = "name"
	serverURLFlagName   = "server-url"
	routePathFlagName   = "route-path"
)

var (
	scaffoldUse = Verb.String()

	scaffoldShort = i18n.T("root.verbs.scaffold.short", "Generate a YAML scaffold for a declarative resource")

	scaffoldLong = normalizers.LongDesc(i18n.T("root.verbs.scaffold.long",
		`Scaffold emits a commented YAML starter configuration for a supported
declarative resource path.

The output is intended to be edited and then used with declarative commands
such as apply or sync.`))

	scaffoldExamples = normalizers.Examples(i18n.T("root.verbs.scaffold.examples",
		fmt.Sprintf(`
		# Generate a starter API configuration
		%[1]s scaffold api
		# Generate a root-level child resource scaffold
		%[1]s scaffold api_version
		# Generate a nested child scaffold
		%[1]s scaffold api.versions
		# Generate an analytics dashboard scaffold with a starter tile
		%[1]s scaffold analytics.dashboards
		# Convert an OpenAPI spec into an AI Gateway MCP Server with one tool per operation
		%[1]s scaffold ai_gateway_mcp_servers --from-openapi ./openapi.yaml --ai-gateway my-ai-gateway
		`, meta.CLIName)))
)

func NewScaffoldCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     scaffoldUse + " <resource-path>",
		Short:   scaffoldShort,
		Long:    scaffoldLong,
		Example: scaffoldExamples,
		Args:    cobra.MaximumNArgs(1),
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return nil
		},
		RunE: runScaffold,
	}

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), "--"+cmdcommon.OutputFlagName) ||
			strings.Contains(err.Error(), "-"+cmdcommon.OutputFlagShort) {
			return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
		}
		return err
	})

	// scaffold rejects --output itself in RunE; opt out of root validation so
	// the actionable "not supported" message can surface instead of the
	// generic "invalid value" from the root validator.
	cmdcommon.SkipOutputFormatValidation(cmd)

	cmd.Flags().String(fromOpenAPIFlagName, "",
		"Path to an OpenAPI 3 spec (- for stdin) to convert into an ai_gateway_mcp_servers entry, one tool per "+
			"operation. Local only: nothing is sent to Konnect")
	cmd.Flags().String(aiGatewayFlagName, "", "Ref of the AI Gateway the MCP Server belongs to (with --from-openapi)")
	cmd.Flags().String(nameFlagName, "", "MCP Server ref and name (with --from-openapi; default: from info.title)")
	cmd.Flags().String(serverURLFlagName, "",
		"Upstream API URL (with --from-openapi; default: the spec's first server)")
	cmd.Flags().String(routePathFlagName, "",
		"AI Gateway route path for the MCP Server (with --from-openapi; default: /<name>)")

	return cmd, nil
}

func runScaffold(command *cobra.Command, args []string) error {
	command.SilenceUsage = true

	if outputFlag := command.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
		return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
	}

	if len(args) == 0 {
		return printAvailableResourcePaths(command)
	}

	subject, err := resources.ResolveExplainSubject(args[0])
	if err != nil {
		return err
	}

	if specPath, _ := command.Flags().GetString(fromOpenAPIFlagName); specPath != "" {
		return runFromOpenAPI(command, subject, specPath)
	}
	for _, name := range []string{aiGatewayFlagName, nameFlagName, serverURLFlagName, routePathFlagName} {
		if command.Flags().Changed(name) {
			return &cmdpkg.UsageError{Err: fmt.Errorf("--%s requires --%s", name, fromOpenAPIFlagName)}
		}
	}

	scaffold, err := resources.RenderScaffoldYAML(subject)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(command.OutOrStdout(), scaffold)
	return err
}

// printAvailableResourcePaths lists the resource paths scaffold accepts, so a
// bare invocation is discoverable instead of an argument error.
func printAvailableResourcePaths(command *cobra.Command) error {
	out := command.OutOrStdout()
	if _, err := fmt.Fprintln(out, "Available resource paths:"); err != nil {
		return err
	}
	for _, path := range resources.ExplainResourcePaths() {
		if _, err := fmt.Fprintf(out, "  %s\n", path); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(
		out,
		"\nRun '%[1]s scaffold <resource-path>'. Child resources also accept nested paths,\n"+
			"for example '%[1]s scaffold api.versions'.\n",
		meta.CLIName,
	)
	return err
}

// runFromOpenAPI converts an OpenAPI spec into an ai_gateway_mcp_servers entry and prints it as YAML.
// Conversion warnings go to stderr.
func runFromOpenAPI(command *cobra.Command, subject *resources.ExplainSubject, specPath string) error {
	if subject.Doc == nil || subject.Doc.ResourceType != resources.ResourceTypeAIGatewayMCPServer {
		return &cmdpkg.UsageError{Err: fmt.Errorf(
			"--%s is only supported for ai_gateway_mcp_servers, not %s", fromOpenAPIFlagName, subject.DisplayPath)}
	}

	opts := openapi2mcp.Options{}
	flags := command.Flags()
	opts.AIGateway, _ = flags.GetString(aiGatewayFlagName)
	opts.Name, _ = flags.GetString(nameFlagName)
	opts.ServerURL, _ = flags.GetString(serverURLFlagName)
	opts.RoutePath, _ = flags.GetString(routePathFlagName)
	if opts.AIGateway == "" {
		return &cmdpkg.UsageError{Err: fmt.Errorf("--%s requires --%s", fromOpenAPIFlagName, aiGatewayFlagName)}
	}

	var spec []byte
	var err error
	if specPath == "-" {
		spec, err = io.ReadAll(command.InOrStdin())
	} else {
		spec, err = os.ReadFile(specPath)
	}
	if err != nil {
		return fmt.Errorf("reading OpenAPI spec: %w", err)
	}

	result, err := openapi2mcp.Convert(spec, opts)
	if err != nil {
		return fmt.Errorf("converting OpenAPI spec: %w", err)
	}
	for _, w := range result.Warnings {
		if _, err := fmt.Fprintf(command.ErrOrStderr(), "warning: %s\n", w); err != nil {
			return err
		}
	}

	out, err := yaml.Marshal(result.Config)
	if err != nil {
		return fmt.Errorf("rendering MCP Server YAML: %w", err)
	}
	_, err = command.OutOrStdout().Write(out)
	return err
}
