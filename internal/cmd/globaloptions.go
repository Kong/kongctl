package cmd

//import (
//	"fmt"
//	"strings"
//
//	//"github.com/kong/kong-cli/internal/meta"
//	"github.com/spf13/cobra"
//	"github.com/spf13/pflag"
//)
//
//type GlobalOptions struct {
//	ConfigFilePath string
//	Output         string
//}
//
//func NewGlobalOptions() *GlobalOptions {
//	return &GlobalOptions{
//		// The default config file path in NewOptions is "" to distinguish between
//		// when a user provides a flag and when they don't. This value is populated
//		// from the command line flag if provided
//		ConfigFilePath: "",
//		Output:         DefaultOutputFormat,
//	}
//}
//
//func ValidOutputOptions() []string {
//	return []string{"text", "json", "yaml", "list", "table"}
//}
//
//// Validates that a given set of options are valid for the root command
//func (o *GlobalOptions) Validate(_ *cobra.Command, _ []string) error {
//	// --------------------------------
//	// Validates the output flag is from the valid set
//	validOutputOptions := ValidOutputOptions()
//	validOutput := false
//	for _, validChoice := range validOutputOptions {
//		if o.Output == validChoice {
//			validOutput = true
//			break
//		}
//	}
//	if !validOutput {
//		return fmt.Errorf("invalid output option: '%s'. Valid options are: %s",
//			o.Output, strings.Join(validOutputOptions, ", "))
//	}
//	// --------------------------------
//
//	return nil
//}
//
//// Takes an instance of the options and creates associated flags for the members
//// and adds the flags to a given FlagSet
//func (o *GlobalOptions) LinkFlags(fSet *pflag.FlagSet) {
//	// fSet.StringVarP(&o.ConfigFilePath, ConfigFilePathFlagName, "c", o.ConfigFilePath,
//	//	fmt.Sprintf("Config file (default is %s/%s.yaml).", DefaultConfigFilePath, meta.CLIName))
//	// fSet.StringVarP(&o.Output, OutputFlagName, "o", o.Output,
//	//	fmt.Sprintf("Output format. One of: %s", strings.Join(ValidOutputOptions(), ", ")))
//}
