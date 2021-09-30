package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/filecoin-project/storetheindex/command"
	"github.com/filecoin-project/storetheindex/internal/version"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("indexer-node")

func main() {
	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler to cancel the context
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)
		select {
		case <-interrupt:
			cancel()
			fmt.Println("Received interrupt signal, shutting down...")
			fmt.Println("(Hit ctrl-c again to force-shutdown the daemon.)")
		case <-ctx.Done():
		}
		// Allow any further SIGTERM or SIGINT to kill process
		signal.Stop(interrupt)
	}()

	if err := logging.SetLogLevel("*", "info"); err != nil {
		log.Fatal(err)
	}

	app := &cli.App{
		Name:    "indexer",
		Usage:   "Indexer Node: Filecoin's data indexer",
		Version: version.String(),
		Commands: []*cli.Command{
			command.DaemonCmd,
			command.FindCmd,
			command.ImportCmd,
			command.InitCmd,
			command.RegisterCmd,
			command.SyntheticCmd,
			command.IngestCmd,
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
