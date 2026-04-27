package main

import (
	"encoding/json"
	"fmt"
	"os"
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

	fmt.Printf("extension=%s\n", ctx.MatchedCommandPath.ExtensionID)
	fmt.Printf("profile=%s\n", ctx.Resolved.Profile)
	fmt.Printf("base_url=%s\n", ctx.Resolved.BaseURL)
	fmt.Printf("data_dir=%s\n", ctx.Resolved.ExtensionDataDir)
	fmt.Printf("args=%v\n", os.Args[1:])
}
