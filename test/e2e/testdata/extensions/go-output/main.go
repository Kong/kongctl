package main

import (
	"fmt"
	"os"

	"github.com/kong/kongctl/pkg/sdk"
)

type fixtureOutput struct {
	Kind         string   `json:"kind"          yaml:"kind"`
	Args         []string `json:"args"          yaml:"args"`
	Profile      string   `json:"profile"       yaml:"profile"`
	OutputFormat string   `json:"output_format" yaml:"output_format"`
	DataDirSet   bool     `json:"data_dir_set"  yaml:"data_dir_set"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	runtimeCtx, err := sdk.LoadRuntimeContextFromEnv()
	if err != nil {
		return err
	}

	output := fixtureOutput{
		Kind:         "e2e-go",
		Args:         runtimeCtx.Args(),
		Profile:      runtimeCtx.Resolved.Profile,
		OutputFormat: runtimeCtx.OutputSettings.Format,
		DataDirSet:   runtimeCtx.DataDir() != "",
	}
	return runtimeCtx.Output().Render(output, output)
}
