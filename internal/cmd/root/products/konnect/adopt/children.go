package adopt

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type childCandidate struct{ id, name string }

type childSelection struct {
	identifier       string
	all              bool
	pattern          string
	idOnly, nameOnly bool
}

func addChildSelectionFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "Adopt all children in this family within the selected parent")
	cmd.Flags().String("name-pattern", "", "Adopt children with names matching a quoted glob (*, ?, [abc])")
	cmd.MarkFlagsMutuallyExclusive("all", "name-pattern")
}

func readChildSelection(cmd *cobra.Command, args []string, idFlag, nameFlag string) (childSelection, error) {
	var selection childSelection
	if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
		return selection, err
	}
	selection.all, _ = cmd.Flags().GetBool("all")
	selection.pattern, _ = cmd.Flags().GetString("name-pattern")
	count := 0
	if selection.all {
		count++
	}
	if cmd.Flags().Changed("name-pattern") {
		count++
		if selection.pattern == "" {
			return selection, fmt.Errorf("--name-pattern cannot be empty")
		}
		if _, err := compileNamePattern(selection.pattern); err != nil {
			return selection, fmt.Errorf("invalid name pattern: %w", err)
		}
	}
	if len(args) != 0 {
		selection.identifier = args[0]
		count++
	}
	for _, flag := range []string{idFlag, nameFlag} {
		if flag == "" || !cmd.Flags().Changed(flag) {
			continue
		}
		selection.identifier, _ = cmd.Flags().GetString(flag)
		selection.idOnly = flag == idFlag
		selection.nameOnly = flag == nameFlag
		count++
	}
	if count != 1 {
		return selection, fmt.Errorf("provide exactly one child identifier, --all, or --name-pattern (or an exact " +
			"child selector flag)")
	}
	if !selection.all && selection.pattern == "" && strings.TrimSpace(selection.identifier) == "" {
		return selection, fmt.Errorf("child identifier cannot be empty")
	}
	return selection, nil
}

func selectChildren(values []childCandidate, selection childSelection) ([]childCandidate, error) {
	var selected []childCandidate
	var pattern *regexp.Regexp
	if selection.pattern != "" {
		var err error
		pattern, err = compileNamePattern(selection.pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid name pattern: %w", err)
		}
	}
	seen := make(map[string]bool)
	for _, value := range values {
		matches := selection.all
		if pattern != nil {
			matches = pattern.MatchString(value.name)
		} else if !selection.all {
			matches = (!selection.nameOnly && value.id == selection.identifier) ||
				(!selection.idOnly && value.name == selection.identifier)
		}
		if !matches {
			continue
		}
		if value.id == "" || value.name == "" || seen[value.id] {
			return nil, fmt.Errorf("child collection contains a missing identity or duplicate ID")
		}
		seen[value.id] = true
		selected = append(selected, value)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no children match the selection within the selected parent")
	}
	if !selection.all && selection.pattern == "" && len(selected) != 1 {
		return nil, fmt.Errorf("child identifier %q is ambiguous; use the child ID flag", selection.identifier)
	}
	slices.SortFunc(selected, func(a, b childCandidate) int { return strings.Compare(a.id, b.id) })
	return selected, nil
}

func adoptionManagedBy(current map[string]string, namespace string,
	overwrite bool, version string,
) (map[string]string, error) {
	if owner := current[resources.ChildNamespaceKey]; owner != "" && !overwrite {
		return nil, fmt.Errorf("child already has namespace %q; use --overwrite-namespace to adopt it", owner)
	}
	if value, present := current[resources.ChildProtectedKey]; present && value != "true" && value != "false" {
		return nil, fmt.Errorf("child has invalid protection metadata")
	}
	metadata := resources.BuildChildManagedBy(namespace, resources.IsChildProtected(current), version)
	if err := resources.ValidateChildManagedBy(metadata, false); err != nil {
		return nil, err
	}
	return metadata, nil
}

type preparedChildAdoption func() (*adoptCommon.AdoptResult, error)

