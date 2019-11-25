/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package support

import (
	gproto "github.com/hyperledger/fabric-protos-go/gossip"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/common/privdata"
	gossipapi "github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/comm"
	gcommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
)

// GossipAdapter defines the Gossip functions that are required for collection data processing
type GossipAdapter interface {
	PeersOfChannel(id gcommon.ChannelID) []discovery.NetworkMember
	SelfMembershipInfo() discovery.NetworkMember
	IdentityInfo() gossipapi.PeerIdentitySet
	Send(msg *gproto.GossipMessage, peers ...*comm.RemotePeer)
}

// CollectionConfigRetriever retrieves collection config data and policies
type CollectionConfigRetriever interface {
	Config(ns, coll string) (*pb.StaticCollectionConfig, error)
	Policy(ns, coll string) (privdata.CollectionAccessPolicy, error)
}
