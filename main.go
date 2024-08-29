package main

import (
	"context"
	"fmt"
	"os"
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

func registerSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer signal.Stop(sigs)
		sig := <-sigs
		fmt.Println("received", sig, ", terminating...")
		cancel()
	}()
	return ctx
}

func main() {
	ctx := registerSignalHandler()
	bi := build.Info{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
	root.Execute(ctx, iostreams.GetOSIOStreams(), &bi)
}
