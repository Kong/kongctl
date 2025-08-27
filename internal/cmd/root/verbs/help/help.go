package help

import (
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var helpTemplates embed.FS

var (
	helpUse = "help"

	helpShort = i18n.T("root.verbs.help.helpShort", "Display extended help for a command")

	helpLong = normalizers.LongDesc(i18n.T("root.verbs.help.helpLong",
		`Display extended help documentation for a command.

This provides more detailed information than the standard --help flag, including:
- Comprehensive command explanations
- Detailed parameter descriptions
- Multiple examples with explanations
- Common use cases and workflows
- Troubleshooting tips`))

	helpExamples = normalizers.Examples(i18n.T("root.verbs.help.helpExamples",
		fmt.Sprintf(`
  # Show extended help for apply command
  %[1]s help apply
  
  # Show extended help for plan command
  %[1]s help plan
  
  # Show extended help for sync command
  %[1]s help sync`, meta.CLIName)))
)

// NewHelpCmd creates a new help command
func NewHelpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     helpUse + " [command]",
		Short:   helpShort,
		Long:    helpLong,
		Example: helpExamples,
		Args:    cobra.ExactArgs(1),
		RunE:    runHelp,
	}

	return cmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	commandName := args[0]

	// Load the help template for the command
	helpContent, err := loadHelpTemplate(commandName)
	if err != nil {
		// If no extended help exists, fall back to regular help
		rootCmd := cmd.Root()
		targetCmd, _, err := rootCmd.Find([]string{commandName})
		if err != nil {
			return fmt.Errorf("unknown command %q", commandName)
		}

		return targetCmd.Help()
	}

	// Get IO streams
	streams := cmd.Context().Value(iostreams.StreamsKey).(*iostreams.IOStreams)

	// Display help through pager if available
	if err := displayWithPager(helpContent, streams); err != nil {
		// Fall back to direct output if pager fails
		fmt.Fprint(streams.Out, helpContent)
	}

	return nil
}

func loadHelpTemplate(command string) (string, error) {
	// Map command names to template files
	templateFile := fmt.Sprintf("templates/%s.md", command)

	content, err := helpTemplates.ReadFile(templateFile)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func displayWithPager(content string, streams *iostreams.IOStreams) error {
	// Detect pager
	pager := os.Getenv("PAGER")
	if pager == "" {
		// Try common pagers
		for _, p := range []string{"less", "more"} {
			if _, err := exec.LookPath(p); err == nil {
				pager = p
				break
			}
		}
	}

	if pager == "" {
		return fmt.Errorf("no pager found")
	}

	// Handle less specifically for better experience
	if strings.Contains(pager, "less") {
		pager = "less -R" // Enable color support
	}

	// Create pager command
	var pagerCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		pagerCmd = exec.Command("cmd", "/c", pager)
	} else {
		pagerCmd = exec.Command("sh", "-c", pager)
	}

	// Set up pipes
	pagerCmd.Stdout = streams.Out
	pagerCmd.Stderr = streams.ErrOut

	// Create pipe for input
	pipeReader, pipeWriter := io.Pipe()
	pagerCmd.Stdin = pipeReader

	// Start pager
	if err := pagerCmd.Start(); err != nil {
		pipeWriter.Close()
		return err
	}

	// Write content to pager
	go func() {
		defer pipeWriter.Close()
		fmt.Fprint(pipeWriter, content)
	}()

	// Wait for pager to finish
	return pagerCmd.Wait()
}
