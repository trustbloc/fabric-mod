/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"fmt"

	gossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
)

const (
	lsccNamespace = "lscc"
)

//getCCUpgradeHandler returns block publisher chaincode upgrade handler related to given versioned DB instance
func getCCUpgradeHandler(vdb *VersionedDB) gossipapi.ChaincodeUpgradeHandler {
	return func(txnMetadata gossipapi.TxMetadata, ccID string) error {
		logger.Debugf("[%s] Clearing lscc state cache for chaincode [%s]", vdb.chainName, ccID)

		if err := vdb.cache.DelState(vdb.chainName, lsccNamespace, ccID); err != nil {
			logger.Errorf("[%s] Error clearing lscc state cache for chaincode [%s]: %s", vdb.chainName, ccID, err)
			return err
		}

		collKey := fmt.Sprintf("%s~collection", ccID)
		logger.Debugf("[%s] Clearing lscc state cache for chaincode collections [%s]", vdb.chainName, collKey)

		if err := vdb.cache.DelState(vdb.chainName, lsccNamespace, collKey); err != nil {
			logger.Errorf("[%s] Error clearing lscc state cache for chaincode collection [%s]: %s", vdb.chainName, collKey, err)
			return err
		}

		return nil
	}
}
