/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statedb

import (
	gossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
)

//AddCCUpgradeHandler adds chaincode upgrade handler to blockpublisher
func AddCCUpgradeHandler(chainName string, handler gossipapi.ChaincodeUpgradeHandler) {
	//do nothing
}
