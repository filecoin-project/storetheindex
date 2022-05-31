package httpingestserver

import (
	"encoding/json"
	"io"
	"net/http"

	indexer "github.com/filecoin-project/go-indexer-core"
	"github.com/filecoin-project/storetheindex/internal/httpserver"
	"github.com/filecoin-project/storetheindex/internal/ingest"
	"github.com/filecoin-project/storetheindex/internal/registry"
	"github.com/filecoin-project/storetheindex/internal/version"
	"github.com/filecoin-project/storetheindex/server/ingest/handler"
)

type httpHandler struct {
	ingestHandler *handler.IngestHandler
}

func newHandler(indexer indexer.Interface, ingester *ingest.Ingester, registry *registry.Registry) *httpHandler {
	return &httpHandler{
		ingestHandler: handler.NewIngestHandler(indexer, ingester, registry),
	}
}

// POST /discover
func (h *httpHandler) discoverProvider(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed reading body", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = h.ingestHandler.DiscoverProvider(body)
	if err != nil {
		httpserver.HandleError(w, err, "discover")
		return
	}

	// Retrun accepted (202) response
	w.WriteHeader(http.StatusAccepted)
}

// POST /register
func (h *httpHandler) registerProvider(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed reading body", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	err = h.ingestHandler.RegisterProvider(r.Context(), body)
	if err != nil {
		httpserver.HandleError(w, err, "register")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DELETE /register/{providerid}
func (h *httpHandler) removeProvider(w http.ResponseWriter, r *http.Request) {
	/*
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Errorw("failed reading body", "err", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		unregReq := new(model.UnregisterRequest)
		err = unregReq.UnmarshalJSON(body)
		if err != nil {
			log.Errorw("error unmarshalling unregistration request", "err", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		err = h.registry.RemoveProvider(providerID)
		if err != nil {
			log.Errorw("cannot process unregistration request", "err", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		go func() {
			err := h.engine.RemoveProvider(providerID)
			if err != nil {
				log.Errorw("cannot remove content for provider", "err", err)
			}
		}()

		// Retrun accepted (202) response
		w.WriteHeader(http.StatusAccepted)
	*/

	w.WriteHeader(http.StatusNotImplemented)
}

// ----- ingest handlers -----
// PUT /ingest/announce
func (h *httpHandler) announce(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	defer r.Body.Close()
	err := h.ingestHandler.Announce(r.Body)
	if err != nil {
		httpserver.HandleError(w, err, "announce")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /health
func (h *httpHandler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	v := version.String()
	b, _ := json.Marshal(v)
	httpserver.WriteJsonResponse(w, http.StatusOK, b)
}
