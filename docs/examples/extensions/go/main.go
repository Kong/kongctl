package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kong/kongctl/pkg/sdk"
)

type userDisplayRecord struct {
	ID      string `json:"id"      yaml:"id"`
	Email   string `json:"email"   yaml:"email"`
	Name    string `json:"name"    yaml:"name"`
	Active  string `json:"active"  yaml:"active"`
	Profile string `json:"profile" yaml:"profile"`
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

	ctx := context.Background()
	konnect, err := runtimeCtx.KonnectSDK(ctx)
	if err != nil {
		return fmt.Errorf("create Konnect SDK client: %w", err)
	}

	res, err := konnect.Me.GetUsersMe(ctx)
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}
	user := res.GetUser()
	if user == nil {
		return fmt.Errorf("current user response did not include a user")
	}

	record := userDisplayRecord{
		ID:      "n/a",
		Email:   "n/a",
		Name:    "n/a",
		Active:  "n/a",
		Profile: runtimeCtx.Resolved.Profile,
	}

	if user.ID != nil && *user.ID != "" {
		record.ID = *user.ID
	}
	if user.Email != nil && *user.Email != "" {
		record.Email = *user.Email
	}
	if user.FullName != nil && *user.FullName != "" {
		record.Name = *user.FullName
	}
	if user.Active != nil {
		record.Active = fmt.Sprintf("%t", *user.Active)
	}

	return runtimeCtx.Output().Render(record, user)
}
