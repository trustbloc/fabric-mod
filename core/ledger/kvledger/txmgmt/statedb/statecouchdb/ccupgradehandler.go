/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"github.com/hyperledger/fabric/extensions/gossip/api"
)

const lsccKeyPrefix = "%s~"

//getCCUpgradeHandler returns block publisher chaincode upgrade handler related to given versioned DB instance
func getCCUpgradeHandler(vdb *VersionedDB) api.ChaincodeUpgradeHandler {
	return func(txMetadata api.TxMetadata, chaincodeName string) error {
		logger.Debugf("Clearing lscc state cache for chaincode [%s]", chaincodeName)
		// TODO: evictEntry is no longer implemented
		//vdb.cache.evictEntry(chaincodeName)
		return nil
	}
}
