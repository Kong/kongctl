package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kong/kong-cli/internal/cmd/root"
	"github.com/kong/kong-cli/internal/iostreams"
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
	root.Execute(ctx, iostreams.GetOSIOStreams())
}
