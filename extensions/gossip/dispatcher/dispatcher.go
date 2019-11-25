/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dispatcher

import (
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	"github.com/hyperledger/fabric/extensions/collections/api/support"
	gossip "github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/protoext"
)

type gossipAdapter interface {
	PeersOfChannel(id common.ChannelID) []discovery.NetworkMember
	SelfMembershipInfo() discovery.NetworkMember
	IdentityInfo() gossip.PeerIdentitySet
}

type collConfigRetrieverProvider interface {
	ForChannel(channelID string) support.CollectionConfigRetriever
}

// Provider is a Gossip dispatcher provider
type Provider struct {
}

// New returns a new Gossip message dispatcher provider
func NewProvider() *Provider {
	return &Provider{}
}

// Initialize initializes the provider
func (p *Provider) Initialize(gossipAdapter gossipAdapter, ccProvider collConfigRetrieverProvider) *Provider {
	// Noop
	return p
}

// ForChannel returns a new dispatcher for the given channel
func (p *Provider) ForChannel(channelID string, dataStore storeapi.Store) *Dispatcher {
	return &Dispatcher{}
}

// Dispatcher is a extensions Gossip message dispatcher
type Dispatcher struct {
}

// Dispatch is a noop implementation
func (s *Dispatcher) Dispatch(msg protoext.ReceivedMessage) bool {
	// Nothing to handle
	return false
}
