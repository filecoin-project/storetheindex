package handler

import (
	"context"

	indexer "github.com/filecoin-project/go-indexer-core"
	"github.com/filecoin-project/storetheindex/api/v0/finder/models"
	pb "github.com/filecoin-project/storetheindex/api/v0/finder/pb"
	"github.com/filecoin-project/storetheindex/internal/providers"
	"github.com/libp2p/go-libp2p-core/peer"
)

// Finder handles requests for the finder resource
type Finder struct {
	indexer  indexer.Interface
	registry *providers.Registry
}

// HandlerFunc is the function signature required by handlers in this package
type HandlerFunc func(context.Context, peer.ID, *pb.Message) (*pb.Message, error)

func NewFinder(indexer indexer.Interface, registry *providers.Registry) *Finder {
	return &Finder{
		indexer:  indexer,
		registry: registry,
	}
}

func (h *Finder) Get(ctx context.Context, p peer.ID, msg *pb.Message) (*pb.Message, error) {
	req, err := models.UnmarshalReq(msg.GetData())
	if err != nil {
		return nil, err
	}

	r, err := models.PopulateResponse(h.indexer, h.registry, req.Cids)
	if err != nil {
		return nil, err
	}
	rb, err := models.MarshalResp(r)
	if err != nil {
		return nil, err
	}

	resp := &pb.Message{
		Type: pb.Message_RESPONSE,
		Data: rb,
	}

	return resp, nil
}
