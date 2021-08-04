package command

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/filecoin-project/storetheindex/config"
	"github.com/urfave/cli/v2"
)

func TestInit(t *testing.T) {
	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	os.Setenv(config.EnvDir, tempDir)

	app := &cli.App{
		Name: "indexer",
		Commands: []*cli.Command{
			InitCmd,
		},
	}

	badAddr := "ip3/127.0.0.1/tcp/9999"
	err := app.RunContext(ctx, []string{"storetheindex", "init", "-adminaddr", badAddr})
	if err == nil {
		log.Fatal("expected error")
	}

	err = app.RunContext(ctx, []string{"storetheindex", "init", "-finderaddr", badAddr})
	if err == nil {
		log.Fatal("expected error")
	}

	goodAddr := "/ip4/127.0.0.1/tcp/7777"
	storeType := "pogreb"
	cacheSize := 2701
	args := []string{
		"storetheindex", "init",
		"-finderaddr", goodAddr,
		"-cachesize", fmt.Sprint(cacheSize),
		"-store", storeType,
	}
	err = app.RunContext(ctx, args)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load("")
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Addresses.Finder != goodAddr {
		t.Error("finder listen address was not configured")
	}
	if cfg.Indexer.CacheSize != cacheSize {
		t.Error("cache size was tno configured")
	}
	if cfg.Indexer.ValueStoreType != storeType {
		t.Error("value store type was not configured")
	}

	t.Log(cfg.String())
}
