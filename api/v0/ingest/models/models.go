package models

import (
	"bytes"

	"github.com/filecoin-project/go-indexer-core"
	"github.com/filecoin-project/storetheindex/internal/signature"
	"github.com/ipfs/go-cid"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

type DiscoverRequest struct {
	DiscoveryAddr string
	Nonce         []byte
	Signature     []byte
}

type RegisterRequest struct {
	AddrInfo  peer.AddrInfo
	Nonce     []byte
	Signature []byte
}

// IgenstRequest is a request to store a single CID.  This is intentionally
// limited to one CID as bulk CID ingestion should be done via advertisement
// ingestion method.
type IngestRequest struct {
	Cid       cid.Cid
	Value     indexer.Value
	Nonce     []byte
	Signature []byte
}

// ProviderData aggregates provider-related data that wants to
// be added in a response
type ProviderInfo struct {
	AddrInfo      peer.AddrInfo
	LastIndex     cid.Cid
	LastIndexTime string `json:"omitempty"`
}

func (r *RegisterRequest) Sign(privKey ic.PrivKey) error {
	var err error
	r.Nonce, err = signature.Nonce()
	if err != nil {
		return err
	}
	r.Signature, err = privKey.Sign(r.signedData())
	return err
}

func (r *RegisterRequest) VerifySignature() error {
	return signature.Verify(r.AddrInfo.ID, r.signedData(), r.Signature)
}

func (r *RegisterRequest) signedData() []byte {
	var buf bytes.Buffer
	for _, a := range r.AddrInfo.Addrs {
		buf.Write(a.Bytes())
	}
	buf.Write(r.Nonce)
	return buf.Bytes()
}

func (r *IngestRequest) Sign(privKey ic.PrivKey) error {
	var err error
	r.Nonce, err = signature.Nonce()
	if err != nil {
		return err
	}
	r.Signature, err = privKey.Sign(r.signedData())
	return err
}

func (r *IngestRequest) VerifySignature() error {
	return signature.Verify(r.Value.ProviderID, r.signedData(), r.Signature)
}

func (r *IngestRequest) signedData() []byte {
	data := make([]byte, r.Cid.ByteLen()+len(r.Value.Metadata)+len(r.Nonce))
	copy(data, r.Cid.Bytes())
	offset := r.Cid.ByteLen()
	copy(data[offset:], r.Value.Metadata)
	offset += len(r.Value.Metadata)
	copy(data[offset:], r.Nonce)
	return data
}
