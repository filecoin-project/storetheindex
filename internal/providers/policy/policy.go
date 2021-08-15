package policy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/filecoin-project/storetheindex/config"
	"github.com/libp2p/go-libp2p-core/peer"
)

type Policy struct {
	defaultAllow bool
	except       map[peer.ID]struct{}
	trust        map[peer.ID]struct{}
}

func New(cfg config.Policy) (*Policy, error) {
	policy := new(Policy)

	switch strings.ToLower(cfg.Action) {
	case "block":
	case "allow":
		policy.defaultAllow = true
	default:
		return nil, errors.New("default policy must be \"block\" or \"allow\"")
	}

	if len(cfg.Except) != 0 {
		exceptIDs := make(map[peer.ID]struct{}, len(cfg.Except))
		for _, except := range cfg.Except {
			excPeerID, err := peer.Decode(except)
			if err != nil {
				return nil, fmt.Errorf("error decoding except policy peer id %q: %s", except, err)
			}
			exceptIDs[excPeerID] = struct{}{}
		}
		policy.except = exceptIDs
	}

	if len(cfg.Trust) != 0 {
		trustIDs := make(map[peer.ID]struct{}, len(cfg.Trust))
		for _, trust := range cfg.Trust {
			trustPeerID, err := peer.Decode(trust)
			if err != nil {
				return nil, fmt.Errorf("error decoding trust policy peer id %q: %s", trust, err)
			}
			trustIDs[trustPeerID] = struct{}{}
		}
		policy.trust = trustIDs
	}

	if !policy.defaultAllow && len(policy.except) == 0 && len(policy.trust) == 0 {
		return nil, errors.New("policy does not allow any providers")
	}

	return policy, nil
}

// Trusted returns true if the provider is explicitly trusted.  A trusted
// provider is allowed without requiring verification.
func (p *Policy) Trusted(providerID peer.ID) bool {
	_, ok := p.trust[providerID]
	return ok
}

// Allowed returns true if the policy allows the provider to index content.
// This check does not check whether the provider is trusted. An allowed
// provider must still be verified.
func (p *Policy) Allowed(providerID peer.ID) bool {
	_, ok := p.except[providerID]
	if p.defaultAllow {
		return !ok
	}
	return ok
}
