/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"github.com/hyperledger/fabric/gossip/service"
)

// gossipProvider is a Gossip service provider
type gossipProvider struct {
}

// newGossipProvider returns a new Gossip service provider
func newGossipProvider() *gossipProvider {
	return &gossipProvider{}
}

// GetGossipService returns the GossipService
func (p *gossipProvider) GetGossipService() service.GossipService {
	return service.GetGossipService()
}
