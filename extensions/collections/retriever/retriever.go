/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package retriever

import (
	"github.com/hyperledger/fabric/core/ledger"
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	supportapi "github.com/hyperledger/fabric/extensions/collections/api/support"
	gossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
)

// Provider is a private data provider.
type Provider struct {
}

// NewProvider returns a new private data provider
func NewProvider(
	storeProvider func(channelID string) storeapi.Store,
	ledgerProvider func(channelID string) ledger.PeerLedger,
	gossipProvider func() supportapi.GossipAdapter,
	blockPublisherProvider func(channelID string) gossipapi.BlockPublisher) storeapi.Provider {

	return &Provider{}
}

// RetrieverForChannel returns the private data dataRetriever for the given channel
func (p *Provider) RetrieverForChannel(channelID string) storeapi.Retriever {
	panic("not implemented")
}
