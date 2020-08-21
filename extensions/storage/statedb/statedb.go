/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statedb

import (
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	gossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
)

//AddCCUpgradeHandler adds chaincode upgrade handler to blockpublisher
func AddCCUpgradeHandler(chainName string, handler gossipapi.ChaincodeUpgradeHandler) {
	//do nothing
}

// Register registers a state database for a given channel
func Register(channelID string, db statedb.VersionedDB) {
	// do nothing
}
