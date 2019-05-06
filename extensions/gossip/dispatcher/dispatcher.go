/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dispatcher

import (
	"github.com/hyperledger/fabric/core/ledger"
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	gossip "github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/protoext"
)

type gossipAdapter interface {
	PeersOfChannel(common.ChainID) []discovery.NetworkMember
	SelfMembershipInfo() discovery.NetworkMember
	IdentityInfo() gossip.PeerIdentitySet
}

// New returns a new Gossip message dispatcher
func New(
	channelID string,
	dataStore storeapi.Store,
	gossipAdapter gossipAdapter,
	ledger ledger.PeerLedger) *Dispatcher {
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
