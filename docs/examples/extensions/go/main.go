package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type extensionContext struct {
	MatchedCommandPath struct {
		ID          string   `json:"id"`
		ExtensionID string   `json:"extension_id"`
		Path        []string `json:"path"`
	} `json:"matched_command_path"`
	Resolved struct {
		Profile          string `json:"profile"`
		BaseURL          string `json:"base_url"`
		Output           string `json:"output"`
		ExtensionDataDir string `json:"extension_data_dir"`
	} `json:"resolved"`
	Host struct {
		KongctlPath string `json:"kongctl_path"`
	} `json:"host"`
}

func main() {
	contextPath := os.Getenv("KONGCTL_EXTENSION_CONTEXT")
	if contextPath == "" {
		fmt.Fprintln(os.Stderr, "KONGCTL_EXTENSION_CONTEXT is not set")
		os.Exit(1)
	}

	file, err := os.Open(contextPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open context: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var ctx extensionContext
	if err := json.NewDecoder(file).Decode(&ctx); err != nil {
		fmt.Fprintf(os.Stderr, "decode context: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "extension=%s\n", ctx.MatchedCommandPath.ExtensionID)
	fmt.Fprintf(os.Stderr, "profile=%s\n", ctx.Resolved.Profile)
	fmt.Fprintf(os.Stderr, "base_url=%s\n", ctx.Resolved.BaseURL)
	fmt.Fprintf(os.Stderr, "data_dir=%s\n", ctx.Resolved.ExtensionDataDir)
	fmt.Fprintf(os.Stderr, "args=%v\n", os.Args[1:])

	kongctlPath := ctx.Host.KongctlPath
	if kongctlPath == "" {
		kongctlPath = "kongctl"
	}
	command := exec.Command(kongctlPath, "get", "me")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		fmt.Fprintf(os.Stderr,
			"kongctl get me failed; authenticate with kongctl login or provide a Konnect PAT to try the reentrant call: %v\n",
			err)
	}
}
