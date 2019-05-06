/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package support

import (
	gossipapi "github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/comm"
	gcommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	gproto "github.com/hyperledger/fabric/protos/gossip"
)

// GossipAdapter defines the Gossip functions that are required for collection data processing
type GossipAdapter interface {
	PeersOfChannel(gcommon.ChainID) []discovery.NetworkMember
	SelfMembershipInfo() discovery.NetworkMember
	IdentityInfo() gossipapi.PeerIdentitySet
	Send(msg *gproto.GossipMessage, peers ...*comm.RemotePeer)
}
