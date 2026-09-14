package extensions

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const annotationPersistentFlag = "kongctl.extension.persistent-flag"

func pathContains(parent, child CommandPath) bool {
	return len(parent.Path) <= len(child.Path) &&
		slices.Equal(CommandPathNames(parent), CommandPathNames(child)[:len(parent.Path)])
}

func validatePersistentDeclarations(paths []CommandPath) error {
	for _, path := range paths {
		if len(path.Path) == 1 && IsOpenBuiltInRoot(path.Path[0].Name) && len(path.PersistentFlags) > 0 {
			return fmt.Errorf("persistent_flags cannot be declared on shared built-in root %q", path.Path[0].Name)
		}
		seen := map[string]bool{}
		for _, ancestor := range paths {
			if !pathContains(ancestor, path) {
				continue
			}
			for _, flag := range ancestor.PersistentFlags {
				if !flagNamePattern.MatchString(flag.Name) {
					return fmt.Errorf("invalid persistent flag name %q", flag.Name)
				}
				if flag.Type != "string" && flag.Type != "bool" {
					return fmt.Errorf("persistent flag --%s: unsupported type %q; use string or bool", flag.Name, flag.Type)
				}
				if reservedPersistentFlagName(flag.Name) {
					return fmt.Errorf("persistent flag --%s collides with a host flag", flag.Name)
				}
				if seen[flag.Name] {
					return fmt.Errorf("path %q: duplicate or inherited persistent flag --%s", CommandPathString(path), flag.Name)
				}
				seen[flag.Name] = true
			}
		}
		for _, flag := range path.Flags {
			if seen[flag.Name] {
				return fmt.Errorf("path %q: help-only flag --%s duplicates a persistent flag", CommandPathString(path), flag.Name)
			}
		}
	}
	return nil
}

func reservedPersistentFlagName(name string) bool {
	return IsHostFlagName(name) || slices.Contains([]string{
		"help", "version", cmdcommon.ConfigFilePathFlagName,
		cmdcommon.LogLevelFlagName, cmdcommon.LogFileFlagName,
		konnectcommon.BaseURLFlagName, konnectcommon.RegionFlagName,
		konnectcommon.PATFlagName, konnectcommon.RequestPageSizeFlagName,
	}, name)
}

func validatePersistentHostCollisions(root *cobra.Command, ext Extension) error {
	for _, path := range ext.CommandPaths {
		for _, declaration := range path.PersistentFlags {
			command := root
			for depth := 0; command != nil; depth++ {
				if flag := findFlag(command, declaration.Name); flag != nil && flag.Annotations[annotationPersistentFlag] == nil {
					return fmt.Errorf("persistent flag --%s collides with a host flag", declaration.Name)
				}
				if depth >= len(path.Path) {
					break
				}
				command = findChildByName(command, path.Path[depth].Name)
			}
		}
	}
	return nil
}

func registerPersistentFlags(command *cobra.Command, declarations []Flag) {
	for _, declaration := range declarations {
		if command.PersistentFlags().Lookup(declaration.Name) != nil {
			continue
		}
		if declaration.Type == "bool" {
			command.PersistentFlags().Bool(declaration.Name, false, declaration.Description)
		} else {
			command.PersistentFlags().String(declaration.Name, "", declaration.Description)
		}
		command.PersistentFlags().Lookup(declaration.Name).Annotations = map[string][]string{
			annotationPersistentFlag: {command.Annotations[annotationExtensionID]},
		}
	}
}

func persistentExtensionFlag(command *cobra.Command, name string) *pflag.Flag {
	owner := command.Annotations[annotationExtensionID]
	for current := command; current != nil && syntheticOwnedBy(current, owner); current = current.Parent() {
		if flag := current.PersistentFlags().Lookup(name); flag != nil && flag.Annotations[annotationPersistentFlag] != nil {
			return flag
		}
	}
	return nil
}

