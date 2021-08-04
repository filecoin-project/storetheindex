package command

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/urfave/cli/v2"

	httpclient "github.com/filecoin-project/storetheindex/api/v1/client/http"
	p2pclient "github.com/filecoin-project/storetheindex/api/v1/client/libp2p"
	"github.com/filecoin-project/storetheindex/internal/finder"
	"github.com/filecoin-project/storetheindex/server/net"
)

const getTimeout = 15 * time.Second

var GetCmd = &cli.Command{
	Name:   "get",
	Usage:  "Get single Cid from idexer",
	Flags:  ClientCmdFlags,
	Action: getCidCmd,
}

func getCidCmd(cctx *cli.Context) error {
	protocol := cctx.String("protocol")
	endpoint := cctx.String("finder_ep")
	var err error
	var cl finder.Interface
	var end net.Endpoint

	switch protocol {
	case "http":
		cl, err = httpclient.New()
		if err != nil {
			return err
		}

		end = net.NewHTTPEndpoint(endpoint)
	case "libp2p":
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		end, err = net.NewP2PEndpoint(endpoint)
		if err != nil {
			return err
		}
		// NOTE: Creaeting a new host just for querying purposes.
		// Libp2p protocol requests from CLI should only be used
		// for testing purposes. This interface is in place
		/// for long-running peers.
		var host host.Host
		host, err = libp2p.New(ctx)
		if err != nil {
			return err
		}
		cl, err = p2pclient.New(ctx, host)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognized protocol type for client interaction: %s", protocol)
	}

	cget := cctx.Args().Get(0)
	if cget == "" {
		return fmt.Errorf("no cid provided as input")
	}
	ccid, err := cid.Decode(cget)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), getTimeout)
	defer cancel()

	resp, err := cl.Get(ctx, ccid, end)
	log.Info("Response: %v", resp)
	return err

}
