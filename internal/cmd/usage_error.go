package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	RequiresSubcommandAnnotation = "kongctl/requires-subcommand"
	formatFlagName               = "format"
)

type Suggestion struct {
	Kind   string
	Values []string
}

type UsageError struct {
	Err        error
	Suggestion Suggestion
}

func (e *UsageError) Error() string {
	if e == nil || e.Err == nil {
		return "invalid command usage"
	}
	return e.Err.Error()
}

func (e *UsageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ConfigureRequiresSubcommand(command *cobra.Command) {
	if command == nil {
		return
	}
	MarkRequiresSubcommand(command)
	command.RunE = RequireSubcommand
}

func MarkRequiresSubcommand(command *cobra.Command) {
	if command == nil {
		return
	}
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations[RequiresSubcommandAnnotation] = "true"
}

func CommandRequiresSubcommand(command *cobra.Command) bool {
	if command == nil || command.Annotations == nil {
		return false
	}
	return command.Annotations[RequiresSubcommandAnnotation] == "true"
}

func RequireSubcommand(command *cobra.Command, args []string) error {
	if len(args) == 0 {
		return MissingSubcommandError(command)
	}
	return UnknownSubcommandError(command, args[0])
}

func MissingSubcommandError(command *cobra.Command) error {
	return &UsageError{
		Err: fmt.Errorf("command %q requires a subcommand", commandPath(command)),
		Suggestion: Suggestion{
			Kind:   "subcommand",
			Values: AvailableSubcommands(command),
		},
	}
}

func UnknownSubcommandError(command *cobra.Command, arg string) error {
	return &UsageError{
		Err: fmt.Errorf("unknown command %q for %q", arg, commandPath(command)),
		Suggestion: Suggestion{
			Kind:   "command",
			Values: SuggestSimilarCommands(command, arg),
		},
	}
}

func SuggestionForError(command *cobra.Command, err error) Suggestion {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return usageErr.Suggestion
	}
	if suggestion := SuggestSimilarFlags(command, err); len(suggestion.Values) > 0 {
		return suggestion
	}
	return Suggestion{}
}

func IsUsageError(err error) bool {
	if err == nil {
		return false
	}

	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return true
	}

	var configErr *ConfigurationError
	if errors.As(err, &configErr) {
		return true
	}

	var notExistErr *pflag.NotExistError
	if errors.As(err, &notExistErr) {
		return true
	}

	var valueRequiredErr *pflag.ValueRequiredError
	if errors.As(err, &valueRequiredErr) {
		return true
	}

	var invalidValueErr *pflag.InvalidValueError
	if errors.As(err, &invalidValueErr) {
		return true
	}

	var invalidSyntaxErr *pflag.InvalidSyntaxError
	if errors.As(err, &invalidSyntaxErr) {
		return true
	}

	return isCobraArgumentError(err.Error())
}

func isCobraArgumentError(message string) bool {
	message = strings.TrimSpace(message)
	return strings.HasPrefix(message, "accepts ") ||
		strings.HasPrefix(message, "requires at least ") ||
		strings.HasPrefix(message, "required flag(s) ") ||
		strings.HasPrefix(message, "unknown command ") ||
		strings.HasPrefix(message, "unexpected argument ")
}

func SuggestSimilarCommands(command *cobra.Command, typed string) []string {
	if command == nil || strings.TrimSpace(typed) == "" {
		return nil
	}

	type candidate struct {
		name  string
		score int
	}
	candidates := []candidate{}
	typed = strings.ToLower(strings.TrimSpace(typed))
	bestScore := 3

	for _, child := range command.Commands() {
		if !child.IsAvailableCommand() {
			continue
		}
		name := child.Name()
		score := levenshtein(typed, strings.ToLower(name))
		if strings.HasPrefix(strings.ToLower(name), typed) {
			score = 0
		}
		for _, explicit := range child.SuggestFor {
			if strings.EqualFold(typed, explicit) {
				score = 0
				break
			}
		}
		if score > 2 {
			continue
		}
		if score < bestScore {
			bestScore = score
			candidates = candidates[:0]
		}
		if score == bestScore {
			candidates = append(candidates, candidate{name: name, score: score})
		}
	}

	suggestions := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		suggestions = append(suggestions, candidate.name)
	}
	slices.Sort(suggestions)
	return suggestions
}

