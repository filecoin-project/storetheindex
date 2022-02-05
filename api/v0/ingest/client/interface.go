package client

import (
	"context"

	v0 "github.com/filecoin-project/storetheindex/api/v0"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multihash"
)

// Ingest is the interface implemented by all ingest client protocols
type Ingest interface {
	Register(ctx context.Context, providerID peer.ID, privateKey crypto.PrivKey, addrs []string) error
	IndexContent(ctx context.Context, providerID peer.ID, privateKey crypto.PrivKey, m multihash.Multihash, contextID []byte, metadata v0.Metadata, addrs []string) error
	Announce(ctx context.Context, provider *peer.AddrInfo, root cid.Cid) error
}
