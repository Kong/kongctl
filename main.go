package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd/root"
	"github.com/kong/kongctl/internal/iostreams"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func registerSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	return ctx, cancel
}

func main() {
	ctx, cancel := registerSignalHandler()
	defer cancel()
	bi := build.Info{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
	root.Execute(ctx, iostreams.GetOSIOStreams(), &bi)
}