func AvailableSubcommands(command *cobra.Command) []string {
	if command == nil {
		return nil
	}

	var subcommands []string
	for _, child := range command.Commands() {
		if child.IsAvailableCommand() {
			subcommands = append(subcommands, child.Name())
		}
	}
	slices.Sort(subcommands)
	return subcommands
}

func SuggestSimilarFlags(command *cobra.Command, err error) Suggestion {
	if command == nil || err == nil {
		return Suggestion{}
	}

	var flagErr *pflag.NotExistError
	if !errors.As(err, &flagErr) {
		return Suggestion{}
	}

	name := flagErr.GetSpecifiedName()
	if name == "" {
		return Suggestion{}
	}

	shortnames := flagErr.GetSpecifiedShortnames()
	if shortnames != "" {
		return suggestSimilarShorthandFlags(command, name)
	}
	return suggestSimilarLongFlags(command, name)
}

func suggestSimilarLongFlags(command *cobra.Command, typed string) Suggestion {
	candidates := make([]flagSuggestion, 0)
	if strings.EqualFold(typed, formatFlagName) {
		if output := suggestedOutputFlag(command); output != nil {
			candidates = append(candidates, newFlagSuggestion(outputFlagSuggestionLabel(output), output))
		}
	}
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		if similar(typed, flag.Name) {
			candidates = append(candidates, newFlagSuggestion("--"+flag.Name, flag))
		}
	})
	return Suggestion{Kind: "flag", Values: formatFlagSuggestions(candidates)}
}

func suggestSimilarShorthandFlags(command *cobra.Command, typed string) Suggestion {
	candidates := make([]flagSuggestion, 0)
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flag.Shorthand == "" {
			return
		}
		if similar(typed, flag.Shorthand) {
			candidates = append(candidates, newFlagSuggestion(fmt.Sprintf("-%s, --%s", flag.Shorthand, flag.Name), flag))
		}
	})
	return Suggestion{Kind: "flag", Values: formatFlagSuggestions(candidates)}
}

func suggestedOutputFlag(command *cobra.Command) *pflag.Flag {
	if command == nil || cmdcommon.IsOutputFormatValidationSkipped(command) {
		return nil
	}
	flag := command.Flags().Lookup(cmdcommon.OutputFlagName)
	if flag == nil || flag.Hidden {
		return nil
	}
	return flag
}

func outputFlagSuggestionLabel(flag *pflag.Flag) string {
	if flag == nil || strings.TrimSpace(flag.Shorthand) == "" {
		return "--" + cmdcommon.OutputFlagName
	}
	return fmt.Sprintf("--%s, -%s", cmdcommon.OutputFlagName, flag.Shorthand)
}

type flagSuggestion struct {
	label       string
	description string
}

func newFlagSuggestion(label string, flag *pflag.Flag) flagSuggestion {
	return flagSuggestion{
		label:       label,
		description: flagDescription(flag),
	}
}

func formatFlagSuggestions(candidates []flagSuggestion) []string {
	slices.SortFunc(candidates, func(a, b flagSuggestion) int {
		return strings.Compare(a.label, b.label)
	})

	width := 0
	for _, candidate := range candidates {
		width = max(width, len(candidate.label))
	}

	suggestions := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.description == "" {
			suggestions = append(suggestions, candidate.label)
			continue
		}
		suggestions = append(suggestions, fmt.Sprintf("%-*s  %s", width, candidate.label, candidate.description))
	}
	return suggestions
}

func flagDescription(flag *pflag.Flag) string {
	if flag == nil {
		return ""
	}
	description, _, _ := strings.Cut(strings.TrimSpace(flag.Usage), "\n")
	return strings.TrimSpace(description)
}

func similar(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	return strings.HasPrefix(b, a) || levenshtein(a, b) <= 2
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range len(prev) {
		prev[j] = j
	}

	for i, ca := range a {
		curr[0] = i + 1
		for j, cb := range b {
			cost := 1
			if ca == cb {
				cost = 0
			}
			curr[j+1] = min(
				curr[j]+1,
				prev[j+1]+1,
				prev[j]+cost,
			)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func commandPath(command *cobra.Command) string {
	if command == nil {
		return "kongctl"
	}
	return command.CommandPath()
}
