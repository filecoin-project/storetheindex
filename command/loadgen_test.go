//go:build !race

package command

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/filecoin-project/storetheindex/command/loadgen"
	"github.com/filecoin-project/storetheindex/config"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

const FinderAddr = "/ip4/127.0.0.1/tcp/13000"
const IngestAddr = "/ip4/127.0.0.1/tcp/13001"
const AdminAddr = "/ip4/127.0.0.1/tcp/13002"

func TestSmallLoad(t *testing.T) {
	ctx, cncl := context.WithTimeout(context.Background(), 60*time.Second)
	defer cncl()
	testLoadHelper(ctx, t, 1, 1000, false)
}

func TestSmallLoadOverHTTP(t *testing.T) {
	ctx, cncl := context.WithTimeout(context.Background(), 60*time.Second)
	defer cncl()
	testLoadHelper(ctx, t, 1, 1000, true)
}

func TestLargeLoad(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		t.Skip("skipping Large load test on linux because it takes too long in Github Actions CI.")
	}

	ctx, cncl := context.WithTimeout(context.Background(), 180*time.Second)
	defer cncl()
	testLoadHelper(ctx, t, 10, 1000, false)
}

func testLoadHelper(ctx context.Context, t *testing.T, concurrentProviders uint, numberOfEntriesPerProvider uint, useHTTP bool) {
	switch runtime.GOOS {
	case "windows":
		t.Skip("skipping test on", runtime.GOOS)
	}

	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	os.Setenv(config.EnvDir, tempDir)
	os.Setenv("STORETHEINDEX_LISTEN_FINDER", FinderAddr)
	os.Setenv("STORETHEINDEX_LISTEN_ADMIN", AdminAddr)
	os.Setenv("STORETHEINDEX_LISTEN_INGEST", IngestAddr)
	os.Setenv("STORETHEINDE_PUBSUB_TOPIC", loadgen.DefaultConfig().GossipSubTopic)

	app := &cli.App{
		Name: "indexer",
		Commands: []*cli.Command{
			InitCmd,
			DaemonCmd,
			LoadGenCmd,
			LoadGenVerifyCmd,
		},
	}

	err := app.RunContext(ctx, []string{"storetheindex", "init"})
	require.NoError(t, err)

	go func() {
		app.RunContext(ctx, []string{"storetheindex", "daemon"})
	}()

	finderParts := strings.Split(FinderAddr, "/")
	finderPort := finderParts[len(finderParts)-1]

	c := http.Client{}
	for {
		resp, err := c.Get("http://127.0.0.1:" + finderPort + "/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Sleep a bit for the ingester to finish spinning up (should be right after finder but CI runs slow)
	time.Sleep(1 * time.Second)
	ingestParts := strings.Split(IngestAddr, "/")
	ingestPort := ingestParts[len(ingestParts)-1]
	for {
		resp, err := c.Get("http://127.0.0.1:" + ingestPort + "/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	go func() {
		startLoadgen(ctx, "http://127.0.0.1:"+ingestPort, concurrentProviders, numberOfEntriesPerProvider, useHTTP)
	}()

	foundAll := false
LOOP:
	for {
		err = app.RunContext(ctx, []string{"storetheindex", "loadgen-verify", "--indexerFind=http://127.0.0.1:" + finderPort, "--concurrentProviders=" + fmt.Sprint(concurrentProviders), "--maxEntryNumber=" + fmt.Sprint(numberOfEntriesPerProvider)})
		if err == nil {
			foundAll = true
			break
		}
		select {
		case <-ctx.Done():
			break LOOP
		case <-time.After(1 * time.Second):
		}
	}

	require.True(t, foundAll, "Did not find all entries")
}

func startLoadgen(ctx context.Context, indexerAddr string, concurrentProviders uint, numberOfEntriesPerProvider uint, useHTTP bool) {
	loadConfig := loadgen.DefaultConfig()
	loadConfig.StopAfterNEntries = uint64(numberOfEntriesPerProvider)
	loadConfig.IsHttp = useHTTP

	loadgen.StartLoadGen(ctx, loadConfig, loadgen.LoadGenOpts{
		IndexerAddr:         indexerAddr,
		ConcurrentProviders: concurrentProviders,
	})
}
