/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package privdata

import (
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/common/privdata"
	extdissemination "github.com/hyperledger/fabric/extensions/collections/dissemination"
	"github.com/hyperledger/fabric/gossip/protoext"
)

func (d *distributorImpl) disseminationPlanForExt(ns string, rwSet *rwset.CollectionPvtReadWriteSet, colCP *peer.CollectionConfig, colAP privdata.CollectionAccessPolicy, pvtDataMsg *protoext.SignedGossipMessage) ([]*dissemination, error) {
	dissPlan, handled, err := extdissemination.ComputeDisseminationPlan(d.chainID, ns, rwSet, colCP, colAP, pvtDataMsg, d.gossipAdapter)
	if err != nil {
		return nil, err
	}

	if !handled {
		// Use default dissemination plan
		return d.disseminationPlanForMsg(colAP, colAP.AccessFilter(), pvtDataMsg)
	}

	dPlan := make([]*dissemination, len(dissPlan))
	for i, dp := range dissPlan {
		dPlan[i] = &dissemination{msg: dp.Msg, criteria: dp.Criteria}
	}
	return dPlan, nil
}
