package httpingestserver

import (
	"context"
	"fmt"
	"net"
	"net/http"

	indexer "github.com/filecoin-project/go-indexer-core"
	"github.com/filecoin-project/storetheindex/internal/ingest"
	"github.com/filecoin-project/storetheindex/internal/registry"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("indexer/ingest")

type Server struct {
	server *http.Server
	l      net.Listener
}

func (s *Server) URL() string {
	return fmt.Sprint("http://", s.l.Addr().String())
}

func New(listen string, indexer indexer.Interface, ingester *ingest.Ingester, registry *registry.Registry, options ...ServerOption) (*Server, error) {
	var cfg serverConfig
	if err := cfg.apply(append([]ServerOption{serverDefaults}, options...)...); err != nil {
		return nil, err
	}
	var err error

	l, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter().StrictSlash(true)
	server := &http.Server{
		Handler:      r,
		WriteTimeout: cfg.apiWriteTimeout,
		ReadTimeout:  cfg.apiReadTimeout,
	}
	s := &Server{server, l}

	h := newHandler(indexer, ingester, registry)

	// Advertisement routes
	r.HandleFunc("/ingest/announce", h.announce).Methods(http.MethodPut)

	// Discovery
	r.HandleFunc("/discover", h.discoverProvider).Methods(http.MethodPost)

	// Registration routes
	r.HandleFunc("/register", h.registerProvider).Methods(http.MethodPost)
	r.HandleFunc("/register/{providerid}", h.removeProvider).Methods(http.MethodDelete)

	r.HandleFunc("/health", h.health).Methods(http.MethodGet)
	return s, nil
}

func (s *Server) Start() error {
	log.Infow("ingest http server listening", "listen_addr", s.l.Addr())
	return s.server.Serve(s.l)
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info("ingest http server shutdown")
	return s.server.Shutdown(ctx)
}
