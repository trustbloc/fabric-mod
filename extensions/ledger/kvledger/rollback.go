/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import "github.com/hyperledger/fabric/core/ledger/kvledger"

func RollbackKVLedger(rootFSPath, ledgerID string, blockNum uint64) error {
	return kvledger.RollbackKVLedger(rootFSPath, ledgerID, blockNum)
}