func inheritedPersistentDeclarations(command *cobra.Command) []Flag {
	var declarations []Flag
	owner := command.Annotations[annotationExtensionID]
	for current := command; current != nil && syntheticOwnedBy(current, owner); current = current.Parent() {
		current.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Annotations[annotationPersistentFlag] != nil {
				declarations = append(declarations, Flag{Name: flag.Name, Type: flag.Value.Type(), Description: flag.Usage})
			}
		})
	}
	return declarations
}

// persistentFlagEnd consumes exactly one occurrence without collapsing its
// spelling or repeated values into pflag's final parsed value.
func persistentFlagEnd(args []string, index int, flag *pflag.Flag) (int, error) {
	_, value, explicit := strings.Cut(args[index], "=")
	if flag.Value.Type() == "bool" {
		if explicit {
			if _, err := strconv.ParseBool(value); err != nil {
				return index, fmt.Errorf("flag --%s: %w", flag.Name, err)
			}
		}
		return index + 1, nil
	}
	if explicit {
		return index + 1, nil
	}
	if index+1 >= len(args) || args[index+1] == "--" {
		return index, fmt.Errorf("flag --%s requires a value", flag.Name)
	}
	return index + 2, nil
}

type invocationKey struct{}

type extensionInvocation struct {
	command  *cobra.Command
	original []string
	args     []string
}

// ExecuteContextC preserves extension invocation tokens before Cobra's parent
// traversal discards them. Built-in invocations still use Cobra unchanged.
func ExecuteContextC(ctx context.Context, root *cobra.Command, args []string) (*cobra.Command, error) {
	routing, invocation, err := prepareExtensionInvocation(root, args)
	if err != nil {
		return root, err
	}
	if invocation == nil {
		invocation = &extensionInvocation{original: slices.Clone(args)}
	} else {
		// Set bound host values before Cobra's initializers load configuration.
		// RunE applies those same overrides to the resulting configuration hook.
		if _, err := SplitExtensionArgs(invocation.command, invocation.args, nil); err != nil {
			return invocation.command, err
		}
	}
	ctx = context.WithValue(ctx, invocationKey{}, invocation)
	root.SetArgs(routing)
	return root.ExecuteContextC(ctx)
}

func prepareExtensionInvocation(root *cobra.Command, args []string) ([]string, *extensionInvocation, error) {
	command := root
	var routing, forwarded []string
	for i := 0; i < len(args); {
		token := args[i]
		if token == "--" || token == "--help" || token == "-h" {
			forwarded = append(forwarded, args[i:]...)
			break
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			name, _, _ := strings.Cut(strings.TrimPrefix(token, "--"), "=")
			next := i + 1
			if flag := persistentExtensionFlag(command, name); flag != nil && strings.HasPrefix(token, "--") {
				var err error
				next, err = persistentFlagEnd(args, i, flag)
				if err != nil {
					return nil, nil, err
				}
			} else {
				flags := collectHostFlags(command)
				flag := flags.Lookup(name)
				if !strings.HasPrefix(token, "--") {
					flag = flags.ShorthandLookup(token[1:2])
				}
				if flag == nil {
					forwarded = append(forwarded, args[i:]...)
					break
				}
				if flag.NoOptDefVal == "" && !strings.Contains(token, "=") &&
					(strings.HasPrefix(token, "--") || len(token) == 2) {
					next = min(i+2, len(args))
				}
			}
			forwarded = append(forwarded, args[i:next]...)
			i = next
			continue
		}
		child := findChildByNameOrAlias(command, token)
		if child == nil {
			forwarded = append(forwarded, args[i:]...)
			break
		}
		command = child
		routing = append(routing, child.Name())
		i++
	}
	if len(inheritedPersistentDeclarations(command)) == 0 {
		return args, nil, nil
	}
	return routing, &extensionInvocation{command: command, original: slices.Clone(args), args: forwarded}, nil
}
