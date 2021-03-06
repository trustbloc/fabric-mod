/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger"
)

func RollbackKVLedger(ledgerconfig *ledger.Config, ledgerID string, blockNum uint64) error {
	return kvledger.RollbackKVLedger(ledgerconfig.RootFSPath, ledgerID, blockNum)
}
