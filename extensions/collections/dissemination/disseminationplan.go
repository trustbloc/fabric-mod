/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dissemination

import (
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/common/privdata"
	"github.com/hyperledger/fabric/extensions/collections/api/dissemination"
	gossipapi "github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/protoext"
)

type gossipAdapter interface {
	PeersOfChannel(id common.ChannelID) []discovery.NetworkMember
	SelfMembershipInfo() discovery.NetworkMember
	IdentityInfo() gossipapi.PeerIdentitySet
}

// ComputeDisseminationPlan returns the dissemination plan for extensions collection types
func ComputeDisseminationPlan(
	channelID, ns string,
	rwSet *rwset.CollectionPvtReadWriteSet,
	colCP *peer.CollectionConfig,
	colAP privdata.CollectionAccessPolicy,
	pvtDataMsg *protoext.SignedGossipMessage,
	gossipAdapter gossipAdapter) ([]*dissemination.Plan, bool, error) {
	panic("not implemented")
}