// Prepare the entire selection before the first write. APIs do not provide
// atomic bulk adoption, so report completed writes if execution stops midway.
func runChildAdoptions(setup adoptCommon.AdoptRunSetup, values []childCandidate,
	prepare func(childCandidate) (preparedChildAdoption, error),
) error {
	type preparedChild struct {
		candidate childCandidate
		write     preparedChildAdoption
	}
	prepared := make([]preparedChild, 0, len(values))
	for _, value := range values {
		write, err := prepare(value)
		if err != nil {
			return fmt.Errorf("cannot adopt %q (%s): %w", value.name, value.id, err)
		}
		prepared = append(prepared, preparedChild{candidate: value, write: write})
	}
	results := make([]*adoptCommon.AdoptResult, 0, len(prepared))
	for _, child := range prepared {
		result, err := child.write()
		if err != nil {
			if result != nil {
				results = append(results, result)
			}
			if printErr := printChildAdoptions(setup, results); printErr != nil {
				return fmt.Errorf("adoption failed after %d of %d children; output failed: %w; adoption error: %w",
					len(results),
					len(values),
					printErr,
					err)
			}
			if result != nil {
				action := "adopted"
				if setup.AdoptFlags.DryRun {
					action = "previewed"
				}
				return fmt.Errorf("adoption stopped at %q (%s) after %d of %d children %s; "+
					"the resource was %s but post-adoption processing failed: %w",
					child.candidate.name, child.candidate.id, len(results), len(values), action, action, err)
			}
			return fmt.Errorf("adoption stopped at %q (%s) after %d of %d children completed; the failing write"+
				" may have reached the API: %w", child.candidate.name, child.candidate.id, len(results), len(values), err)
		}
		results = append(results, result)
	}
	return printChildAdoptions(setup, results)
}

func printChildAdoptions(setup adoptCommon.AdoptRunSetup, results []*adoptCommon.AdoptResult) error {
	if len(results) == 0 {
		return nil
	}
	if handled, err := adoptCommon.PrintDryRun(setup.Helper, setup.OutType); handled {
		return err
	}
	if setup.OutType == cmdCommon.TEXT {
		for _, result := range results {
			if err := adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, result.ResourceType); err != nil {
				return err
			}
		}
		return nil
	}
	if len(results) == 1 {
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, results[0], results[0].ResourceType)
	}
	printer, err := cli.Format(setup.OutType.String(), setup.Helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(results)
	return nil
}

func printSingleChildAdoption(setup adoptCommon.AdoptRunSetup, result *adoptCommon.AdoptResult,
	adoptionErr error, description string,
) error {
	return adoptCommon.PrintAdoptionOutcome(setup.Helper, setup.OutType, result, adoptionErr, description)
}

// Resource names are opaque strings, so wildcards also match slashes. Retain
// Go glob syntax while translating to an anchored, rune-aware regexp.
func compileNamePattern(pattern string) (*regexp.Regexp, error) {
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, err
	}
	var expression strings.Builder
	expression.WriteString(`(?s)\A`)
	chars := []rune(pattern)
	for i := 0; i < len(chars); i++ {
		switch char := chars[i]; char {
		case '*':
			expression.WriteString(".*")
		case '?':
			expression.WriteByte('.')
		case '\\':
			i++
			expression.WriteString(regexp.QuoteMeta(string(chars[i])))
		case '[':
			expression.WriteByte('[')
			i++
			if chars[i] == '^' {
				expression.WriteByte('^')
				i++
			}
			for ; chars[i] != ']'; i++ {
				char := chars[i]
				escaped := char == '\\'
				if escaped {
					i++
					char = chars[i]
				}
				if char == '[' || char == ']' || char == '^' || char == '\\' || char == '-' && escaped {
					expression.WriteByte('\\')
				}
				expression.WriteRune(char)
			}
			expression.WriteByte(']')
		default:
			expression.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
	expression.WriteString(`\z`)
	return regexp.Compile(expression.String())
}
